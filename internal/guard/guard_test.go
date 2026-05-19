package guard

import (
	"errors"
	"strings"
	"testing"
)

func TestEvaluateMergesDecisionsWithBlockPrecedence(t *testing.T) {
	result := Evaluate(GuardInput{Tool: "write", Project: "/repo"}, []Rule{
		staticRule("allow-rule", Decision(GuardAllow, SeverityInfo, "ok", "allow-rule")),
		staticRule("warn-rule", Decision(GuardWarn, SeverityWarning, "careful", "warn-rule")),
		staticRule("block-rule", Decision(GuardBlock, SeverityDanger, "blocked", "block-rule")),
	}, nil)

	if result.Decision.Action != GuardBlock || result.Decision.RuleID != "block-rule" {
		t.Fatalf("decision = %#v, want block precedence", result.Decision)
	}
	if len(result.Decisions) != 3 {
		t.Fatalf("decisions = %#v, want all rule results retained", result.Decisions)
	}
}

func TestDestructiveToolGuardErrorBlocks(t *testing.T) {
	result := Evaluate(GuardInput{Tool: "write", Project: "/repo"}, []Rule{
		errorRule("explode", errors.New("boom")),
	}, nil)

	if result.Decision.Action != GuardBlock || result.Decision.Severity != SeverityDanger {
		t.Fatalf("decision = %#v, want destructive guard error block", result.Decision)
	}
	if !strings.Contains(result.Decision.Reason, "boom") {
		t.Fatalf("reason = %q, want error message", result.Decision.Reason)
	}
}

func TestNonDestructiveToolGuardErrorAllowsWithWarning(t *testing.T) {
	result := Evaluate(GuardInput{Tool: "read", Project: "/repo"}, []Rule{
		errorRule("explode", errors.New("boom")),
	}, nil)

	if result.Decision.Action != GuardAllow || result.Decision.Severity != SeverityWarning {
		t.Fatalf("decision = %#v, want allow warning", result.Decision)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "boom") {
		t.Fatalf("warnings = %#v, want guard error warning", result.Warnings)
	}
}

func TestSuppressionSkipsMatchingRule(t *testing.T) {
	result := Evaluate(GuardInput{Tool: "write", Project: "/repo", CWD: "/repo", Args: map[string]any{"path": "testdata/generated/out.txt"}}, []Rule{
		staticRule("write-existing-file", Decision(GuardBlock, SeverityDanger, "blocked", "write-existing-file")),
	}, []Suppression{{
		RuleID: "write-existing-file",
		Reason: "generated fixture overwrite",
		Scope:  "project",
		Paths:  []string{"testdata/generated/**"},
		Tools:  []string{"write"},
	}})

	if result.Decision.Action != GuardAllow {
		t.Fatalf("decision = %#v, want allow because rule suppressed", result.Decision)
	}
	if len(result.SuppressedRules) != 1 || result.SuppressedRules[0] != "write-existing-file" {
		t.Fatalf("SuppressedRules = %#v", result.SuppressedRules)
	}
}

func TestInvalidSuppressionWithoutReasonIsIgnored(t *testing.T) {
	result := Evaluate(GuardInput{Tool: "write", Project: "/repo", Args: map[string]any{"path": "testdata/generated/out.txt"}}, []Rule{
		staticRule("write-existing-file", Decision(GuardBlock, SeverityDanger, "blocked", "write-existing-file")),
	}, []Suppression{{RuleID: "write-existing-file", Scope: "project", Paths: []string{"testdata/generated/**"}, Tools: []string{"write"}}})

	if result.Decision.Action != GuardBlock {
		t.Fatalf("decision = %#v, want block because invalid suppression ignored", result.Decision)
	}
}

func staticRule(id string, decision GuardDecision) Rule {
	return RuleFunc{id: id, fn: func(input GuardInput) (GuardDecision, error) { return decision, nil }}
}

func errorRule(id string, err error) Rule {
	return RuleFunc{id: id, fn: func(input GuardInput) (GuardDecision, error) { return GuardDecision{}, err }}
}
