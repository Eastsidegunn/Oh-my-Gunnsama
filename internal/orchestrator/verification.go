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

// buildVerificationCheck constructs a VerificationCheck from a user-supplied
// spec string. Security-critical: all user input flows here. Untrusted strings
// must NEVER be interpolated into shell commands; they must be passed either
// as argv elements (Shell=false) or as positional sh-c parameters
// (Shell=true with values in Command[], referenced as $1/$2/...).
func buildVerificationCheck(spec, cwd string, idx int) verify.VerificationCheck {
	base := verify.VerificationCheck{
		ID:           fmt.Sprintf("check_%d", idx),
		Required:     true,
		EvidenceKind: verify.EvidenceCommand,
		CWD:          cwd,
		Timeout:      10 * time.Second,
	}

	parts := strings.SplitN(spec, ":", 3)
	if len(parts) < 2 {
		base.Name = spec
		base.Shell = false
		base.Command = []string{"false"}
		base.Claim = fmt.Sprintf("malformed check spec %q (missing colon)", spec)
		return base
	}

	kind := parts[0]
	switch kind {
	case "file_exists":
		path := parts[1]
		base.Name = "file_exists:" + path
		base.Shell = false
		base.Command = []string{"test", "-f", path}
		base.Claim = fmt.Sprintf("file %q exists", path)

	case "file_content":
		path := parts[1]
		if len(parts) < 3 {
			base.Name = "file_content:malformed"
			base.Shell = false
			base.Command = []string{"false"}
			base.Claim = fmt.Sprintf("file_content needs expected value (spec=%q)", spec)
			return base
		}
		expected := parts[2]
		base.Name = "file_content:" + path
		base.Shell = true
		base.ShellCommand = `[ "$(cat -- "$1")" = "$2" ]`
		base.Command = []string{path, expected}
		base.Claim = fmt.Sprintf("file %q content == %q", path, expected)

	case "bash":
		bashCmd := parts[1]
		if len(parts) >= 3 {
			bashCmd = parts[1] + ":" + parts[2]
		}
		base.Name = "bash:" + bashCmd
		base.Shell = true
		base.ShellCommand = bashCmd
		base.Claim = fmt.Sprintf("shell %q exits 0", bashCmd)

	default:
		base.Name = "unknown:" + kind
		base.Shell = false
		base.Command = []string{"false"}
		base.Claim = fmt.Sprintf("unknown check kind %q in spec %q", kind, spec)
	}
	return base
}
