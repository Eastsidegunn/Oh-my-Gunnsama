package pi

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
)

// Driver spawns the pi-coding-agent binary in `--mode json` and exposes its
// JSONL event stream as an adapter.Worker. A Driver is single-use: each
// instance may be Spawned at most once. Reuse requires constructing a new
// Driver via New.
//
// Lifecycle (Spawn → Events → Wait):
//   - Spawn forks three goroutines: a stderr drainer, a stdout scanner that
//     parses each JSONL line via ParseLine and pushes WorkerEvents on the
//     buffered Events channel, and a wait-reaper.
//   - The events channel is closed by the stdout scanner when the subprocess
//     closes stdout (i.e., when it exits), so consumers can `for evt := range
//     d.Events()` cleanly.
//   - The wait-reaper orders pipe drainage strictly before cmd.Wait (per
//     os/exec docs, calling Wait while pipes are still being read is a race);
//     then it publishes the WorkerResult and closes the internal done channel
//     that gates Wait.
type Driver struct {
	binaryPath string
	provider   string
	model      string
	apiKey     string

	mu        sync.Mutex
	cmd       *exec.Cmd
	events    chan adapter.WorkerEvent
	stderrBuf strings.Builder
	done      chan struct{}
	result    adapter.WorkerResult
	waitErr   error
	spawned   bool
}

// New constructs a Driver using the supplied WorkerConfig. The pi binary is
// located via DiscoverBinary (cfg.BinaryPath wins; otherwise $PATH is
// consulted). Provider and model are hard-wired to Anthropic Claude for v0.1;
// the API key is sourced from ANTHROPIC_API_KEY at construction time. Future
// versions will move both into WorkerConfig.
func New(cfg config.WorkerConfig) (*Driver, error) {
	binPath, err := DiscoverBinary(cfg.BinaryPath)
	if err != nil {
		return nil, fmt.Errorf("pi worker: %w", err)
	}
	return &Driver{
		binaryPath: binPath,
		provider:   "anthropic",
		model:      "claude-3-5-sonnet-20241022",
		apiKey:     os.Getenv("ANTHROPIC_API_KEY"),
	}, nil
}

// Spawn starts the pi subprocess and wires up event streaming. Returns an
// error on second invocation or if the subprocess could not be started.
// If Spawn returns an error, Events/Wait MUST NOT be called on this Driver.
func (d *Driver) Spawn(ctx context.Context, spec adapter.WorkerSpec) error {
	d.mu.Lock()
	if d.spawned {
		d.mu.Unlock()
		return errors.New("pi driver: already spawned")
	}
	d.spawned = true
	d.events = make(chan adapter.WorkerEvent, 64)
	d.done = make(chan struct{})
	d.mu.Unlock()

	args := []string{
		"--mode", "json",
		"--provider", d.provider,
		"--model", d.model,
		"--no-session",
	}
	if d.apiKey != "" {
		args = append(args, "--api-key", d.apiKey)
	}
	if len(spec.AllowTools) > 0 {
		args = append(args, "--tools", strings.Join(spec.AllowTools, ","))
	}
	args = append(args, spec.Goal)

	cmd := exec.CommandContext(ctx, d.binaryPath, args...)
	if spec.ProjectDir != "" {
		cmd.Dir = spec.ProjectDir
	}
	cmd.Env = os.Environ()
	for k, v := range spec.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("pi driver: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("pi driver: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("pi driver: start: %w", err)
	}

	d.mu.Lock()
	d.cmd = cmd
	d.mu.Unlock()

	// stderr drainer: best-effort append into stderrBuf under mu. Exits at EOF.
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		_, _ = io.Copy(stderrSink{d: d}, stderr)
	}()

	// stdout scanner: parses JSONL lines, pushes WorkerEvents on d.events,
	// and closes d.events when stdout EOFs.
	scanDone := make(chan struct{})
	go func() {
		defer close(scanDone)
		d.scanStdout(stdout)
	}()

	// wait-reaper: per os/exec docs, "it is incorrect to call Wait before
	// all reads from the pipe have completed", so we await both pipe
	// drainers before invoking cmd.Wait. The subprocess closing its stdout
	// (e.g., on exit or SIGKILL) is what unblocks scanStdout, so this
	// ordering is non-blocking in the steady state.
	go func() {
		<-scanDone
		<-stderrDone
		cmdErr := cmd.Wait()
		d.publishResult(cmdErr)
		close(d.done)
	}()

	return nil
}

