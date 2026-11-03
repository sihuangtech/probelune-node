package checks

import (
	ct "github.com/sihuangtech/probelune-contracts"
	"testing"
)

func TestAllContractChecksHaveExactlyOneExecutor(t *testing.T) {
	seen := map[string]bool{}
	for _, entry := range registry {
		if seen[entry.kind] {
			t.Fatalf("duplicate executor %s", entry.kind)
		}
		seen[entry.kind] = true
	}
	for _, definition := range ct.CheckDefinitions() {
		if run, ok := executorFor(definition.Type); !ok || run == nil {
			t.Fatalf("missing executor %s", definition.Type)
		}
		delete(seen, definition.Type)
	}
	if len(seen) != 0 {
		t.Fatalf("unrecognized executors: %v", seen)
	}
}
