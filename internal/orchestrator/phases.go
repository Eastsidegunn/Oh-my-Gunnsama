package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/state"
	"oh-my-gunnsama/internal/storage"
	"oh-my-gunnsama/internal/verify"
)

// preWorker is the "bail allowed" phase. It validates input, generates IDs,
// emits creation/spawn events, calls the WorkerFactory, transitions state
// to running, emits the executing markers, and finally calls Worker.Spawn.
// Returns a *preWorkerErr (unwrappable via errors.As) on any hard failure;
// handleRun translates that into a protocol.ErrorResponse with the correct
// ErrorClass.
//
// Once preWorker returns nil, the Run is committed: runWithWorker MUST run
// to terminal events regardless of worker outcome.
func preWorker(ctx context.Context, rc *runContext) error {
	rc.Goal = stringValue(rc.Request.Payload, "goal")
	if strings.TrimSpace(rc.Goal) == "" {
		return bailErr(protocol.ErrorInvalidConfig, "goal is required")
	}
	if rc.Deps.WorkerFactory == nil {
		return bailErr(protocol.ErrorInvalidConfig, "WorkerFactory is required")
	}

	cfg, err := config.Load(configOptions(rc.Request, rc.Deps))
	if err != nil {
		return bailErr(protocol.ErrorInvalidConfig, err.Error())
	}
	workerCfg, ok := cfg.Workers["pi"]
	if !ok {
		workerCfg = config.WorkerConfig{Kind: "pi"}
	}
	rc.WorkerCfg = workerCfg

	rc.RunID = generateID("run")
	rc.SessionID = generateID("session")
	rc.WorkID = generateID("work")

	rc.emit(ctx, storage.Event{
		Type:      storage.EventRunCreated,
		RunID:     rc.RunID,
		Status:    "created",
		ActorType: "user",
		Attributes: map[string]string{
			"goal": truncate(rc.Goal, 500),
		},
	})

	rc.emit(ctx, storage.Event{
		Type:      storage.EventWorkCreated,
		RunID:     rc.RunID,
		WorkID:    rc.WorkID,
		Status:    "ready",
		ActorType: "run_core",
		Attributes: map[string]string{
			"goal":  truncate(rc.Goal, 500),
			"role":  "executor",
			"title": truncate(rc.Goal, 80),
		},
	})

	rc.emit(ctx, storage.Event{
		Type:           storage.EventAgentSpawned,
		RunID:          rc.RunID,
		WorkID:         rc.WorkID,
		AgentSessionID: rc.SessionID,
		Status:         "spawning",
		ActorType:      "run_core",
		Provider:       workerCfg.Kind,
		Attributes:     map[string]string{"role": "executor"},
	})

	worker, err := rc.Deps.WorkerFactory(ctx, workerCfg)
	if err != nil {
		rc.emit(ctx, storage.Event{
			Type:           storage.EventAgentFailed,
			RunID:          rc.RunID,
			AgentSessionID: rc.SessionID,
			Status:         "failed",
			ActorType:      "run_core",
			Attributes:     map[string]string{"error_message": err.Error()},
		})
		return bailErr(protocol.ErrorUnknown, fmt.Sprintf("worker factory: %v", err))
	}
	rc.Worker = worker

	rc.StateStore = rc.Deps.StateStore
	if rc.StateStore == nil && rc.Request.Project != "" {
		rc.StateStore = state.NewStore(rc.Request.Project, state.Options{})
	}
	if rc.StateStore != nil {
		if _, err := rc.StateStore.Transition(rc.SessionID, state.TransitionInput{
			To:        state.LifecycleRunning,
			Context:   "run_execute",
			OwnerID:   rc.Deps.OwnerID,
			RequestID: rc.Request.RequestID,
			Now:       now(rc.Deps),
		}); err != nil {
			rc.Warnings = append(rc.Warnings, "state transition (running) skipped: "+err.Error())
		}
	}

	rc.emit(ctx, storage.Event{
		Type:           storage.EventRunStatusChanged,
		RunID:          rc.RunID,
		AgentSessionID: rc.SessionID,
		Status:         "executing",
		Lifecycle:      string(state.LifecycleRunning),
		ActorType:      "run_core",
	})

	rc.emit(ctx, storage.Event{
		Type:      storage.EventWorkStarted,
		RunID:     rc.RunID,
		WorkID:    rc.WorkID,
		Status:    "running",
		ActorType: "run_core",
	})

	spawnCtx, cancel := context.WithCancel(ctx)
	rc.Cancel = cancel
	spec := adapter.WorkerSpec{
		Goal:       rc.Goal,
		ProjectDir: rc.Request.CWD,
		AllowTools: []string{"read", "bash", "edit", "write"},
	}
	if err := rc.Worker.Spawn(spawnCtx, spec); err != nil {
		cancel()
		rc.emit(ctx, storage.Event{
			Type:           storage.EventAgentFailed,
			RunID:          rc.RunID,
			AgentSessionID: rc.SessionID,
			Status:         "failed",
			ActorType:      "run_core",
			Attributes:     map[string]string{"error_message": err.Error()},
		})
		return bailErr(protocol.ErrorUnknown, fmt.Sprintf("worker spawn: %v", err))
	}
	return nil
}

