package catalog

import (
	"math/rand"
	"testing"
)

func TestMachineRoleIncludes(t *testing.T) {
	tests := []struct {
		name     string
		role     MachineRole
		minRole  MachineRole
		expected bool
	}{
		{"dc includes dc", RoleDC, RoleDC, true},
		{"dc includes member", RoleDC, RoleMember, true},
		{"dc includes workstation", RoleDC, RoleWorkstation, true},
		{"member includes member", RoleMember, RoleMember, true},
		{"member includes workstation", RoleMember, RoleWorkstation, true},
		{"member excludes dc", RoleMember, RoleDC, false},
		{"workstation includes workstation", RoleWorkstation, RoleWorkstation, true},
		{"workstation excludes member", RoleWorkstation, RoleMember, false},
		{"workstation excludes dc", RoleWorkstation, RoleDC, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.role.Includes(tt.minRole); got != tt.expected {
				t.Errorf("MachineRole(%q).Includes(%q) = %v, want %v", tt.role, tt.minRole, got, tt.expected)
			}
		})
	}
}

func TestEventLevelString(t *testing.T) {
	tests := []struct {
		level    EventLevel
		expected string
	}{
		{LevelLogAlways, "LogAlways"},
		{LevelCritical, "Critical"},
		{LevelError, "Error"},
		{LevelWarning, "Warning"},
		{LevelInformation, "Information"},
		{LevelVerbose, "Verbose"},
		{EventLevel(99), "Unknown(99)"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("EventLevel(%d).String() = %q, want %q", tt.level, got, tt.expected)
			}
		})
	}
}

func TestRegisterAndDefaultRegistry(t *testing.T) {
	// Save and restore global state
	old := globalDefinitions
	globalDefinitions = nil
	defer func() { globalDefinitions = old }()

	Register(EventDefinition{
		Channel:  "Security",
		EventID:  4624,
		MinRole:  RoleWorkstation,
		Provider: "Microsoft-Windows-Security-Auditing",
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			return nil, "test"
		},
	})
	Register(EventDefinition{
		Channel:  "Security",
		EventID:  5136,
		MinRole:  RoleDC,
		Provider: "Microsoft-Windows-Security-Auditing",
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			return nil, "dc only"
		},
	})
	Register(EventDefinition{
		Channel:  "System",
		EventID:  7045,
		MinRole:  RoleMember,
		Provider: "Service Control Manager",
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) {
			return nil, "member"
		},
	})

	t.Run("workstation registry", func(t *testing.T) {
		reg := DefaultRegistry(RoleWorkstation)
		channels := reg.Channels()
		if len(channels) != 1 {
			t.Fatalf("expected 1 channel for workstation, got %d: %v", len(channels), channels)
		}
		if channels[0] != "Security" {
			t.Errorf("expected channel Security, got %s", channels[0])
		}
		events := reg.EventsForChannel("Security")
		if len(events) != 1 {
			t.Fatalf("expected 1 Security event for workstation, got %d", len(events))
		}
		if events[0].EventID != 4624 {
			t.Errorf("expected EventID 4624, got %d", events[0].EventID)
		}
	})

	t.Run("member registry", func(t *testing.T) {
		reg := DefaultRegistry(RoleMember)
		channels := reg.Channels()
		if len(channels) != 2 {
			t.Fatalf("expected 2 channels for member, got %d: %v", len(channels), channels)
		}
		secEvents := reg.EventsForChannel("Security")
		if len(secEvents) != 1 {
			t.Fatalf("expected 1 Security event for member, got %d", len(secEvents))
		}
		sysEvents := reg.EventsForChannel("System")
		if len(sysEvents) != 1 {
			t.Fatalf("expected 1 System event for member, got %d", len(sysEvents))
		}
	})

	t.Run("dc registry", func(t *testing.T) {
		reg := DefaultRegistry(RoleDC)
		secEvents := reg.EventsForChannel("Security")
		if len(secEvents) != 2 {
			t.Fatalf("expected 2 Security events for dc, got %d", len(secEvents))
		}
	})
}

func TestRegistryChannelFilter(t *testing.T) {
	old := globalDefinitions
	globalDefinitions = nil
	defer func() { globalDefinitions = old }()

	Register(EventDefinition{
		Channel: "Security", EventID: 4624, MinRole: RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) { return nil, "" },
	})
	Register(EventDefinition{
		Channel: "System", EventID: 7045, MinRole: RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) { return nil, "" },
	})
	Register(EventDefinition{
		Channel: "Application", EventID: 1000, MinRole: RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) { return nil, "" },
	})

	reg := DefaultRegistry(RoleWorkstation)

	t.Run("filter to specific channels", func(t *testing.T) {
		filtered := reg.FilterChannels([]string{"Security", "System"})
		channels := filtered.Channels()
		if len(channels) != 2 {
			t.Fatalf("expected 2 channels, got %d: %v", len(channels), channels)
		}
		appEvents := filtered.EventsForChannel("Application")
		if len(appEvents) != 0 {
			t.Errorf("expected no Application events, got %d", len(appEvents))
		}
	})

	t.Run("empty filter returns all", func(t *testing.T) {
		filtered := reg.FilterChannels(nil)
		if len(filtered.Channels()) != 3 {
			t.Errorf("expected 3 channels with nil filter, got %d", len(filtered.Channels()))
		}
	})
}

func TestRegistryRandomEvent(t *testing.T) {
	old := globalDefinitions
	globalDefinitions = nil
	defer func() { globalDefinitions = old }()

	Register(EventDefinition{
		Channel: "Security", EventID: 4624, MinRole: RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) { return nil, "" },
	})
	Register(EventDefinition{
		Channel: "System", EventID: 7045, MinRole: RoleWorkstation,
		Generate: func(r *rand.Rand, opts *GenerateOpts) ([]EventDataField, string) { return nil, "" },
	})

	reg := DefaultRegistry(RoleWorkstation)
	rng := rand.New(rand.NewSource(42))

	// Draw many samples, both event IDs should appear
	seen := make(map[int]bool)
	for i := 0; i < 100; i++ {
		def := reg.RandomEvent(rng)
		if def == nil {
			t.Fatal("RandomEvent returned nil")
		}
		seen[def.EventID] = true
	}
	if !seen[4624] {
		t.Error("never saw EventID 4624 in 100 draws")
	}
	if !seen[7045] {
		t.Error("never saw EventID 7045 in 100 draws")
	}
}

func TestRegistryRandomEventEmpty(t *testing.T) {
	reg := &Registry{
		channels: make(map[string][]*EventDefinition),
	}
	rng := rand.New(rand.NewSource(42))
	if def := reg.RandomEvent(rng); def != nil {
		t.Errorf("expected nil from empty registry, got EventID %d", def.EventID)
	}
}

func TestEventsForChannelMissing(t *testing.T) {
	reg := &Registry{
		channels: make(map[string][]*EventDefinition),
	}
	events := reg.EventsForChannel("nonexistent")
	if len(events) != 0 {
		t.Errorf("expected 0 events for missing channel, got %d", len(events))
	}
}
