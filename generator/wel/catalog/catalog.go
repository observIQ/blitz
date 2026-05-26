// Package catalog provides the event definition registry for the Windows Event Log generator.
// It defines event types, machine roles, and channel-based selection logic used to generate
// realistic Windows Event Log entries across all supported channels and event IDs.
package catalog

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
)

// MachineRole represents the role of the simulated machine, which controls
// which event definitions are eligible for generation.
type MachineRole string

const (
	// RoleWorkstation is a standard desktop. All events with MinRole=RoleWorkstation are eligible.
	RoleWorkstation MachineRole = "workstation"
	// RoleMember is a member server. Events with MinRole <= RoleMember are eligible.
	RoleMember MachineRole = "member"
	// RoleDC is a Domain Controller. All events are eligible.
	RoleDC MachineRole = "dc"
)

// roleLevel maps each role to a numeric level for comparison.
// Higher level includes all lower levels.
var roleLevel = map[MachineRole]int{
	RoleWorkstation: 0,
	RoleMember:      1,
	RoleDC:          2,
}

// Includes returns true if the machine role includes the given minimum role.
// A DC includes all roles, a member includes member and workstation, etc.
func (r MachineRole) Includes(minRole MachineRole) bool {
	return roleLevel[r] >= roleLevel[minRole]
}

// EventLevel represents the severity level of a Windows Event.
type EventLevel int

const (
	LevelLogAlways   EventLevel = 0 // Used by Security audit events
	LevelCritical    EventLevel = 1
	LevelError       EventLevel = 2
	LevelWarning     EventLevel = 3
	LevelInformation EventLevel = 4
	LevelVerbose     EventLevel = 5
)

// String returns the human-readable name for the event level.
func (l EventLevel) String() string {
	switch l {
	case LevelLogAlways:
		return "LogAlways"
	case LevelCritical:
		return "Critical"
	case LevelError:
		return "Error"
	case LevelWarning:
		return "Warning"
	case LevelInformation:
		return "Information"
	case LevelVerbose:
		return "Verbose"
	default:
		return fmt.Sprintf("Unknown(%d)", l)
	}
}

// EventDataField is an ordered key-value pair in an event's EventData section.
type EventDataField struct {
	Name  string
	Value string
}

// GenerateOpts carries contextual data for event field generation.
type GenerateOpts struct {
	Computer   string
	DomainName string
	Role       MachineRole
	Usernames  []string
	IPs        []string
	Hostnames  []string
	State      *StateTracker
}

// EventDefinition defines how to generate a specific Windows Event type.
type EventDefinition struct {
	Channel      string
	EventID      int
	Version      int
	Level        EventLevel
	Provider     string
	ProviderGUID string
	Task         int
	TaskName     string
	Opcode       int
	OpcodeName   string
	Keywords     uint64
	KeywordNames []string

	// MinRole is the minimum machine role required for this event to be eligible.
	MinRole MachineRole

	// Generate produces randomized EventData fields and a rendered message.
	Generate func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string)
}

// globalDefinitions holds all registered event definitions.
// Populated by init() functions in channel-specific files.
var globalDefinitions []*EventDefinition

// Register adds an event definition to the global catalog.
// Called from init() functions in channel-specific files.
func Register(def EventDefinition) {
	globalDefinitions = append(globalDefinitions, &def)
}

// Registry maps channel names to their event definitions, filtered by role.
type Registry struct {
	channels map[string][]*EventDefinition
	// flattened is a pre-computed slice of all definitions for random selection.
	flattened []*EventDefinition
}

// DefaultRegistry returns a new registry populated with all registered event
// definitions that are eligible for the given machine role.
func DefaultRegistry(role MachineRole) *Registry {
	r := &Registry{
		channels: make(map[string][]*EventDefinition),
	}
	for _, def := range globalDefinitions {
		if role.Includes(def.MinRole) {
			r.channels[def.Channel] = append(r.channels[def.Channel], def)
		}
	}
	r.buildFlattened()
	return r
}

// Channels returns the sorted list of channel names in the registry.
func (r *Registry) Channels() []string {
	names := make([]string, 0, len(r.channels))
	for ch := range r.channels {
		names = append(names, ch)
	}
	sort.Strings(names)
	return names
}

// EventsForChannel returns the event definitions registered for a channel.
// Returns nil if the channel is not in the registry.
func (r *Registry) EventsForChannel(channel string) []*EventDefinition {
	return r.channels[channel]
}

