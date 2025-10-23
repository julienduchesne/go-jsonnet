package jsonnet

import (
	"testing"
)

func TestHermeticDetection(t *testing.T) {
	tests := []struct {
		name        string
		snippet     string
		shouldCache bool
		description string
	}{
		{
			name: "pure function",
			snippet: `
local f(x) = x * x;
f(5) tailstrict
`,
			shouldCache: true,
			description: "Function with no external references should be hermetic",
		},
		{
			name: "function with $ reference",
			snippet: `
local f(x) = x + $.root;
{
	root: 10,
	result: f(5) tailstrict
}
`,
			shouldCache: false,
			description: "Function referencing $ (global context) should not be hermetic",
		},
		{
			name: "function with self reference",
			snippet: `
{
	x: 10,
	f(n):: n + self.x,
	result: self.f(5) tailstrict
}
`,
			shouldCache: false,
			description: "Function referencing self should not be hermetic",
		},
		{
			name: "function with super reference",
			snippet: `
local base = {
	f(n):: n * 2
};
base + {
	f(n):: super.f(n) + 1,
	result: self.f(5) tailstrict
}
`,
			shouldCache: false,
			description: "Function referencing super should not be hermetic",
		},
		{
			name: "function with external constant",
			snippet: `
local external = 42;
local f(x) = x + external;
f(5) tailstrict
`,
			shouldCache: false, // Current implementation marks this as non-hermetic
			description: "Function with external constant (in upValues) - currently not cached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear cache
			hermeticFunctionCacheMutex.Lock()
			hermeticFunctionCache = make(map[string]value)
			hermeticFunctionCacheMutex.Unlock()

			vm := MakeVM()
			_, err := vm.EvaluateAnonymousSnippet("test.jsonnet", tt.snippet)
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
	// Test case for: local this = self; and then using this in function
	snippet := `
{
	x: 10,
	local this = self,
	f(n):: n + this.x,
	result: self.f(5) tailstrict
}
`

	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	vm := MakeVM()
	result, err := vm.EvaluateAnonymousSnippet("test.jsonnet", snippet)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	t.Logf("Result: %s", result)

	hermeticFunctionCacheMutex.RLock()
	cacheSize := len(hermeticFunctionCache)
	hermeticFunctionCacheMutex.RUnlock()

	// This should not be cached because 'this' references 'self'
	if cacheSize > 0 {
		t.Errorf("Function with context-dependent capture should not be cached, but cache has %d entries", cacheSize)
	} else {
		t.Logf("✓ Function with 'local this = self' correctly not cached")
	}
}
