package orchestrator

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/guard"
	"oh-my-gunnsama/internal/prompt"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/route"
	"oh-my-gunnsama/internal/state"
	"oh-my-gunnsama/internal/storage"
)

type Request = protocol.Request
type Response = protocol.Response

type Dependencies struct {
	ConfigOptions     config.Options
	RegistryOptions   registry.Options
	GuardRules        []guard.Rule
	GuardSuppressions []guard.Suppression
	StateStore        *state.Store
	StorageSink       storage.Sink
	PromptSections    []prompt.SectionSpec
	PromptBudget      prompt.TokenBudget
	ProjectContext    prompt.ProjectContext
	ContextSnapshot   *prompt.PromptContextSnapshot
	Flags             map[string]string
	OwnerID           string
	Now               func() time.Time
	WorkerFactory     WorkerFactory
}

func Handle(ctx context.Context, req *Request, deps Dependencies) Response {
	if req == nil {
		return protocol.ErrorResponse(nil, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, "request is required", false), nil)
	}
	switch req.Event {
	case protocol.EventOnSubmit:
		return handleSubmit(ctx, req, deps)
	case protocol.EventOnToolBefore:
		return handleToolBefore(ctx, req, deps)
	case protocol.EventOnError:
		return handleStateTransition(ctx, req, deps, lifecycleForError(req), "error")
	case protocol.EventOnStop:
		return handleStateTransition(ctx, req, deps, state.LifecycleCancelled, "stop")
	case protocol.EventOnToolAfter:
		return handleToolAfter(ctx, req, deps)
	case protocol.EventOnCompact:
		return protocol.OKResponse(req, protocol.ActionNone, "", nil)
	case protocol.EventOnRun:
		return handleRun(ctx, req, deps)
	default:
		return protocol.OKResponse(req, protocol.ActionNone, "", nil)
	}
}

func handleSubmit(ctx context.Context, req *Request, deps Dependencies) Response {
	cfg, err := config.Load(configOptions(req, deps))
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), nil)
	}
	reg, err := registry.Build(cfg, deps.RegistryOptions)
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), nil)
	}
	result := route.Route(payloadText(req), reg)
	agent, _ := reg.Agent(result.Agent)
	sections := deps.PromptSections
	if len(sections) == 0 {
		sections = defaultPromptSections(agent, result, deps.ProjectContext)
	}
	model := modelInfo(agent)
	out, err := prompt.BuildPrompt(prompt.BuildPromptInput{
		Agent:    agent,
		Model:    model,
		Project:  deps.ProjectContext,
		Context:  deps.ContextSnapshot,
		Route:    result,
		Flags:    deps.Flags,
		Budget:   deps.PromptBudget,
		Sections: sections,
	})
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), append(reg.Warnings, routeWarning(result)))
	}
	warnings := append([]string{}, reg.Warnings...)
	warnings = append(warnings, out.Warnings...)
	warnings = append(warnings, routeWarning(result))
	if err := recordStorageEvent(ctx, req, deps, storage.Event{Type: storage.EventRunCreated, Status: "scoped", ActorType: "user"}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}
	return protocol.OKResponse(req, protocol.ActionInjectPrompt, out.SystemPrompt, warnings)
}

func handleToolBefore(ctx context.Context, req *Request, deps Dependencies) Response {
	input := guardInput(req)
	result := guard.Evaluate(input, deps.GuardRules, deps.GuardSuppressions)
	warnings := append([]string{}, result.Warnings...)
	action := protocol.ActionNone
	switch result.Decision.Action {
	case guard.GuardBlock:
		action = protocol.ActionBlock
	case guard.GuardWarn:
		action = protocol.ActionWarn
	case guard.GuardAllow:
		action = protocol.ActionNone
	}
	if action == protocol.ActionWarn && result.Decision.Reason != "" {
		warnings = append(warnings, result.Decision.Reason)
	}
	eventType := storage.EventToolCallStarted
	status := "allowed"
	if action == protocol.ActionBlock {
		eventType = storage.EventToolCallBlocked
		status = "blocked"
	} else if action == protocol.ActionWarn {
		status = "warned"
	}
	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:       eventType,
		Status:     status,
		ActorType:  "tool",
		Attributes: storageAttributes(req, map[string]string{"decision_reason": result.Decision.Reason}),
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}
	return protocol.OKResponse(req, action, result.Decision.Reason, warnings)
}

func handleToolAfter(ctx context.Context, req *Request, deps Dependencies) Response {
	eventType := storage.EventToolCallCompleted
	status := "completed"
	if stringValue(req.Payload, "error_message") != "" || stringValue(req.Payload, "error") != "" {
		eventType = storage.EventToolCallFailed
		status = "failed"
	}
	warnings := []string{}
	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:       eventType,
		Status:     status,
		ActorType:  "tool",
		Attributes: storageAttributes(req, nil),
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}
	return protocol.OKResponse(req, protocol.ActionNone, "", warnings)
}

