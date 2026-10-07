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

func TestEnvironmentSystemForKeyWithOS(t *testing.T) {
	env := &Environment{Systems: []*SystemIdentity{
		{Hostname: "win-1", OSInfo: OSInfo{Type: OSWindows}},
		{Hostname: "lin-1", OSInfo: OSInfo{Type: OSLinux}},
		{Hostname: "win-2", OSInfo: OSInfo{Type: OSWindows}},
		{Hostname: "lin-2", OSInfo: OSInfo{Type: OSLinux}},
		nil,
	}}

	// Only systems running the requested OS are eligible, for every key.
	for _, k := range []string{"hostmetrics", "apache", "nginx", "postgres", "wel", "traces"} {
		if s := env.SystemForKeyWithOS(k, OSLinux); s == nil || s.OSInfo.Type != OSLinux {
			t.Errorf("SystemForKeyWithOS(%q, linux) = %+v, want a linux system", k, s)
		}
		if s := env.SystemForKeyWithOS(k, OSWindows); s == nil || s.OSInfo.Type != OSWindows {
			t.Errorf("SystemForKeyWithOS(%q, windows) = %+v, want a windows system", k, s)
		}
	}

	// Deterministic for a repeated key.
	first := env.SystemForKeyWithOS("hostmetrics", OSLinux)
	for i := 0; i < 5; i++ {
		if env.SystemForKeyWithOS("hostmetrics", OSLinux) != first {
			t.Fatal("SystemForKeyWithOS is not deterministic for a repeated key")
		}
	}

	// No system runs the OS: nil, not a system of another OS.
	if s := env.SystemForKeyWithOS("hostmetrics", OSMacOS); s != nil {
		t.Errorf("SystemForKeyWithOS(macos) = %+v, want nil", s)
	}
}
