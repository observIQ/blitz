package datagen

import "testing"

func TestParseCIDRs(t *testing.T) {
	nets, err := parseCIDRs("10.0.0.0/8", "2001:db8::/32")
	if err != nil {
		t.Fatalf("parseCIDRs(valid) error = %v, want nil", err)
	}
	if len(nets) != 2 {
		t.Errorf("parseCIDRs(valid) returned %d nets, want 2", len(nets))
	}
	if _, err := parseCIDRs("10.0.0.0/8", "garbage"); err == nil {
		t.Error("parseCIDRs with a bad entry should return an error")
	}
}

func TestMustParseCIDRs_PanicsOnBadLiteral(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("mustParseCIDRs should panic on a malformed literal")
		}
	}()
	mustParseCIDRs("not-a-cidr")
}