func handleStateTransition(ctx context.Context, req *Request, deps Dependencies, to state.Lifecycle, contextName string) Response {
	store := deps.StateStore
	if store == nil {
		if req.Project == "" || req.SessionID == "" {
			return protocol.OKResponse(req, protocol.ActionNone, "", nil)
		}
		store = state.NewStore(req.Project, state.Options{})
	}
	if req.SessionID == "" {
		return protocol.OKResponse(req, protocol.ActionNone, "", []string{"state transition skipped: session_id is required"})
	}
	record, err := store.Transition(req.SessionID, state.TransitionInput{
		To:            to,
		Context:       contextName,
		OwnerID:       deps.OwnerID,
		RequestID:     req.RequestID,
		BlockedReason: blockedReason(req),
		Error:         errorSummary(req),
		Now:           now(deps),
	})
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorUnknown, err.Error(), false), nil)
	}
	if err := recordStorageEvent(ctx, req, deps, storage.Event{Type: storage.EventRunStatusChanged, Status: string(record.Lifecycle), Lifecycle: string(record.Lifecycle), ActorType: "run_core", ActorID: deps.OwnerID}); err != nil {
		return protocol.OKResponse(req, protocol.ActionWarn, "", []string{storageWarning(err)})
	}
	return protocol.OKResponse(req, protocol.ActionNone, "", nil)
}

func recordStorageEvent(ctx context.Context, req *Request, deps Dependencies, event storage.Event) error {
	if deps.StorageSink == nil {
		return nil
	}
	event.RunID = firstNonEmpty(event.RunID, stringValue(req.Payload, "run_id"), req.RequestID)
	event.WorkID = firstNonEmpty(event.WorkID, stringValue(req.Payload, "work_id"))
	event.AgentSessionID = firstNonEmpty(event.AgentSessionID, req.SessionID, stringValue(req.Payload, "agent_session_id"))
	event.EvidenceID = firstNonEmpty(event.EvidenceID, stringValue(req.Payload, "evidence_id"))
	event.ToolCallID = firstNonEmpty(event.ToolCallID, stringValue(req.Payload, "tool_call_id"))
	event.VerificationID = firstNonEmpty(event.VerificationID, stringValue(req.Payload, "verification_id"))
	event.RequestID = firstNonEmpty(event.RequestID, req.RequestID)
	event.Provider = firstNonEmpty(event.Provider, string(req.Provider))
	event.HostEvent = firstNonEmpty(event.HostEvent, string(req.Event))
	event.ProjectRoot = firstNonEmpty(event.ProjectRoot, req.Project)
	if event.ActorID == "" {
		event.ActorID = firstNonEmpty(deps.OwnerID, req.SessionID)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = now(deps)
	}
	return storage.Recorder{Sink: deps.StorageSink, Now: deps.Now}.Record(ctx, event)
}

func storageAttributes(req *Request, extra map[string]string) map[string]string {
	attrs := map[string]string{}
	for _, key := range []string{"tool", "agent", "path", "command", "error", "error_class", "error_code", "error_message", "blocked_reason"} {
		if value := stringValue(req.Payload, key); value != "" {
			attrs[key] = value
		}
	}
	if args, ok := req.Payload["args"].(map[string]any); ok {
		for _, key := range []string{"path", "command"} {
			if value, ok := args[key].(string); ok && value != "" {
				attrs["arg_"+key] = value
			}
		}
	}
	for key, value := range extra {
		if value != "" {
			attrs[key] = value
		}
	}
	if len(attrs) == 0 {
		return nil
	}
	return attrs
}