// publishResult records the subprocess outcome under mu. Callers MUST close
// d.done after this returns (close is the happens-before edge that makes
// d.result/d.waitErr visible to Wait callers).
func (d *Driver) publishResult(cmdErr error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if cmdErr == nil {
		d.result = adapter.WorkerResult{ExitCode: 0, Success: true}
		return
	}
	exitCode := -1
	var ee *exec.ExitError
	if errors.As(cmdErr, &ee) {
		exitCode = ee.ExitCode()
	}
	d.result = adapter.WorkerResult{
		ExitCode: exitCode,
		Success:  false,
		Reason:   strings.TrimSpace(d.stderrBuf.String()),
	}
	// A non-zero exit code is the subprocess's outcome, not a Wait failure;
	// only surface waitErr when exit status is unrepresentable (signaled
	// processes report ExitCode() == -1, and genuine I/O errors aren't
	// *exec.ExitError at all).
	if exitCode == -1 {
		d.waitErr = cmdErr
	}
}

// stderrSink is an io.Writer that appends to the Driver's stderr buffer
// under mu. Value receiver keeps allocation cheap.
type stderrSink struct{ d *Driver }

func (s stderrSink) Write(p []byte) (int, error) {
	s.d.mu.Lock()
	s.d.stderrBuf.Write(p)
	s.d.mu.Unlock()
	return len(p), nil
}

// scanStdout reads JSONL from r, parses each line via ParseLine, and pushes
// WorkerEvents to d.events. Closes d.events when r EOFs or errors.
// ErrSkipLine lines (whitespace, session header) are silently dropped;
// malformed lines are recorded in stderrBuf for diagnostics. Events that
// arrive while the channel is full are dropped (also recorded) to ensure
// the scanner never blocks the subprocess.
func (d *Driver) scanStdout(r io.Reader) {
	defer close(d.events)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024) // 1 MiB max line
	seq := 0
	for scanner.Scan() {
		seq++
		evt, err := ParseLine(scanner.Bytes(), seq)
		if err != nil {
			if errors.Is(err, ErrSkipLine) {
				continue
			}
			d.mu.Lock()
			d.stderrBuf.WriteString(fmt.Sprintf("[pi-parse-error seq=%d] %v\n", seq, err))
			d.mu.Unlock()
			continue
		}
		select {
		case d.events <- evt:
		default:
			d.mu.Lock()
			d.stderrBuf.WriteString(fmt.Sprintf("[pi-event-dropped seq=%d kind=%s] events channel full\n", seq, evt.Kind))
			d.mu.Unlock()
		}
	}
}

// Events returns the read-only event channel. The channel is closed when
// the subprocess closes its stdout; consumers should range until close.
func (d *Driver) Events() <-chan adapter.WorkerEvent {
	return d.events
}

// Wait blocks until the subprocess has been reaped (and the result
// published) or ctx is cancelled. The returned error is non-nil only when
// ctx was cancelled or the subprocess terminated abnormally in a way that
// cannot be encoded as an exit code (e.g., SIGKILL).
func (d *Driver) Wait(ctx context.Context) (adapter.WorkerResult, error) {
	select {
	case <-d.done:
		d.mu.Lock()
		defer d.mu.Unlock()
		return d.result, d.waitErr
	case <-ctx.Done():
		return adapter.WorkerResult{}, ctx.Err()
	}
}

// Abort sends SIGKILL to the subprocess. Safe to call before Spawn (no-op)
// and idempotent on already-exited processes. The subprocess exit will be
// observed by the wait-reaper and reflected in the next Wait return.
func (d *Driver) Abort(ctx context.Context) error {
	d.mu.Lock()
	cmd := d.cmd
	d.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}

// Compile-time interface check.
var _ adapter.Worker = (*Driver)(nil)
