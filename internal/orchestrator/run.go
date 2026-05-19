package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/state"
	"oh-my-gunnsama/internal/storage"
)

// WorkerFactory constructs a Worker for the given WorkerConfig. Callers (the
// orchestrator dispatcher) inject a factory through Dependencies so that
// handleRun can spawn workers without the package importing concrete adapters.
type WorkerFactory func(ctx context.Context, cfg config.WorkerConfig) (adapter.Worker, error)

func handleRun(ctx context.Context, req *Request, deps Dependencies) Response {
	goal := stringValue(req.Payload, "goal")
	if strings.TrimSpace(goal) == "" {
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorInvalidConfig, "goal is required", false), nil)
	}
	if deps.WorkerFactory == nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorInvalidConfig, "WorkerFactory is required", false), nil)
	}

	cfg, err := config.Load(configOptions(req, deps))
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), nil)
	}
	workerCfg, ok := cfg.Workers["pi"]
	if !ok {
		workerCfg = config.WorkerConfig{Kind: "pi"}
	}

	runID := generateID("run")
	sessionID := generateID("session")
	workID := generateID("work")
	warnings := []string{}

	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:      storage.EventRunCreated,
		RunID:     runID,
		Status:    "created",
		ActorType: "user",
		Attributes: map[string]string{
			"goal": truncate(goal, 500),
		},
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}

	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:      storage.EventWorkCreated,
		RunID:     runID,
		WorkID:    workID,
		Status:    "ready",
		ActorType: "run_core",
		Attributes: map[string]string{
			"goal":  truncate(goal, 500),
			"role":  "executor",
			"title": truncate(goal, 80),
		},
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}

	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:           storage.EventAgentSpawned,
		RunID:          runID,
		WorkID:         workID,
		AgentSessionID: sessionID,
		Status:         "spawning",
		ActorType:      "run_core",
		Provider:       workerCfg.Kind,
		Attributes: map[string]string{
			"role": "executor",
		},
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}

	worker, err := deps.WorkerFactory(ctx, workerCfg)
	if err != nil {
		_ = recordStorageEvent(ctx, req, deps, storage.Event{
			Type:           storage.EventAgentFailed,
			RunID:          runID,
			AgentSessionID: sessionID,
			Status:         "failed",
			ActorType:      "run_core",
			Attributes:     map[string]string{"error_message": err.Error()},
		})
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorUnknown, fmt.Sprintf("worker factory: %v", err), false), warnings)
	}

	stateStore := deps.StateStore
	if stateStore == nil && req.Project != "" {
		stateStore = state.NewStore(req.Project, state.Options{})
	}
	if stateStore != nil {
		if _, err := stateStore.Transition(sessionID, state.TransitionInput{
			To:        state.LifecycleRunning,
			Context:   "run_execute",
			OwnerID:   deps.OwnerID,
			RequestID: req.RequestID,
			Now:       now(deps),
		}); err != nil {
			warnings = append(warnings, "state transition (running) skipped: "+err.Error())
		}
	}
	if err := recordStorageEvent(ctx, req, deps, storage.Event{
		Type:           storage.EventRunStatusChanged,
		RunID:          runID,
		AgentSessionID: sessionID,
		Status:         "executing",
		Lifecycle:      string(state.LifecycleRunning),
		ActorType:      "run_core",
	}); err != nil {
		warnings = append(warnings, storageWarning(err))
	}

	_ = recordStorageEvent(ctx, req, deps, storage.Event{
		Type:      storage.EventWorkStarted,
		RunID:     runID,
		WorkID:    workID,
		Status:    "running",
		ActorType: "run_core",
	})

	spawnCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	spec := adapter.WorkerSpec{
		Goal:       goal,
		ProjectDir: req.CWD,
		AllowTools: []string{"read", "bash", "edit", "write"},
	}
	if err := worker.Spawn(spawnCtx, spec); err != nil {
		_ = recordStorageEvent(ctx, req, deps, storage.Event{
			Type:           storage.EventAgentFailed,
			RunID:          runID,
			AgentSessionID: sessionID,
			Status:         "failed",
			ActorType:      "run_core",
			Attributes:     map[string]string{"error_message": err.Error()},
		})
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorUnknown, fmt.Sprintf("worker spawn: %v", err), false), warnings)
	}

	eventCount := 0
	for evt := range worker.Events() {
		eventCount++
		storageEvent := workerEventToStorage(evt, runID, workID, sessionID)
		if err := recordStorageEvent(ctx, req, deps, storageEvent); err != nil {
			warnings = append(warnings, storageWarning(err))
		}
	}

	result, waitErr := worker.Wait(ctx)
	finalLifecycle := state.LifecycleFinished
	success := waitErr == nil && result.Success
	if !success {
		finalLifecycle = state.LifecycleFailed
	}

	workEventType := storage.EventWorkCompleted
	workFinalStatus := "completed"
	if !success {
		workEventType = storage.EventWorkFailed
		workFinalStatus = "failed"
	}
	_ = recordStorageEvent(ctx, req, deps, storage.Event{
		Type:      workEventType,
		RunID:     runID,
		WorkID:    workID,
		Status:    workFinalStatus,
		ActorType: "run_core",
	})

	if stateStore != nil {
		if _, err := stateStore.Transition(sessionID, state.TransitionInput{
			To:        finalLifecycle,
			Context:   "run_terminate",
			OwnerID:   deps.OwnerID,
			RequestID: req.RequestID,
			Now:       now(deps),
		}); err != nil {
			warnings = append(warnings, "state transition (terminal) skipped: "+err.Error())
		}
	}
	finalStatus := "completed"
	if !success {
		finalStatus = "failed"
	}
	_ = recordStorageEvent(ctx, req, deps, storage.Event{
		Type:           storage.EventRunStatusChanged,
		RunID:          runID,
		AgentSessionID: sessionID,
		Status:         finalStatus,
		Lifecycle:      string(finalLifecycle),
		ActorType:      "run_core",
		Attributes:     map[string]string{"exit_code": fmt.Sprintf("%d", result.ExitCode)},
	})
	_ = recordStorageEvent(ctx, req, deps, storage.Event{
		Type:           storage.EventAgentStopped,
		RunID:          runID,
		AgentSessionID: sessionID,
		Status:         finalStatus,
		Lifecycle:      string(finalLifecycle),
		ActorType:      "run_core",
	})

	summary := fmt.Sprintf("run_id=%s work_id=%s session_id=%s status=%s events=%d", runID, workID, sessionID, finalLifecycle, eventCount)
	if !success {
		errMsg := summary
		if waitErr != nil {
			errMsg += " err=" + waitErr.Error()
		}
		failWarn := "run failed"
		if result.Reason != "" {
			failWarn += ": " + result.Reason
		}
		return protocol.OKResponse(req, protocol.ActionWarn, errMsg, append(warnings, failWarn))
	}
	return protocol.OKResponse(req, protocol.ActionNone, summary, warnings)
}

