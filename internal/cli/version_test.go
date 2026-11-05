package cli

import (
	"bytes"
	"testing"
)

func TestRunPrintsVersionWithoutOtherFlags(t *testing.T) {
	old := Version
	Version = "v9.9.9"
	t.Cleanup(func() { Version = old })

	var out bytes.Buffer
	if err := Run([]string{"--version"}, &out); err != nil {
		t.Fatalf("--version needs no other flags: %v", err)
	}
	if got := out.String(); got != "helmcov v9.9.9\n" {
		t.Errorf("output = %q", got)
	}
}
