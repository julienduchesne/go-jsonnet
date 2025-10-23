//go:build !goexperiment.arenas

package jsonnet

import (
	"bytes"
	"io"

	"github.com/google/go-jsonnet/ast"
)

// evaluateWithArenaSupport falls back to regular heap allocation when arenas are not available
func evaluateWithArenaSupport(node ast.Node, ext vmExtMap, tla vmExtMap, nativeFuncs map[string]*NativeFunction,
	maxStack int, ic *importCache, traceOut io.Writer, stringOutputMode bool, evalHook EvalHook) (string, error) {

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return "", err
	}

	result, err := evaluateAux(i, node, tla)
	if err != nil {
		return "", err
	}

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
	return buf.String(), nil
}

func evaluateMultiWithArenaSupport(node ast.Node, ext vmExtMap, tla vmExtMap, nativeFuncs map[string]*NativeFunction,
	maxStack int, ic *importCache, traceOut io.Writer, stringOutputMode bool, evalHook EvalHook) (map[string]string, error) {

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return nil, err
	}

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

	i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
	if err != nil {
		return nil, err
	}

	result, err := evaluateAux(i, node, tla)
	if err != nil {
		return nil, err
	}

	i.stack.setCurrentTrace(manifestationTrace())
	manifested, err := i.manifestAndSerializeYAMLStream(result)
	i.stack.clearCurrentTrace()
	return manifested, err
}
