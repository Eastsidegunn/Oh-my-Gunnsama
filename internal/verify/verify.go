package verify

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"oh-my-gunnsama/internal/storage"
)

type VerificationPlan struct {
	Checks []VerificationCheck `json:"checks" yaml:"checks"`
}

type VerificationCheck struct {
	ID           string        `json:"id" yaml:"id"`
	Name         string        `json:"name" yaml:"name"`
	Command      []string      `json:"command" yaml:"command"`
	Shell        bool          `json:"shell,omitempty" yaml:"shell,omitempty"`
	ShellCommand string        `json:"shell_command,omitempty" yaml:"shell_command,omitempty"`
	CWD          string        `json:"cwd,omitempty" yaml:"cwd,omitempty"`
	Timeout      time.Duration `json:"timeout" yaml:"timeout"`
	Claim        string        `json:"claim" yaml:"claim"`
	Required     bool          `json:"required" yaml:"required"`
	EvidenceKind EvidenceKind  `json:"evidence_kind" yaml:"evidence_kind"`
	Reason       string        `json:"reason,omitempty" yaml:"reason,omitempty"`
	Artifacts    []string      `json:"artifacts,omitempty" yaml:"artifacts,omitempty"`
}

type EvidenceKind string

const (
	EvidenceCommand  EvidenceKind = "command"
	EvidenceManual   EvidenceKind = "manual"
	EvidenceArtifact EvidenceKind = "artifact"
)

type CheckEvidence struct {
	CheckID         string        `json:"check_id"`
	Kind            EvidenceKind  `json:"kind"`
	Passed          bool          `json:"passed"`
	Command         []string      `json:"command,omitempty"`
	Shell           bool          `json:"shell,omitempty"`
	ExitCode        int           `json:"exit_code,omitempty"`
	Stdout          string        `json:"stdout,omitempty"`
	Stderr          string        `json:"stderr,omitempty"`
	OutputTruncated bool          `json:"output_truncated,omitempty"`
	Artifacts       []string      `json:"artifacts,omitempty"`
	StartedAt       time.Time     `json:"started_at"`
	Duration        time.Duration `json:"duration"`
	Reason          string        `json:"reason,omitempty"`
}

type VerificationResult struct {
	Passed   bool            `json:"passed"`
	Evidence []CheckEvidence `json:"evidence"`
	Summary  string          `json:"summary"`
}

type Options struct {
	Runner         Runner
	StorageSink    storage.Sink
	RunID          string
	WorkID         string
	VerificationID string
	ActorID        string
	ProjectRoot    string
	Now            func() time.Time
}

type Runner interface {
	Run(context.Context, VerificationCheck) (CommandResult, error)
}

type CommandResult struct {
	ExitCode        int
	Stdout          string
	Stderr          string
	Duration        time.Duration
	OutputTruncated bool
}

func RunPlan(ctx context.Context, plan VerificationPlan, options Options) (VerificationResult, error) {
	if options.Runner == nil {
		options.Runner = ExecRunner{}
	}
	result := VerificationResult{Passed: true}
	if len(plan.Checks) == 0 {
		return VerificationResult{Passed: false, Summary: "no evidence collected"}, nil
	}
	for _, check := range plan.Checks {
		evidence, err := runCheck(ctx, check, options.Runner)
		if err != nil {
			return VerificationResult{}, err
		}
		result.Evidence = append(result.Evidence, evidence)
		if check.Required && !evidence.Passed {
			result.Passed = false
		}
	}
	if len(result.Evidence) == 0 {
		result.Passed = false
		result.Summary = "no evidence collected"
		return result, nil
	}
	if result.Passed {
		result.Summary = "verification passed with evidence"
	} else {
		result.Summary = "verification failed with evidence"
	}
	if err := recordVerificationStorage(ctx, result, options); err != nil {
		return VerificationResult{}, err
	}
	return result, nil
}

