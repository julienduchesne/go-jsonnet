package jsonnet

import (
	"testing"
)

func BenchmarkHermeticFunctionWithCache(b *testing.B) {
	// Benchmark hermetic function calls with caching enabled

	// Clear cache before benchmark
	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	vm := MakeVM()

	// This function is expensive and hermetic
	// We call it with tailstrict to enable caching
	snippet := `
local fib(n) = if n <= 1 then n else fib(n-1) + fib(n-2);
[
	fib(10) tailstrict,
	fib(10) tailstrict,
	fib(10) tailstrict,
	fib(10) tailstrict,
	fib(10) tailstrict,
]
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache between iterations to measure fresh performance
		hermeticFunctionCacheMutex.Lock()
		hermeticFunctionCache = make(map[string]value)
		hermeticFunctionCacheMutex.Unlock()

		_, err := vm.EvaluateAnonymousSnippet("bench.jsonnet", snippet)
		if err != nil {
			b.Fatalf("Evaluation failed: %v", err)
		}
	}
}

func BenchmarkHermeticFunctionCacheHit(b *testing.B) {
	// Benchmark cache hits for hermetic functions

	// Pre-populate cache
	hermeticFunctionCacheMutex.Lock()
	hermeticFunctionCache = make(map[string]value)
	hermeticFunctionCacheMutex.Unlock()

	vm := MakeVM()

	snippet := `
local square(x) = x * x;
square(42) tailstrict
`

	// Prime the cache
	_, err := vm.EvaluateAnonymousSnippet("prime.jsonnet", snippet)
	if err != nil {
		b.Fatalf("Prime failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := vm.EvaluateAnonymousSnippet("bench.jsonnet", snippet)
		if err != nil {
			b.Fatalf("Evaluation failed: %v", err)
		}
	}
}
