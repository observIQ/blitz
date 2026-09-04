package datagen

import (
	"errors"
	"math/rand"
	"testing"
)

// These tests exercise the developer-error paths converted from panics to error
// returns (PIPE-1003). They double as regression guards: they assert that each
// constructor rejects a nil domain, and that GenerateEnvironment propagates a
// failure from any of its identity-generation stages.

func TestGenerateUserIdentity_NilDomain(t *testing.T) {
	u, err := GenerateUserIdentity(rand.New(rand.NewSource(1)), nil)
	if err == nil || u != nil {
		t.Errorf("GenerateUserIdentity(nil) = (%v, %v), want (nil, error)", u, err)
	}
}

func TestGenerateUsers_NilDomain(t *testing.T) {
	users, err := GenerateUsers(1, 1, nil)
	if err == nil || users != nil {
		t.Errorf("GenerateUsers(nil) = (%v, %v), want (nil, error)", users, err)
	}
}

func TestGenerateGroups_NilDomain(t *testing.T) {
	groups, err := GenerateGroups(1, 5, 0, nil, nil)
	if err == nil || groups != nil {
		t.Errorf("GenerateGroups(nil) = (%v, %v), want (nil, error)", groups, err)
	}
}

func TestGenerateSystems_NilDomainPropagates(t *testing.T) {
	systems, err := generateSystems(1, 1, 1, 3, nil, GenerateDefaultNetworks())
	if err == nil || systems != nil {
		t.Errorf("generateSystems(nil) = (%v, %v), want (nil, error)", systems, err)
	}
}

func TestGenerateEnvironment_PropagatesStageErrors(t *testing.T) {
	boom := errors.New("boom")
	seeds := &SeedConfig{Shared: 1}

	t.Run("users stage error", func(t *testing.T) {
		orig := genUsers
		genUsers = func(int64, int, *DomainIdentity) ([]*UserIdentity, error) { return nil, boom }
		defer func() { genUsers = orig }()
		if env, err := GenerateEnvironment(seeds, nil); !errors.Is(err, boom) || env != nil {
			t.Errorf("want boom + nil env, got (%v, %v)", env, err)
		}
	})

	t.Run("groups stage error", func(t *testing.T) {
		orig := genGroups
		genGroups = func(int64, int, int, *DomainIdentity, []*UserIdentity) ([]*GroupIdentity, error) { return nil, boom }
		defer func() { genGroups = orig }()
		if env, err := GenerateEnvironment(seeds, nil); !errors.Is(err, boom) || env != nil {
			t.Errorf("want boom + nil env, got (%v, %v)", env, err)
		}
	})

	t.Run("systems stage error", func(t *testing.T) {
		orig := genSystems
		genSystems = func(_, _, _ int64, _ int, _ *DomainIdentity, _ []*NetworkIdentity) ([]*SystemIdentity, error) {
			return nil, boom
		}
		defer func() { genSystems = orig }()
		if env, err := GenerateEnvironment(seeds, nil); !errors.Is(err, boom) || env != nil {
			t.Errorf("want boom + nil env, got (%v, %v)", env, err)
		}
	})
}
