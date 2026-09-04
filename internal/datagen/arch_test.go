package datagen

import "testing"

func TestParseArch(t *testing.T) {
	valid := map[string]Arch{
		"amd64":   ArchAMD64,
		"arm32":   ArchARM32,
		"arm64":   ArchARM64,
		"ia64":    ArchIA64,
		"ppc32":   ArchPPC32,
		"ppc64":   ArchPPC64,
		"s390x":   ArchS390X,
		"x86":     ArchX86,
		" AMD64 ": ArchAMD64, // trimmed + lowercased
	}
	for in, want := range valid {
		got, err := ParseArch(in)
		if err != nil {
			t.Errorf("ParseArch(%q): unexpected error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseArch(%q) = %q, want %q", in, got, want)
		}
	}

	for _, in := range []string{"", "sparc", "riscv64", "mips"} {
		if _, err := ParseArch(in); err == nil {
			t.Errorf("ParseArch(%q): want error, got nil", in)
		}
	}
}
