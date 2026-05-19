package guard

import (
	"fmt"
	"path/filepath"
	"strings"
)

type GuardInput struct {
	RequestID string         `json:"request_id"`
	Project   string         `json:"project"`
	CWD       string         `json:"cwd"`
	Tool      string         `json:"tool"`
	Args      map[string]any `json:"args"`
	Agent     string         `json:"agent,omitempty"`
	Autonomy  string         `json:"autonomy,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
}

type GuardAction string

const (
	GuardAllow GuardAction = "allow"
	GuardWarn  GuardAction = "warn"
	GuardBlock GuardAction = "block"
)

type GuardSeverity string

const (
	SeverityInfo    GuardSeverity = "info"
	SeverityWarning GuardSeverity = "warning"
	SeverityDanger  GuardSeverity = "danger"
)

type GuardDecision struct {
	Action   GuardAction   `json:"action"`
	Severity GuardSeverity `json:"severity"`
	Reason   string        `json:"reason"`
	RuleID   string        `json:"rule_id"`
}

type Result struct {
	Decision        GuardDecision
	Decisions       []GuardDecision
	Warnings        []string
	SuppressedRules []string
}

type Rule interface {
	ID() string
	Evaluate(GuardInput) (GuardDecision, error)
}

type RuleFunc struct {
	id string
	fn func(GuardInput) (GuardDecision, error)
}

func (r RuleFunc) ID() string { return r.id }
func (r RuleFunc) Evaluate(input GuardInput) (GuardDecision, error) {
	return r.fn(input)
}

type Suppression struct {
	RuleID string
	Reason string
	Scope  string
	Paths  []string
	Tools  []string
}

func Decision(action GuardAction, severity GuardSeverity, reason, ruleID string) GuardDecision {
	return GuardDecision{Action: action, Severity: severity, Reason: reason, RuleID: ruleID}
}

func Evaluate(input GuardInput, rules []Rule, suppressions []Suppression) Result {
	result := Result{Decision: Decision(GuardAllow, SeverityInfo, "allowed", "default-allow")}
	for _, rule := range rules {
		if suppressionApplies(input, rule.ID(), suppressions) {
			result.SuppressedRules = append(result.SuppressedRules, rule.ID())
			continue
		}
		decision, err := rule.Evaluate(input)
		if err != nil {
			decision = errorDecision(input, rule.ID(), err)
			if !isDestructive(input.Tool) {
				result.Warnings = append(result.Warnings, decision.Reason)
			}
		}
		if decision.Action == "" {
			decision = Decision(GuardAllow, SeverityInfo, "allowed", rule.ID())
		}
		result.Decisions = append(result.Decisions, decision)
		if stricter(decision.Action, result.Decision.Action) {
			result.Decision = decision
		} else if decision.Action == result.Decision.Action && severityRank(decision.Severity) > severityRank(result.Decision.Severity) {
			result.Decision = decision
		}
	}
	return result
}

func errorDecision(input GuardInput, ruleID string, err error) GuardDecision {
	reason := fmt.Sprintf("guard rule %q failed: %v", ruleID, err)
	if isDestructive(input.Tool) {
		return Decision(GuardBlock, SeverityDanger, reason, ruleID)
	}
	return Decision(GuardAllow, SeverityWarning, reason, ruleID)
}

func isDestructive(tool string) bool {
	switch strings.ToLower(tool) {
	case "write", "edit", "apply_patch", "bash", "shell_mutation", "delete", "rm":
		return true
	default:
		return false
	}
}

func stricter(left, right GuardAction) bool {
	return actionRank(left) > actionRank(right)
}

func actionRank(action GuardAction) int {
	switch action {
	case GuardBlock:
		return 3
	case GuardWarn:
		return 2
	case GuardAllow:
		return 1
	default:
		return 0
	}
}

func severityRank(severity GuardSeverity) int {
	switch severity {
	case SeverityDanger:
		return 3
	case SeverityWarning:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 0
	}
}

func suppressionApplies(input GuardInput, ruleID string, suppressions []Suppression) bool {
	for _, suppression := range suppressions {
		if !validSuppression(suppression) || suppression.RuleID != ruleID {
			continue
		}
		if len(suppression.Tools) > 0 && !containsString(suppression.Tools, input.Tool) {
			continue
		}
		if len(suppression.Paths) > 0 && !pathMatches(inputPath(input), suppression.Paths) {
			continue
		}
		return true
	}
	return false
}

func validSuppression(s Suppression) bool {
	return s.RuleID != "" && s.Reason != "" && s.Scope == "project" && (len(s.Paths) > 0 || len(s.Tools) > 0)
}

func inputPath(input GuardInput) string {
	for _, key := range []string{"path", "file", "file_path"} {
		if value, ok := input.Args[key].(string); ok {
			return filepath.ToSlash(value)
		}
	}
	return ""
}

func pathMatches(path string, patterns []string) bool {
	if path == "" {
		return false
	}
	path = filepath.ToSlash(path)
	for _, pattern := range patterns {
		pattern = filepath.ToSlash(pattern)
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "**")
			if strings.HasPrefix(path, prefix) {
				return true
			}
			continue
		}
		if ok, _ := filepath.Match(pattern, path); ok {
			return true
		}
		if pattern == path {
			return true
		}
	}
	return false
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
