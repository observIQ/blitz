package resource

import "testing"

func TestWithHost(t *testing.T) {
	r := WithHost("web-01", "apache", "apache.format", "common")
	if r["host.name"] != "web-01" {
		t.Errorf(`host.name = %q, want "web-01"`, r["host.name"])
	}
	if r["telemetry.source"] != "apache" {
		t.Errorf(`telemetry.source = %q, want "apache"`, r["telemetry.source"])
	}
	if r["apache.format"] != "common" {
		t.Errorf(`apache.format = %q, want "common"`, r["apache.format"])
	}
}

func TestWithHost_OddExtrasIgnoresDangling(t *testing.T) {
	r := WithHost("h", "src", "onlykey")
	if _, ok := r["onlykey"]; ok {
		t.Error("dangling extra key should be ignored")
	}
	if r["host.name"] != "h" {
		t.Errorf("host.name = %q, want h", r["host.name"])
	}
}

func TestDefaultUsesProcessHostname(t *testing.T) {
	r := Default("apache")
	if r["host.name"] != Hostname() {
		t.Errorf("Default host.name = %q, want process hostname %q", r["host.name"], Hostname())
	}
	if r["telemetry.source"] != "apache" {
		t.Errorf("telemetry.source = %q, want apache", r["telemetry.source"])
	}
}
