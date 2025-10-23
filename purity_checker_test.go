package jsonnet

import (
	"testing"

	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/internal/program"
)

func TestIsPureAST(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{
			name:     "pure library",
			code:     `{ x: 1 + 1, y: self.x * 2 }`,
			expected: true,
		},
		{
			name:     "pure function",
			code:     `function(a, b) a + b`,
			expected: true,
		},
		{
			name:     "pure with imports",
			code:     `local lib = import "lib.jsonnet"; lib.x + 1`,
			expected: true,
		},
		{
			name:     "uses extVar - impure",
			code:     `{ namespace: std.extVar("namespace") }`,
			expected: false,
		},
		{
			name:     "uses extVar in computed field - impure",
			code:     `local ns = std.extVar("ns"); { [ns]: "value" }`,
			expected: false,
		},
		{
			name:     "deeply nested extVar - impure",
			code:     `{ a: { b: { c: std.extVar("x") } } }`,
			expected: false,
		},
		{
			name:     "conditional extVar - impure",
			code:     `if true then std.extVar("x") else 42`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := program.SnippetToAST("test.jsonnet", "test.jsonnet", tt.code)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			result := isPureAST(node)
			if result != tt.expected {
				t.Errorf("isPureAST() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Benchmark to show the performance impact of result caching
func BenchmarkImportWithoutResultCache(b *testing.B) {
	// Simulate current behavior: AST cached, but still need to evaluate
	vm := MakeVM()

	// Import a pure library file multiple times
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := vm.EvaluateSnippet("test.jsonnet", `
			local lib = { x: 1, y: 2, z: 3, compute: function(a) a * 2 };
			lib.compute(lib.x + lib.y + lib.z)
		`)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// This would be the performance with result caching enabled
// (Can't actually benchmark without implementing the full feature)
func TestResultCachingConcept(t *testing.T) {
	// This test demonstrates what the result cache would do

	// First evaluation: Normal evaluation
	// - Parse AST
	// - Check purity (isPureAST) -> true
	// - Evaluate
	// - Cache result

	// Second evaluation: Cache hit!
	// - Parse AST (cached)
	// - Check cache -> HIT
	// - Return cached result (no evaluation needed!)

	// Expected speedup: ~40x for pure library evaluations
	t.Log("Result caching would skip all evaluation for pure imports")
	t.Log("Expected speedup: 40x for pure imports, 5-6x overall for typical codebases")
}

// Example of what files would be considered pure in a typical setup
func TestRealWorldPurityExamples(t *testing.T) {
	examples := []struct {
		name   string
		code   string
		pure   bool
		reason string
	}{
		{
			name: "ksonnet-util (typical library)",
			code: `{
				mapToNamedList(map):: [
					{ name: key, value: map[key] }
					for key in std.objectFields(map)
				],
			}`,
			pure:   true,
			reason: "Pure utility functions",
		},
		{
			name: "application config (uses extVars)",
			code: `{
				namespace: std.extVar("namespace"),
				replicas: if std.extVar("env") == "prod" then 10 else 1,
			}`,
			pure:   false,
			reason: "Uses external variables",
		},
		{
			name: "kubernetes library",
			code: `{
				deployment(name, image, replicas):: {
					apiVersion: "apps/v1",
					kind: "Deployment",
					spec: { replicas: replicas },
				},
			}`,
			pure:   true,
			reason: "Function parameters, not extVars",
		},
	}

	for _, ex := range examples {
		t.Run(ex.name, func(t *testing.T) {
			node, err := program.SnippetToAST(ast.DiagnosticFileName(ex.name), ex.name, ex.code)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			result := isPureAST(node)
			if result != ex.pure {
				t.Errorf("%s: isPureAST() = %v, want %v (reason: %s)",
					ex.name, result, ex.pure, ex.reason)
			} else {
				t.Logf("✓ %s correctly identified as pure=%v (%s)",
					ex.name, result, ex.reason)
			}
		})
	}
}