func recordVerificationStorage(ctx context.Context, result VerificationResult, options Options) error {
	if options.StorageSink == nil {
		return nil
	}
	status := "failed"
	eventType := storage.EventVerificationFailed
	if result.Passed {
		status = "passed"
		eventType = storage.EventVerificationPassed
	}
	recorder := storage.Recorder{Sink: options.StorageSink, Now: options.Now}
	if err := recorder.Record(ctx, storage.Event{
		Type:           eventType,
		RunID:          options.RunID,
		WorkID:         options.WorkID,
		VerificationID: options.VerificationID,
		ActorType:      "verifier",
		ActorID:        options.ActorID,
		Status:         status,
		ProjectRoot:    options.ProjectRoot,
		Attributes:     map[string]string{"summary": result.Summary},
	}); err != nil {
		return err
	}
	for _, evidence := range result.Evidence {
		if err := recorder.Record(ctx, storage.Event{
			Type:           storage.EventEvidenceCreated,
			RunID:          options.RunID,
			WorkID:         options.WorkID,
			VerificationID: options.VerificationID,
			ActorType:      "verifier",
			ActorID:        options.ActorID,
			Status:         evidenceStatus(evidence),
			ProjectRoot:    options.ProjectRoot,
			Attributes: map[string]string{
				"check_id": evidence.CheckID,
				"kind":     string(evidence.Kind),
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

func evidenceStatus(evidence CheckEvidence) string {
	if evidence.Passed {
		return "passed"
	}
	return "failed"
}

func runCheck(ctx context.Context, check VerificationCheck, runner Runner) (CheckEvidence, error) {
	kind := check.EvidenceKind
	if kind == "" {
		kind = EvidenceCommand
	}
	started := time.Now().UTC()
	switch kind {
	case EvidenceCommand:
		if len(check.Command) == 0 && !check.Shell {
			return CheckEvidence{}, fmt.Errorf("command evidence requires command argv")
		}
		commandCtx := ctx
		cancel := func() {}
		if check.Timeout > 0 {
			commandCtx, cancel = context.WithTimeout(ctx, check.Timeout)
		}
		defer cancel()
		commandResult, err := runner.Run(commandCtx, check)
		passed := err == nil && commandResult.ExitCode == 0
		stderr := commandResult.Stderr
		if err != nil {
			stderr = strings.TrimSpace(strings.Join([]string{stderr, err.Error()}, " "))
		}
		return CheckEvidence{
			CheckID:         check.ID,
			Kind:            EvidenceCommand,
			Passed:          passed,
			Command:         commandForEvidence(check),
			Shell:           check.Shell,
			ExitCode:        commandResult.ExitCode,
			Stdout:          commandResult.Stdout,
			Stderr:          stderr,
			OutputTruncated: commandResult.OutputTruncated,
			StartedAt:       started,
			Duration:        commandResult.Duration,
		}, nil
	case EvidenceManual:
		if check.Reason == "" {
			return CheckEvidence{}, fmt.Errorf("manual evidence reason is required")
		}
		return CheckEvidence{CheckID: check.ID, Kind: EvidenceManual, Passed: true, StartedAt: started, Duration: time.Since(started), Reason: check.Reason}, nil
	case EvidenceArtifact:
		if len(check.Artifacts) == 0 {
			return CheckEvidence{}, fmt.Errorf("artifact evidence requires artifact path")
		}
		return CheckEvidence{CheckID: check.ID, Kind: EvidenceArtifact, Passed: true, StartedAt: started, Duration: time.Since(started), Artifacts: append([]string(nil), check.Artifacts...)}, nil
	default:
		return CheckEvidence{}, fmt.Errorf("unsupported evidence kind %q", kind)
	}
}

func commandForEvidence(check VerificationCheck) []string {
	if check.Shell {
		if check.ShellCommand != "" {
			return []string{check.ShellCommand}
		}
		return append([]string(nil), check.Command...)
	}
	return append([]string(nil), check.Command...)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, check VerificationCheck) (CommandResult, error) {
	start := time.Now()
	var cmd *exec.Cmd
	if check.Shell {
		command := check.ShellCommand
		if command == "" {
			command = strings.Join(check.Command, " ")
		}
		args := []string{"-c", command}
		if check.ShellCommand != "" && len(check.Command) > 0 {
			args = append(args, "--")
			args = append(args, check.Command...)
		}
		cmd = exec.CommandContext(ctx, "/bin/sh", args...)
	} else {
		if len(check.Command) == 0 {
			return CommandResult{}, fmt.Errorf("command argv is required")
		}
		cmd = exec.CommandContext(ctx, check.Command[0], check.Command[1:]...)
	}
	if check.CWD != "" {
		cmd.Dir = check.CWD
	}
	stdout, err := cmd.Output()
	result := CommandResult{Stdout: string(stdout), Duration: time.Since(start)}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
			result.Stderr = string(exitErr.Stderr)
			return result, nil
		}
		return result, err
	}
	return result, nil
}
