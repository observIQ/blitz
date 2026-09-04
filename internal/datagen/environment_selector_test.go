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
