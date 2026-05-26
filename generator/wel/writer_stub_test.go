//go:build !windows

package wel

import (
	"strings"
	"testing"
)

func TestNewEventWriterStub(t *testing.T) {
	writer, err := NewEventWriter(false)
	if writer != nil {
		t.Error("expected nil writer on non-Windows")
	}
	if err == nil {
		t.Fatal("expected error on non-Windows")
	}
	if !strings.Contains(err.Error(), "requires Windows") {
		t.Errorf("error should mention Windows requirement, got: %s", err.Error())
	}
}
