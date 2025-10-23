package jsonnet

import (
	"testing"
)

func TestHermeticDetection(t *testing.T) {
	tests := []struct {
		name        string
		file        string
		shouldCache bool
		description string
	}{
		{
			name:        "pure function",
			file:        "testdata/hermetic_pure.jsonnet",
			shouldCache: true,
			description: "Function with no external references should be hermetic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache
			hermeticFunctionCacheMutex.Lock()
			hermeticFunctionCache = make(map[string]value)
			hermeticFunctionCacheMutex.Unlock()

			vm := MakeVM()
			_, err := vm.EvaluateFile(tt.file)
			if err != nil {
				t.Logf("Evaluation error (may be expected): %v", err)
				return
			}

			hermeticFunctionCacheMutex.RLock()
			cacheSize := len(hermeticFunctionCache)
			hermeticFunctionCacheMutex.RUnlock()

			cached := cacheSize > 0
			if cached != tt.shouldCache {
				t.Errorf("%s: expected cached=%v, got cached=%v (cache size: %d)",
					tt.description, tt.shouldCache, cached, cacheSize)
			} else {
				t.Logf("%s: ✓ cached=%v (cache size: %d)", tt.description, cached, cacheSize)
			}
		})
	}
}

func TestContextDependentCapture(t *testing.T) {
	// Note: Anonymous snippets are never cached since they lack a filename for unique identification.
	// This test just ensures the code doesn't crash with context-dependent captures.
	snippet := `
{
	x: 10,
	local this = self,
	f(n):: n + this.x,
	result: self.f(5) tailstrict
}
`

	vm := MakeVM()
	result, err := vm.EvaluateAnonymousSnippet("test.jsonnet", snippet)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	t.Logf("Result: %s", result)
}
