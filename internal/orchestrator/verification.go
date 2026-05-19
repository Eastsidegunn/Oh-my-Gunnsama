package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"oh-my-gunnsama/internal/verify"
)

func parseChecks(payload map[string]any) []string {
	raw, ok := payload["checks"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

func buildVerificationPlan(checks []string, cwd string) verify.VerificationPlan {
	plan := verify.VerificationPlan{}
	for i, spec := range checks {
		plan.Checks = append(plan.Checks, buildVerificationCheck(spec, cwd, i))
	}
	return plan
}

func buildVerificationCheck(spec, cwd string, idx int) verify.VerificationCheck {
	parts := strings.SplitN(spec, ":", 3)
	if len(parts) < 2 {
		return verify.VerificationCheck{
			ID:           fmt.Sprintf("check_%d", idx),
			Name:         spec,
			Shell:        true,
			ShellCommand: fmt.Sprintf("echo 'malformed check spec: %s' >&2; exit 1", spec),
			Claim:        spec,
			Required:     true,
			EvidenceKind: verify.EvidenceCommand,
			CWD:          cwd,
			Timeout:      10 * time.Second,
		}
	}
	kind := parts[0]
	var shellCmd, name, claim string
	switch kind {
	case "file_exists":
		path := parts[1]
		shellCmd = fmt.Sprintf("test -f %q", path)
		name = "file_exists:" + path
		claim = fmt.Sprintf("file %q exists", path)
	case "file_content":
		path := parts[1]
		if len(parts) < 3 {
			shellCmd = "echo 'file_content needs expected value' >&2; exit 1"
			name = "file_content:malformed"
			claim = spec
		} else {
			expected := parts[2]
			shellCmd = fmt.Sprintf("[ \"$(cat %q)\" = %q ]", path, expected)
			name = "file_content:" + path
			claim = fmt.Sprintf("file %q content == %q", path, expected)
		}
	case "bash":
		bashCmd := parts[1]
		if len(parts) >= 3 {
			bashCmd = parts[1] + ":" + parts[2]
		}
		shellCmd = bashCmd
		name = "bash:" + bashCmd
		claim = fmt.Sprintf("shell %q exits 0", bashCmd)
	default:
		shellCmd = fmt.Sprintf("echo 'unknown check kind: %s' >&2; exit 1", kind)
		name = "unknown:" + kind
		claim = spec
	}
	return verify.VerificationCheck{
		ID:           fmt.Sprintf("check_%d", idx),
		Name:         name,
		Shell:        true,
		ShellCommand: shellCmd,
		Claim:        claim,
		Required:     true,
		EvidenceKind: verify.EvidenceCommand,
		CWD:          cwd,
		Timeout:      10 * time.Second,
	}
}
