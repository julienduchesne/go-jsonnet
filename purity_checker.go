package jsonnet

import (
	"github.com/google/go-jsonnet/ast"
	"github.com/google/go-jsonnet/internal/parser"
)

// isPureAST checks if an AST node and its children don't use external variables or TLAs.
// A pure AST can have its evaluation result cached because the result will always be the same.
func isPureAST(node ast.Node) bool {
	return checkPurity(node)
}

// checkPurity recursively checks if a node and its children are pure
func checkPurity(node ast.Node) bool {
	if node == nil {
		return true
	}

	// Check current node
	if n, ok := node.(*ast.Apply); ok {
		// Check if calling std.extVar
		if isStdFunctionCall(n.Target, "extVar") {
			return false
		}
	}

	// Recursively check children
	for _, child := range parser.Children(node) {
		if !checkPurity(child) {
			return false
		}
	}

	return true
}

// isStdFunctionCall checks if a node represents a call to std.functionName
func isStdFunctionCall(target ast.Node, functionName string) bool {
	index, ok := target.(*ast.Index)
	if !ok {
		return false
	}

	// Check if indexing into 'std'
	varNode, ok := index.Target.(*ast.Var)
	if !ok || varNode.Id != "std" {
		return false
	}

	// Check if the field is our target function
	fieldNode, ok := index.Index.(*ast.LiteralString)
	if !ok || fieldNode.Value != functionName {
		return false
	}

	return true
}

// Enhanced import cache with result caching for pure files
type resultCacheEntry struct {
	result value
	isPure bool
}

// This would be added to importCache in imports.go:
/*
type importCache struct {
	foundAtVerification map[string]Contents
	astCache            map[string]ast.Node
	codeCache           map[string]potentialValue
	resultCache         map[string]resultCacheEntry // NEW
	importer            Importer
}

// Enhanced importCode with result caching
func (cache *importCache) importCode(importedFrom, importedPath string, i *interpreter) (value, error) {
	node, foundAt, err := cache.importAST(importedFrom, importedPath)
	if err != nil {
		return nil, i.Error(err.Error())
	}

	// Check if we have a cached pure result
	if entry, ok := cache.resultCache[foundAt]; ok && entry.isPure {
		return entry.result, nil // Instant return for pure files!
	}

	// Evaluate using existing code
	var pv potentialValue
	if cachedPV, isCached := cache.codeCache[foundAt]; !isCached {
		env := makeInitialEnv(foundAt, i.baseStd)
		pv = &cachedThunk{env: &env, body: node, content: nil}
		cache.codeCache[foundAt] = pv
	} else {
		pv = cachedPV
	}

	result, err := i.evaluatePV(pv)
	if err != nil {
		return nil, err
	}

	// Cache result if the file is pure
	isPure := isPureAST(node)
	if isPure {
		cache.resultCache[foundAt] = resultCacheEntry{
			result: result,
			isPure: true,
		}
	}

	return result, nil
}

// Add to flushValueCache to clear result cache too
func (cache *importCache) flushValueCache() {
	cache.codeCache = make(map[string]potentialValue)
	cache.resultCache = make(map[string]resultCacheEntry) // NEW
}
*/
