//go:build goexperiment.arenas

package jsonnet

import (
	"bytes"
	"io"

	"github.com/google/go-jsonnet/ast"
)

// evaluateWithArenaSupport uses arena allocation for evaluation
func evaluateWithArenaSupport(node ast.Node, ext vmExtMap, tla vmExtMap, nativeFuncs map[string]*NativeFunction,
	maxStack int, ic *importCache, traceOut io.Writer, stringOutputMode bool, evalHook EvalHook) (string, error) {

	// Create arena allocator for this evaluation
	arenaAlloc := newArenaAllocator()
	defer arenaAlloc.Free() // Bulk free all evaluation objects!

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return "", err
	}
	i.arena = arenaAlloc // Attach arena to interpreter

	result, err := evaluateAux(i, node, tla)
	if err != nil {
		return "", err
	}

	// Serialize result (copies data out of arena to heap)
	var buf bytes.Buffer
	i.stack.setCurrentTrace(manifestationTrace())
	if stringOutputMode {
		err = i.manifestString(&buf, result)
	} else {
		err = i.manifestAndSerializeJSON(&buf, result, true, "")
	}
	i.stack.clearCurrentTrace()
	if err != nil {
		return "", err
	}
	buf.WriteString("\n")

	// buf.String() is on heap, arena.Free() frees all evaluation memory
	return buf.String(), nil
}

func evaluateMultiWithArenaSupport(node ast.Node, ext vmExtMap, tla vmExtMap, nativeFuncs map[string]*NativeFunction,
	maxStack int, ic *importCache, traceOut io.Writer, stringOutputMode bool, evalHook EvalHook) (map[string]string, error) {

	arenaAlloc := newArenaAllocator()
	defer arenaAlloc.Free()

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return nil, err
	}
	i.arena = arenaAlloc

	result, err := evaluateAux(i, node, tla)
	if err != nil {
		return nil, err
	}

	i.stack.setCurrentTrace(manifestationTrace())
	manifested, err := i.manifestAndSerializeMulti(result, stringOutputMode)
	i.stack.clearCurrentTrace()
	return manifested, err
}

func evaluateStreamWithArenaSupport(node ast.Node, ext vmExtMap, tla vmExtMap, nativeFuncs map[string]*NativeFunction,
	maxStack int, ic *importCache, traceOut io.Writer, evalHook EvalHook) ([]string, error) {

	arenaAlloc := newArenaAllocator()
	defer arenaAlloc.Free()

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return nil, err
	}
	i.arena = arenaAlloc

	result, err := evaluateAux(i, node, tla)
	if err != nil {
		return nil, err
	}

	i.stack.setCurrentTrace(manifestationTrace())
	manifested, err := i.manifestAndSerializeYAMLStream(result)
	i.stack.clearCurrentTrace()
	return manifested, err
}