// FilterChannels returns a new registry containing only the specified channels.
// If channels is nil or empty, the original registry is returned unchanged.
func (r *Registry) FilterChannels(channels []string) *Registry {
	if len(channels) == 0 {
		return r
	}
	wanted := make(map[string]bool, len(channels))
	for _, ch := range channels {
		wanted[ch] = true
	}
	filtered := &Registry{
		channels: make(map[string][]*EventDefinition),
	}
	for ch, defs := range r.channels {
		if wanted[ch] {
			filtered.channels[ch] = defs
		}
	}
	filtered.buildFlattened()
	return filtered
}

// RandomEvent returns a uniformly random event definition from the registry.
// Returns nil if the registry is empty.
func (r *Registry) RandomEvent(rng *rand.Rand) *EventDefinition {
	if len(r.flattened) == 0 {
		return nil
	}
	return r.flattened[rng.Intn(len(r.flattened))] // #nosec G404
}

// buildFlattened pre-computes the flat slice of all definitions for random selection.
func (r *Registry) buildFlattened() {
	r.flattened = nil
	for _, defs := range r.channels {
		r.flattened = append(r.flattened, defs...)
	}
}

// LogonSession represents an active logon session tracked by the StateTracker.
type LogonSession struct {
	LogonID    string
	Username   string
	DomainName string
}

// TrackedProcess represents a running process tracked by the StateTracker.
type TrackedProcess struct {
	ProcessID   string
	ProcessName string
	Username    string
}

// StateTracker maintains simulated machine state for correlated events.
// For example, logon events (4624) create sessions, and logoff events (4634)
// pick from active sessions. Thread-safe with bounded memory.
type StateTracker struct {
	mu         sync.Mutex
	rng        *rand.Rand
	maxEntries int

	logonSessions map[string]*LogonSession
	processes     map[string]*TrackedProcess
}

// NewStateTracker creates a new StateTracker with the given max entries per type.
func NewStateTracker(maxEntries int) *StateTracker {
	return &StateTracker{
		rng:           rand.New(rand.NewSource(0)), // #nosec G404
		maxEntries:    maxEntries,
		logonSessions: make(map[string]*LogonSession),
		processes:     make(map[string]*TrackedProcess),
	}
}

// AddLogonSession records a new active logon session.
// If the tracker is at capacity, an existing session is evicted.
func (s *StateTracker) AddLogonSession(logonID, username, domain string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.logonSessions) >= s.maxEntries {
		s.evictOneLogon()
	}
	s.logonSessions[logonID] = &LogonSession{
		LogonID:    logonID,
		Username:   username,
		DomainName: domain,
	}
}

// PickLogonSession returns a random active logon session without removing it.
// Returns false if no sessions are active.
func (s *StateTracker) PickLogonSession() (LogonSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.logonSessions) == 0 {
		return LogonSession{}, false
	}

	idx := s.rng.Intn(len(s.logonSessions)) // #nosec G404
	i := 0
	for _, session := range s.logonSessions {
		if i == idx {
			return *session, true
		}
		i++
	}
	return LogonSession{}, false
}

// RemoveLogonSession removes a logon session by ID.
func (s *StateTracker) RemoveLogonSession(logonID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.logonSessions, logonID)
}

// LogonSessionCount returns the number of active logon sessions.
func (s *StateTracker) LogonSessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.logonSessions)
}

// AddProcess records a new running process.
// If the tracker is at capacity, an existing process is evicted.
func (s *StateTracker) AddProcess(processID, processName, username string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.processes) >= s.maxEntries {
		s.evictOneProcess()
	}
	s.processes[processID] = &TrackedProcess{
		ProcessID:   processID,
		ProcessName: processName,
		Username:    username,
	}
}

// PickProcess returns a random running process without removing it.
// Returns false if no processes are tracked.
func (s *StateTracker) PickProcess() (TrackedProcess, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.processes) == 0 {
		return TrackedProcess{}, false
	}

	idx := s.rng.Intn(len(s.processes)) // #nosec G404
	i := 0
	for _, proc := range s.processes {
		if i == idx {
			return *proc, true
		}
		i++
	}
	return TrackedProcess{}, false
}

// RemoveProcess removes a tracked process by ID.
func (s *StateTracker) RemoveProcess(processID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.processes, processID)
}

// evictOneLogon removes one logon session to make room. Must hold mu.
func (s *StateTracker) evictOneLogon() {
	for id := range s.logonSessions {
		delete(s.logonSessions, id)
		return
	}
}

// evictOneProcess removes one process to make room. Must hold mu.
func (s *StateTracker) evictOneProcess() {
	for id := range s.processes {
		delete(s.processes, id)
		return
	}
}
