package sandbox

import (
	"strings"
	"testing"
)

// TestDeniedSubstring pins that denied commands are rejected even when they
// appear in the middle of a longer command line.
func TestDeniedSubstring(t *testing.T) {
	sb, err := New(Spec{Type: "process", DeniedCommands: []string{"rm -rf"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer sb.Close()
	res := sb.Run(t.Context(), Command{Command: "echo ok && rm -rf /tmp/x"})
	if res.Err == nil || !strings.Contains(res.Err.Error(), "denied") {
		t.Fatalf("denied command embedded in a longer command must be rejected by policy, got %+v", res)
	}
}