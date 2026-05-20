package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/state"
	"oh-my-gunnsama/internal/storage"
	"oh-my-gunnsama/internal/verify"
)

// WorkerFactory constructs a Worker for the given WorkerConfig. Callers (the
// orchestrator dispatcher) inject a factory through Dependencies so that the
// Run pipeline can spawn workers without the package importing concrete
// adapters.
type WorkerFactory func(ctx context.Context, cfg config.WorkerConfig) (adapter.Worker, error)

// runContext carries mutable state across the Run pipeline phases. It is a
// plain data carrier with one convenience method (emit). Single-goroutine
// access; no synchronization. Fields are written by one phase and read by
// the next; ownership is documented inline.
type runContext struct {
	Request *Request
	Deps    Dependencies

	// Set by preWorker (validation, IDs, factory, state transition, Spawn).
	Goal       string
	WorkerCfg  config.WorkerConfig
	RunID      string
	WorkID     string
	SessionID  string
	Worker     adapter.Worker
	StateStore *state.Store
	Cancel     context.CancelFunc // cancels Spawn ctx; runWithWorker defers

	// Set by runWithWorker (drain, wait, verify, terminate).
	WorkerResult     adapter.WorkerResult
	WorkerWaitErr    error
	WorkerSuccess    bool
	WorkerEventCount int
	VerifyResult     *verify.VerificationResult
	Success          bool
	FinalLifecycle   state.Lifecycle

	// Accumulated by all phases (soft warnings).
	Warnings []string
}

func newRunContext(req *Request, deps Dependencies) *runContext {
	return &runContext{Request: req, Deps: deps, Warnings: []string{}}
}

// emit records a storage event, accumulating soft warnings on failure.
// Centralises the "record-or-warn" boilerplate previously duplicated
// across handleRun.
func (rc *runContext) emit(ctx context.Context, ev storage.Event) {
	if err := recordStorageEvent(ctx, rc.Request, rc.Deps, ev); err != nil {
		rc.Warnings = append(rc.Warnings, storageWarning(err))
	}
}

// preWorkerErr is a sentinel that lets the bail-out zone (preWorker) return
// a protocol.ErrorClass with its message. handleRun unwraps it to build the
// correct ErrorResponse class. Defined here rather than as a free error so
// that the class survives errors.As across the phase boundary.
type preWorkerErr struct {
	class protocol.ErrorClass
	msg   string
}

func (e *preWorkerErr) Error() string { return e.msg }

func bailErr(class protocol.ErrorClass, msg string) error {
	return &preWorkerErr{class: class, msg: msg}
}

// handleRun is the orchestrator entry for protocol.EventOnRun. It is split
// into two phases: preWorker (may bail with ErrorResponse) and runWithWorker
// (must complete to terminal events). The split mirrors the natural seam in
// the lifecycle: once Spawn succeeds, the run is committed to its terminal
// state emissions.
func handleRun(ctx context.Context, req *Request, deps Dependencies) Response {
	rc := newRunContext(req, deps)

	if err := preWorker(ctx, rc); err != nil {
		var pwe *preWorkerErr
		class := protocol.ErrorUnknown
		msg := err.Error()
		if errors.As(err, &pwe) {
			class = pwe.class
			msg = pwe.msg
		}
		return protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(class, msg, false), rc.Warnings)
	}

	runWithWorker(ctx, rc)
	return buildRunResponse(req, rc)
}

// buildRunResponse formats the final Response from the populated runContext.
// Pure projection: no side effects, no event emission.
func buildRunResponse(req *Request, rc *runContext) Response {
	verificationLabel := "skipped"
	if rc.VerifyResult != nil {
		if rc.VerifyResult.Passed {
			verificationLabel = "verified"
		} else {
			verificationLabel = "failed"
		}
	}
	summary := fmt.Sprintf(
		"run_id=%s work_id=%s session_id=%s status=%s events=%d verification=%s",
		rc.RunID, rc.WorkID, rc.SessionID, rc.FinalLifecycle, rc.WorkerEventCount, verificationLabel,
	)
	if rc.Success {
		return protocol.OKResponse(req, protocol.ActionNone, summary, rc.Warnings)
	}
	errMsg := summary
	if rc.WorkerWaitErr != nil {
		errMsg += " err=" + rc.WorkerWaitErr.Error()
	}
	failWarn := "run failed"
	if rc.WorkerResult.Reason != "" {
		failWarn += ": " + rc.WorkerResult.Reason
	}
	return protocol.OKResponse(req, protocol.ActionWarn, errMsg, append(rc.Warnings, failWarn))
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
