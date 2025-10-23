//go:build goexperiment.arenas

package jsonnet

import (
	"arena"
)

// arenaAllocator wraps arena for use in interpreter
type arenaAllocator struct {
	arena *arena.Arena
}

func newArenaAllocator() *arenaAllocator {
	return &arenaAllocator{
		arena: arena.NewArena(),
	}
}

func (a *arenaAllocator) Free() {
	if a.arena != nil {
		a.arena.Free()
	}
}

// Allocate values in arena
func (a *arenaAllocator) newValueBoolean(v bool) *valueBoolean {
	vb := arena.New[valueBoolean](a.arena)
	vb.value = v
	return vb
}

func (a *arenaAllocator) newValueNumber(v float64) *valueNumber {
	vn := arena.New[valueNumber](a.arena)
	vn.value = v
	return vn
}

func (a *arenaAllocator) newValueNull() *valueNull {
	return arena.New[valueNull](a.arena)
}

func (a *arenaAllocator) newValueString(runes []rune) *valueFlatString {
	vfs := arena.New[valueFlatString](a.arena)
	// Allocate rune slice in arena
	vfs.value = arena.MakeSlice[rune](a.arena, len(runes), len(runes))
	copy(vfs.value, runes)
	return vfs
}

func (a *arenaAllocator) newValueArray(elements []*cachedThunk) *valueArray {
	arr := arena.New[valueArray](a.arena)
	if len(elements) > 0 {
		arr.elements = arena.MakeSlice[*cachedThunk](a.arena, len(elements), len(elements))
		copy(arr.elements, elements)
	}
	return arr
}

func (a *arenaAllocator) newCallFrame() *callFrame {
	return arena.New[callFrame](a.arena)
}

