# Hermetic Function Cache Implementation

## Overview

Implemented a global cache for hermetic functions in go-jsonnet. Hermetic functions (functions with no external references) are automatically cached, providing significant performance improvements when the same function is called multiple times with the same arguments.

## Implementation Details

### What is a Hermetic Function?

A function is considered hermetic if it has no external references that depend on context. Specifically:

**Non-hermetic references:**
1. `$` - global/root context reference
2. `self` - object context reference  
3. `super` - parent object reference
4. Captured variables from enclosing scope (in `upValues`)

**Hermetic:**
- Functions with only parameters and local variables
- Pure computations with no external dependencies

### Cache Behavior

- **Global and Thread-Safe**: Two global caches, both protected by RWMutex:
  1. **Function result cache**: Maps (function location + arguments) → result value
  2. **AST analysis cache**: Maps function location → has-external-refs boolean
- **Automatic**: No API changes needed - caching happens transparently
- **Lazy-Evaluation Preserving**: Only caches when arguments are already evaluated (e.g., with `tailstrict`) to preserve Jsonnet's lazy evaluation semantics
- **Key Generation**: Cache keys are generated from function location (file:line:column) + SHA256 hash of serialized argument values
- **AST Analysis Cache**: The hermetic detection (checking for `$`, `self`, `super`) is cached per function location, avoiding repeated AST traversals

### Performance Impact

Benchmark results show ~21x speedup for cache hits:
- Without cache: 798,824 ns/op
- With cache hit: 37,611 ns/op

### Code Changes

Modified files:
- `thunks.go`: Added cache infrastructure, hermetic detection, and caching logic in `evalCall`

New files:
- `hermetic_cache_test.go`: Tests demonstrating cache functionality
- `hermetic_cache_bench_test.go`: Benchmarks showing performance improvements
- `examples/hermetic_cache_demo.jsonnet`: Example demonstrating the feature

### Example Usage

```jsonnet
local expensiveFunction(x) = x * x;

{
  a: expensiveFunction(5) tailstrict,  // Cache miss - computed
  b: expensiveFunction(5) tailstrict,  // Cache HIT - instant!
  c: expensiveFunction(5) tailstrict,  // Cache HIT - instant!
}
```

### Limitations

- Only works when arguments are already evaluated (e.g., with `tailstrict`)
- Cache never expires or gets cleared (simple global map implementation)
- Non-hermetic functions are never cached:
  - Functions with captured variables from enclosing scope
  - Functions referencing `$`, `self`, or `super`
  - Functions in object methods (have self binding)
- Conservative: functions capturing external constant locals are also not cached (could be improved in future)

### Testing

All existing tests pass. Added specific tests:
- `TestHermeticFunctionCache`: Verifies cache works for hermetic functions with tailstrict
- `TestNonHermeticFunctionNotCached`: Ensures non-hermetic functions aren't cached
- `TestHermeticFunctionCacheAcrossEvaluations`: Confirms cache is shared globally across VM instances
- `TestHermeticDetection`: Comprehensive tests for detecting hermetic vs non-hermetic functions:
  - Pure functions (no external refs) ✓ cached
  - Functions with `$` reference ✗ not cached
  - Functions with `self` reference ✗ not cached
  - Functions with `super` reference ✗ not cached
  - Functions with external constants ✗ not cached (conservative)
- `TestContextDependentCapture`: Tests `local this = self;` pattern correctly not cached

Run tests: `go test ./...`
Run benchmarks: `go test -bench=BenchmarkHermeticFunction`