func storageWarning(err error) string {
	return "storage event not recorded: " + err.Error()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func configOptions(req *Request, deps Dependencies) config.Options {
	options := deps.ConfigOptions
	if options.ProjectDir == "" && req.Project != "" {
		options.ProjectDir = filepath.Join(req.Project, ".omg")
	}
	return options
}

func defaultPromptSections(agent registry.Agent, result route.RouteResult, project prompt.ProjectContext) []prompt.SectionSpec {
	agentName := agent.Name
	if agentName == "" {
		agentName = "omg"
	}
	identity := fmt.Sprintf("Agent: %s\nProcedure: %s\nDomain: %s\nConfidence: %s", agentName, result.Procedure, result.Domain, result.Confidence)
	sections := []prompt.SectionSpec{
		{Name: "route/identity", Source: "internal/orchestrator", Content: identity, Required: true, Priority: 100},
		{Name: "guardrails", Source: "internal/orchestrator", Content: guardrailsText(), Required: true, Priority: 95},
		{Name: "tool-policy", Source: "internal/orchestrator", Content: toolPolicyText(agent.Tools), Required: true, Priority: 90},
		{Name: "autonomy", Source: "internal/orchestrator", Content: autonomyText(agent.Autonomy), Required: true, Priority: 85},
	}
	if strings.TrimSpace(agent.Description) != "" {
		sections = append(sections, prompt.SectionSpec{Name: "agent-description", Source: agent.Source, Content: agent.Description, Required: false, Priority: 80})
	}
	if content := roleFitText(agent.UseWhen, agent.AvoidWhen); content != "" {
		sections = append(sections, prompt.SectionSpec{Name: "role-fit", Source: agent.Source, Content: content, Required: false, Priority: 70})
	}
	if strings.TrimSpace(project.Rules) != "" {
		sections = append(sections, prompt.SectionSpec{Name: "project-rules", Source: "project", Content: project.Rules, Required: false, Priority: 60})
	}
	return sections
}

func toolPolicyText(policy registry.ToolPolicy) string {
	switch policy.Mode {
	case registry.ToolModeAllowlist:
		return fmt.Sprintf("Tool policy: allowlist\nUse only these listed tools: %s", strings.Join(policy.Allow, ", "))
	case registry.ToolModeAllExcept:
		return fmt.Sprintf("Tool policy: all_except\nYou may use available tools except these listed tools: %s", strings.Join(policy.Except, ", "))
	default:
		return "Tool policy: default\nUse the host LLM's default tool policy."
	}
}

func autonomyText(level string) string {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "unspecified"
	}
	content := fmt.Sprintf("Autonomy: %s", level)
	if strings.EqualFold(level, "cautious") {
		content += "\nAsk for confirmation before risky, destructive, external, credential-gated, or production-impacting actions."
	}
	return content
}

func roleFitText(useWhen, avoidWhen []string) string {
	parts := []string{}
	if len(useWhen) > 0 {
		parts = append(parts, "Use when:\n- "+strings.Join(useWhen, "\n- "))
	}
	if len(avoidWhen) > 0 {
		parts = append(parts, "Avoid when:\n- "+strings.Join(avoidWhen, "\n- "))
	}
	return strings.Join(parts, "\n")
}

func guardrailsText() string {
	return strings.Join([]string{
		"Guardrails:",
		"- Stay aware of token and time budget.",
		"- Do not repeat the same approach more than 3 times; report when blocked.",
		"- When uncertain, report evidence and remaining risks.",
	}, "\n")
}

func modelInfo(agent registry.Agent) prompt.ModelInfo {
	if len(agent.Models) == 0 {
		return prompt.ModelInfo{}
	}
	return prompt.ModelInfo{Model: agent.Models[0].Model, Family: agent.Models[0].Variant}
}

func routeWarning(result route.RouteResult) string {
	return fmt.Sprintf("route: procedure=%s domain=%s agent=%s confidence=%s", result.Procedure, result.Domain, result.Agent, result.Confidence)
}

func guardInput(req *Request) guard.GuardInput {
	payload := req.Payload
	args, _ := payload["args"].(map[string]any)
	return guard.GuardInput{
		RequestID: req.RequestID,
		Project:   req.Project,
		CWD:       req.CWD,
		Tool:      stringValue(payload, "tool"),
		Args:      args,
		Agent:     stringValue(payload, "agent"),
		Autonomy:  stringValue(payload, "autonomy"),
		SessionID: req.SessionID,
	}
}

func lifecycleForError(req *Request) state.Lifecycle {
	if stringValue(req.Payload, "blocked_reason") != "" || stringValue(req.Payload, "error_class") == "policy_blocked" {
		return state.LifecycleBlocked
	}
	return state.LifecycleFailed
}

func blockedReason(req *Request) string {
	if req.Event != protocol.EventOnError {
		return ""
	}
	return stringValue(req.Payload, "blocked_reason")
}

func errorSummary(req *Request) *state.OmgErrorSummary {
	if req.Event != protocol.EventOnError {
		return nil
	}
	return &state.OmgErrorSummary{
		Class:   stringValue(req.Payload, "error_class"),
		Domain:  stringValue(req.Payload, "error_domain"),
		Code:    stringValue(req.Payload, "error_code"),
		Message: stringValue(req.Payload, "error_message"),
	}
}

func payloadText(req *Request) string {
	for _, key := range []string{"input", "prompt", "text"} {
		if value := stringValue(req.Payload, key); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	value, _ := values[key].(string)
	return value
}

func now(deps Dependencies) time.Time {
	if deps.Now != nil {
		return deps.Now()
	}
	return time.Now().UTC()
}