// workerEventToStorage maps a worker event to a storage Event. Tool execution
// kinds get dedicated tool_call_* event types; everything else is recorded as
// an agent heartbeat carrying the original kind in attributes.
func workerEventToStorage(evt adapter.WorkerEvent, runID, workID, sessionID string) storage.Event {
	storageEvent := storage.Event{
		Timestamp:      evt.Timestamp,
		RunID:          runID,
		WorkID:         workID,
		AgentSessionID: sessionID,
		ActorType:      "worker",
		Attributes:     map[string]string{"kind": evt.Kind},
	}
	if evt.ToolName != "" {
		storageEvent.Attributes["tool"] = evt.ToolName
	}
	if evt.Message != "" {
		storageEvent.Attributes["message"] = truncate(evt.Message, 500)
	}
	switch evt.Kind {
	case "tool_execution_start":
		storageEvent.Type = storage.EventToolCallStarted
		storageEvent.Status = "started"
	case "tool_execution_end":
		storageEvent.Type = storage.EventToolCallCompleted
		storageEvent.Status = "completed"
	default:
		storageEvent.Type = storage.EventAgentHeartbeat
		storageEvent.Status = "running"
	}
	return storageEvent
}

func generateID(prefix string) string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return prefix + "_unknown"
	}
	return prefix + "_" + hex.EncodeToString(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
