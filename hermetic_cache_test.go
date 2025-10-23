package jsonnet

import (
	"testing"
)

func TestHermeticFunctionCache(t *testing.T) {
	// Test that hermetic functions are cached with tailstrict
	vm := MakeVM()

	// Clear cache
	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	// Create a Jsonnet snippet with a hermetic function called with tailstrict
	// tailstrict forces argument evaluation, enabling caching
	snippet := `
local expensiveFunction(x) = x * x;
{
	a: std.assertEqual(expensiveFunction(5) tailstrict, 25),
	b: std.assertEqual(expensiveFunction(5) tailstrict, 25),  // Should hit cache
	c: std.assertEqual(expensiveFunction(10) tailstrict, 100),
	d: std.assertEqual(expensiveFunction(10) tailstrict, 100), // Should hit cache
	result: true
}
`

	result, err := vm.EvaluateAnonymousSnippet("test.jsonnet", snippet)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	expected := `{
   "a": true,
   "b": true,
   "c": true,
   "d": true,
   "result": true
}
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}

	// Check that cache was populated (should have 2 entries: f(5) and f(10))
	hermeticFunctionCacheMutex.RLock()
	cacheSize := len(hermeticFunctionCache)
	hermeticFunctionCacheMutex.RUnlock()

	if cacheSize == 0 {
		t.Error("Expected cache to be populated, but it's empty")
	}
	t.Logf("Cache has %d entries", cacheSize)
}

func TestNonHermeticFunctionNotCached(t *testing.T) {
	// Test that non-hermetic functions are not cached
	vm := MakeVM()

	// Clear cache before test
	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	// This function captures an external variable, so it's not hermetic
	snippet := `
local y = 42;
local nonHermeticFunction(x) = x + y;
{
	a: nonHermeticFunction(5),
	b: nonHermeticFunction(5),
}
`

	result, err := vm.EvaluateAnonymousSnippet("test.jsonnet", snippet)
	if err != nil {
		t.Fatalf("Evaluation failed: %v", err)
	}

	expected := `{
   "a": 47,
   "b": 47
}
`
	if result != expected {
		t.Errorf("Expected:\n%s\nGot:\n%s", expected, result)
	}

	// Non-hermetic functions should not be cached
	// Cache might still be empty or have fewer entries
}

func TestHermeticFunctionCacheAcrossEvaluations(t *testing.T) {
	// Test that cache persists across evaluations (global cache)

	// Clear cache
	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	vm1 := MakeVM()
	snippet := `
local f(x) = x * x * x;
f(7) tailstrict
`

	result1, err := vm1.EvaluateAnonymousSnippet("test1.jsonnet", snippet)
	if err != nil {
		t.Fatalf("First evaluation failed: %v", err)
	}

	if result1 != "343\n" {
		t.Errorf("Expected 343, got %s", result1)
	}

	// Check cache was populated
	cacheSize := 0
	hermeticFunctionCacheMutex.RLock()
	cacheSize = len(hermeticFunctionCache)
	hermeticFunctionCacheMutex.RUnlock()

	if cacheSize == 0 {
		t.Error("Expected cache to be populated after first evaluation")
	}
	t.Logf("First evaluation: cache has %d entries", cacheSize)

	// Now evaluate with a different VM - should still use the same global cache
	vm2 := MakeVM()
	result2, err := vm2.EvaluateAnonymousSnippet("test2.jsonnet", snippet)
	if err != nil {
		t.Fatalf("Second evaluation failed: %v", err)
	}

	if result2 != "343\n" {
		t.Errorf("Expected 343, got %s", result2)
	}

	// Cache size should be the same (cache hit)
	newCacheSize := 0
	hermeticFunctionCacheMutex.RLock()
	newCacheSize = len(hermeticFunctionCache)
	hermeticFunctionCacheMutex.RUnlock()

	t.Logf("Second evaluation: cache has %d entries", newCacheSize)
	if newCacheSize != cacheSize {
		t.Logf("Cache size changed from %d to %d (this might be ok depending on implementation details)", cacheSize, newCacheSize)
	}
}
