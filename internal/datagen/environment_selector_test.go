package datagen

import "testing"

func TestEnvironmentSystemForKey(t *testing.T) {
	env := &Environment{Systems: []*SystemIdentity{
		{Hostname: "a"}, {Hostname: "b"}, {Hostname: "c"},
	}}

	// Non-empty environment resolves to an in-range system.
	first := env.SystemForKey("hostmetrics")
	if first == nil {
		t.Fatal("SystemForKey returned nil for a non-empty environment")
	}

	// Deterministic: the same key always maps to the same system, so a
	// generator resolves its host once and attributes every record the same way.
	for i := 0; i < 5; i++ {
		if env.SystemForKey("hostmetrics") != first {
			t.Fatal("SystemForKey is not deterministic for a repeated key")
		}
	}

	// Every key resolves to a real member of Systems (total, in-range mapping).
	members := map[*SystemIdentity]bool{}
	for _, s := range env.Systems {
		members[s] = true
	}
	for _, k := range []string{"apache", "nginx", "postgres", "wel", "traces", "json", "fix"} {
		s := env.SystemForKey(k)
		if s == nil {
			t.Fatalf("SystemForKey(%q) = nil", k)
		}
		if !members[s] {
			t.Errorf("SystemForKey(%q) returned a system not in Systems", k)
		}
	}

	// Empty environment resolves to nil rather than panicking.
	if (&Environment{}).SystemForKey("x") != nil {
		t.Error("SystemForKey on an empty environment should return nil")
	}
}

func TestEnvironmentSystemForRequirement(t *testing.T) {
	env := &Environment{Systems: []*SystemIdentity{
		{Hostname: "lin-a", OSInfo: OSInfo{Type: OSLinux}},
		{Hostname: "win-a", OSInfo: OSInfo{Type: OSWindows}},
		{Hostname: "lin-b", OSInfo: OSInfo{Type: OSLinux}},
		{Hostname: "win-b", OSInfo: OSInfo{Type: OSWindows}},
	}}

	for _, key := range []string{"wel", "hostmetrics", "kubernetes", "json", "x", "y", "z"} {
		got := env.SystemFor(key, 0, HostRequirement{OS: OSWindows})
		if got == nil || got.OSInfo.Type != OSWindows {
			t.Fatalf("SystemFor(%q, windows) = %+v, want a windows system", key, got)
		}
		if env.SystemFor(key, 0, HostRequirement{OS: OSWindows}) != got {
			t.Fatalf("SystemFor(%q, windows) is not deterministic", key)
		}
	}
	if got := env.SystemFor("wel", 0, HostRequirement{OS: OSMacOS}); got != nil {
		t.Errorf("SystemFor with no eligible system = %+v, want nil", got)
	}
	if env.SystemFor("k", 0, HostRequirement{}) != env.SystemForKey("k") {
		t.Error("zero requirement must match SystemForKey")
	}
}