// runWithWorker is the "must complete" phase. Once preWorker has returned
// nil, this function MUST run to terminal events regardless of worker
// outcome. It drains the worker event stream, waits for termination, runs
// verification if checks were declared, and emits the terminal lifecycle
// events. Soft failures (storage, state transition, verify execution)
// accumulate as warnings in rc.Warnings; this function never returns an
// error.
func runWithWorker(ctx context.Context, rc *runContext) {
	defer rc.Cancel()

	for evt := range rc.Worker.Events() {
		rc.WorkerEventCount++
		rc.emit(ctx, workerEventToStorage(evt, rc.RunID, rc.WorkID, rc.SessionID))
	}

	result, waitErr := rc.Worker.Wait(ctx)
	rc.WorkerResult = result
	rc.WorkerWaitErr = waitErr
	rc.WorkerSuccess = waitErr == nil && result.Success

	if rc.WorkerSuccess {
		rc.emit(ctx, storage.Event{
			Type:      storage.EventWorkCompleted,
			RunID:     rc.RunID,
			WorkID:    rc.WorkID,
			Status:    "completed",
			ActorType: "run_core",
		})
	} else {
		rc.emit(ctx, storage.Event{
			Type:      storage.EventWorkFailed,
			RunID:     rc.RunID,
			WorkID:    rc.WorkID,
			Status:    "failed",
			ActorType: "run_core",
		})
	}

	checks := parseChecks(rc.Request.Payload)
	if rc.WorkerSuccess && len(checks) > 0 {
		verificationID := generateID("verification")
		plan := buildVerificationPlan(checks, rc.Request.CWD)
		options := verify.Options{
			StorageSink:    rc.Deps.StorageSink,
			RunID:          rc.RunID,
			WorkID:         rc.WorkID,
			VerificationID: verificationID,
			ActorID:        rc.Deps.OwnerID,
			ProjectRoot:    rc.Request.Project,
			Now:            rc.Deps.Now,
		}
		vr, vErr := verify.RunPlan(ctx, plan, options)
		if vErr != nil {
			// User explicitly opted into verification via --check; if the
			// verifier itself cannot execute (storage rejection, runner
			// error, malformed plan), we have no proof the run succeeded.
			// Treat as failed verification so the run terminates as failed
			// rather than silently passing.
			rc.VerifyResult = &verify.VerificationResult{
				Passed:  false,
				Summary: "verification execution error: " + vErr.Error(),
			}
			rc.Warnings = append(rc.Warnings, "verification execution error: "+vErr.Error())
		} else {
			rc.VerifyResult = &vr
		}
	}

	rc.Success = rc.WorkerSuccess
	if rc.VerifyResult != nil && !rc.VerifyResult.Passed {
		rc.Success = false
	}
	rc.FinalLifecycle = state.LifecycleFinished
	if !rc.Success {
		rc.FinalLifecycle = state.LifecycleFailed
	}

	if rc.StateStore != nil {
		if _, err := rc.StateStore.Transition(rc.SessionID, state.TransitionInput{
			To:        rc.FinalLifecycle,
			Context:   "run_terminate",
			OwnerID:   rc.Deps.OwnerID,
			RequestID: rc.Request.RequestID,
			Now:       now(rc.Deps),
		}); err != nil {
			rc.Warnings = append(rc.Warnings, "state transition (terminal) skipped: "+err.Error())
		}
	}
	finalStatus := "completed"
	if !rc.Success {
		finalStatus = "failed"
	}
	rc.emit(ctx, storage.Event{
		Type:           storage.EventRunStatusChanged,
		RunID:          rc.RunID,
		AgentSessionID: rc.SessionID,
		Status:         finalStatus,
		Lifecycle:      string(rc.FinalLifecycle),
		ActorType:      "run_core",
		Attributes:     map[string]string{"exit_code": fmt.Sprintf("%d", result.ExitCode)},
	})
	rc.emit(ctx, storage.Event{
		Type:           storage.EventAgentStopped,
		RunID:          rc.RunID,
		AgentSessionID: rc.SessionID,
		Status:         finalStatus,
		Lifecycle:      string(rc.FinalLifecycle),
		ActorType:      "run_core",
	})
}
