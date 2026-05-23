// Job and Step types, plus atomic JSON-file storage. The updater keeps a
// single rolling state document at $STATE_DIR/state.json containing the
// last N jobs (most-recent first). Writes use the write-temp-then-rename
// pattern so a crash mid-write cannot corrupt the state file.

package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State enumerates the steps of an update job.
type State string

const (
	StateIdle           State = "idle"
	StatePlanning       State = "planning"
	StateSnapshotting   State = "snapshotting"
	StatePulling        State = "pulling"
	StateVerifying      State = "verifying"
	StateMigrating      State = "migrating"
	StateSwapping       State = "swapping"
	StateHealthchecking State = "healthchecking"

	// Terminal success
	StateCommitted State = "committed"

	// Terminal failure paths
	StateRollingBack State = "rolling_back"
	StateRolledBack  State = "rolled_back"
	StateFailed      State = "failed"  // failure DURING rollback — manual intervention needed
	StateAborted     State = "aborted" // user-requested abort
)

// IsTerminal reports whether the state is a final state.
func (s State) IsTerminal() bool {
	switch s {
	case StateCommitted, StateRolledBack, StateFailed, StateAborted:
		return true
	}
	return false
}

// Step records one phase of a Job.
type Step struct {
	State       State      `json:"state"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Output      string     `json:"output,omitempty"`
	Error       string     `json:"error,omitempty"`
}

func (s *Step) Finish(output string, err error) {
	now := time.Now().UTC()
	s.CompletedAt = &now
	s.Output = output
	if err != nil {
		s.Error = err.Error()
	}
}

// Job is one end-to-end update attempt. Persisted to state.json.
type Job struct {
	ID            string     `json:"id"`
	State         State      `json:"state"`
	TargetTag     string     `json:"target_tag"`               // e.g. "v0.1.2"
	TargetVersion string     `json:"target_version,omitempty"` // e.g. "0.1.2"
	TargetImage   string     `json:"target_image,omitempty"`   // e.g. "ghcr.io/algomation-ai/processgit:0.1.2"
	TargetDigest  string     `json:"target_digest,omitempty"`  // e.g. "sha256:..."
	PreviousImage string     `json:"previous_image,omitempty"` // captured before swap for rollback
	StartedAt     time.Time  `json:"started_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
	Steps         []Step     `json:"steps"`
	Error         string     `json:"error,omitempty"`
}

// NewJob mints a job with a unique short ID.
func NewJob(targetTag string) *Job {
	now := time.Now().UTC()
	return &Job{
		ID:        newJobID(),
		State:     StateIdle,
		TargetTag: targetTag,
		StartedAt: now,
		UpdatedAt: now,
		Steps:     []Step{},
	}
}

func newJobID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Reading /dev/urandom shouldn't fail; if it does, fall back to time-based.
		return fmt.Sprintf("job_%d", time.Now().UnixNano())
	}
	return "job_" + hex.EncodeToString(b)
}

// transitionTo advances the job state and starts a Step for it. The previous
// Step (if any) must already have been finished by the caller.
func (j *Job) transitionTo(s State) *Step {
	j.State = s
	j.UpdatedAt = time.Now().UTC()
	if s.IsTerminal() {
		now := time.Now().UTC()
		j.CompletedAt = &now
	}
	step := Step{State: s, StartedAt: time.Now().UTC()}
	j.Steps = append(j.Steps, step)
	return &j.Steps[len(j.Steps)-1]
}

// fail marks the job as terminal-failed with the given error.
func (j *Job) fail(s State, err error) {
	j.State = s
	j.Error = err.Error()
	now := time.Now().UTC()
	j.UpdatedAt = now
	j.CompletedAt = &now
}

// Store is the on-disk job log. Always holds the *full* history bounded by
// maxJobs; new jobs are prepended to keep recent-first ordering.
type Store struct {
	mu      sync.Mutex
	path    string
	maxJobs int
	jobs    []*Job
	active  *Job // pointer into jobs[0] when there's a non-terminal job
}

const defaultMaxJobs = 50

// NewStore loads (or creates) the state file at path.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path:    path,
		maxJobs: defaultMaxJobs,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

type stateFile struct {
	SchemaVersion int    `json:"schema_version"`
	Jobs          []*Job `json:"jobs"`
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		s.jobs = nil
		return nil
	}
	if err != nil {
		return fmt.Errorf("read state: %w", err)
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse state: %w", err)
	}
	s.jobs = sf.Jobs
	for _, j := range s.jobs {
		if !j.State.IsTerminal() {
			s.active = j
			break
		}
	}
	return nil
}

func (s *Store) save() error {
	sf := stateFile{SchemaVersion: 1, Jobs: s.jobs}
	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write tmp state: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("atomic rename state: %w", err)
	}
	return nil
}

// AddJob prepends a job. Returns an error if there's already an active job.
func (s *Store) AddJob(j *Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil {
		return fmt.Errorf("update already in progress: job %s in state %s", s.active.ID, s.active.State)
	}
	s.jobs = append([]*Job{j}, s.jobs...)
	if len(s.jobs) > s.maxJobs {
		s.jobs = s.jobs[:s.maxJobs]
	}
	s.active = j
	return s.save()
}

// PersistActive saves changes to the currently-active job.
func (s *Store) PersistActive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active != nil && s.active.State.IsTerminal() {
		s.active = nil
	}
	return s.save()
}

// ClearActive marks no job as active. Called after the active job reaches a
// terminal state.
func (s *Store) ClearActive() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = nil
	return s.save()
}

// Active returns the currently-running job, or nil.
func (s *Store) Active() *Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active
}

// Get returns a snapshot copy of the job with the given ID.
func (s *Store) Get(id string) (*Job, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			cp := *j
			cp.Steps = append([]Step(nil), j.Steps...)
			return &cp, true
		}
	}
	return nil, false
}

// List returns snapshots of all jobs, recent-first.
func (s *Store) List() []*Job {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*Job, len(s.jobs))
	for i, j := range s.jobs {
		cp := *j
		cp.Steps = append([]Step(nil), j.Steps...)
		out[i] = &cp
	}
	return out
}
