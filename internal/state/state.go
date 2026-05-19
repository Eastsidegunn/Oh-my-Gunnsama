package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Lifecycle string

const (
	LifecycleInactive  Lifecycle = "inactive"
	LifecycleRunning   Lifecycle = "running"
	LifecycleBlocked   Lifecycle = "blocked"
	LifecycleAskUser   Lifecycle = "ask_user"
	LifecycleFinished  Lifecycle = "finished"
	LifecycleFailed    Lifecycle = "failed"
	LifecycleCancelled Lifecycle = "cancelled"
)

const (
	BlockedReasonStaleOwner       = "stale_owner"
	BlockedReasonDaemonRestarted  = "daemon_restarted"
	BlockedReasonHeartbeatExpired = "heartbeat_expired"
)

type OmgErrorSummary struct {
	Class   string `json:"class,omitempty"`
	Domain  string `json:"domain,omitempty"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type StateRecord struct {
	Lifecycle     Lifecycle        `json:"lifecycle"`
	Context       string           `json:"context,omitempty"`
	SessionID     string           `json:"session_id,omitempty"`
	OwnerID       string           `json:"owner_id,omitempty"`
	RequestID     string           `json:"request_id,omitempty"`
	UpdatedAt     time.Time        `json:"updated_at"`
	HeartbeatAt   time.Time        `json:"heartbeat_at,omitempty"`
	BlockedReason string           `json:"blocked_reason,omitempty"`
	Error         *OmgErrorSummary `json:"error,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

type Options struct {
	HeartbeatTimeout time.Duration
}

type TransitionInput struct {
	To            Lifecycle
	Context       string
	OwnerID       string
	RequestID     string
	BlockedReason string
	Error         *OmgErrorSummary
	Metadata      map[string]any
	Now           time.Time
}

type Store struct {
	root             string
	heartbeatTimeout time.Duration
	failBeforeRename error
}

var (
	pathLocksMu sync.Mutex
	pathLocks   = map[string]*sync.Mutex{}
)

func NewStore(projectRoot string, options Options) *Store {
	if options.HeartbeatTimeout == 0 {
		options.HeartbeatTimeout = 120 * time.Second
	}
	return &Store{root: projectRoot, heartbeatTimeout: options.HeartbeatTimeout}
}

func (s *Store) Transition(sessionID string, input TransitionInput) (StateRecord, error) {
	if sessionID == "" {
		return StateRecord{}, fmt.Errorf("session_id is required")
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	current, err := s.Read(sessionID)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return StateRecord{}, err
		}
		current = StateRecord{Lifecycle: LifecycleInactive, SessionID: sessionID}
	}
	if err := validateTransition(current.Lifecycle, input.To); err != nil {
		return StateRecord{}, err
	}
	next := current
	next.Lifecycle = input.To
	next.SessionID = sessionID
	next.UpdatedAt = now
	if input.Context != "" {
		next.Context = input.Context
	}
	if input.OwnerID != "" {
		next.OwnerID = input.OwnerID
	}
	if input.RequestID != "" {
		next.RequestID = input.RequestID
	}
	if input.Metadata != nil {
		next.Metadata = input.Metadata
	}
	next.BlockedReason = input.BlockedReason
	next.Error = input.Error
	if input.To == LifecycleRunning {
		next.HeartbeatAt = now
	}
	if err := s.writeSession(sessionID, next); err != nil {
		return StateRecord{}, err
	}
	return next, nil
}

func (s *Store) Read(sessionID string) (StateRecord, error) {
	return readRecord(s.sessionPath(sessionID))
}

func (s *Store) ReadEffective(sessionID string) (StateRecord, error) {
	if sessionID != "" {
		record, err := s.Read(sessionID)
		if err == nil {
			return record, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return StateRecord{}, err
		}
	}
	return readRecord(s.rootPath())
}

func (s *Store) WriteRoot(record StateRecord) error {
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = time.Now().UTC()
	}
	return s.writeRecord(s.rootPath(), record)
}

func (s *Store) MarkStaleHeartbeats(now time.Time) (StateRecord, bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	entries, err := os.ReadDir(s.sessionDir())
	if err != nil {
		if os.IsNotExist(err) {
			return StateRecord{}, false, nil
		}
		return StateRecord{}, false, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		record, err := s.Read(sessionID)
		if err != nil {
			return StateRecord{}, false, err
		}
		if record.Lifecycle != LifecycleRunning || record.HeartbeatAt.IsZero() {
			continue
		}
		if now.Sub(record.HeartbeatAt) <= s.heartbeatTimeout {
			continue
		}
		record.Lifecycle = LifecycleBlocked
		record.BlockedReason = BlockedReasonStaleOwner
		record.UpdatedAt = now
		if err := s.writeSession(sessionID, record); err != nil {
			return StateRecord{}, false, err
		}
		return record, true, nil
	}
	return StateRecord{}, false, nil
}

func (s *Store) writeSession(sessionID string, record StateRecord) error {
	return s.writeRecord(s.sessionPath(sessionID), record)
}

func (s *Store) writeRecord(path string, record StateRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return s.atomicWrite(path, data)
}

func (s *Store) atomicWrite(path string, data []byte) error {
	lock := lockForPath(path)
	lock.Lock()
	defer lock.Unlock()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.tmp.%d.%d", path, os.Getpid(), time.Now().UnixNano())
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if s.failBeforeRename != nil {
		err := s.failBeforeRename
		_ = os.Remove(tmp)
		s.failBeforeRename = nil
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func readRecord(path string) (StateRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StateRecord{}, err
	}
	var record StateRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return StateRecord{}, err
	}
	return record, nil
}

func validateTransition(from, to Lifecycle) error {
	if to == "" {
		return fmt.Errorf("target lifecycle is required")
	}
	if isTerminal(from) {
		return fmt.Errorf("terminal lifecycle %q cannot transition to %q", from, to)
	}
	allowed := map[Lifecycle][]Lifecycle{
		LifecycleInactive: {LifecycleRunning},
		LifecycleRunning:  {LifecycleBlocked, LifecycleAskUser, LifecycleFinished, LifecycleFailed, LifecycleCancelled},
		LifecycleBlocked:  {LifecycleRunning, LifecycleFailed, LifecycleCancelled},
		LifecycleAskUser:  {LifecycleRunning, LifecycleFailed, LifecycleCancelled},
	}
	for _, candidate := range allowed[from] {
		if candidate == to {
			return nil
		}
	}
	return fmt.Errorf("illegal transition from %q to %q", from, to)
}

func isTerminal(lifecycle Lifecycle) bool {
	return lifecycle == LifecycleFinished || lifecycle == LifecycleFailed || lifecycle == LifecycleCancelled
}

func lockForPath(path string) *sync.Mutex {
	pathLocksMu.Lock()
	defer pathLocksMu.Unlock()
	lock := pathLocks[path]
	if lock == nil {
		lock = &sync.Mutex{}
		pathLocks[path] = lock
	}
	return lock
}

func (s *Store) stateDir() string {
	return filepath.Join(s.root, ".omg", "state")
}

func (s *Store) sessionDir() string {
	return filepath.Join(s.stateDir(), "sessions")
}

func (s *Store) sessionPath(sessionID string) string {
	return filepath.Join(s.sessionDir(), sessionID, "state.json")
}

func (s *Store) rootPath() string {
	return filepath.Join(s.stateDir(), "state.json")
}
