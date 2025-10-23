//go:build !goexperiment.arenas

package jsonnet

// arenaAllocator is a no-op when arenas are not available
type arenaAllocator struct{}

func newArenaAllocator() *arenaAllocator {
	return &arenaAllocator{}
}

func (a *arenaAllocator) Free() {
	// no-op
}

// Allocate values on heap (normal allocation)
func (a *arenaAllocator) newValueBoolean(v bool) *valueBoolean {
	return &valueBoolean{value: v}
}

func (a *arenaAllocator) newValueNumber(v float64) *valueNumber {
	return &valueNumber{value: v}
}

func (a *arenaAllocator) newValueNull() *valueNull {
	return &valueNull{}
}

func (a *arenaAllocator) newValueString(runes []rune) *valueFlatString {
	return &valueFlatString{value: runes}
}

func (a *arenaAllocator) newValueArray(elements []*cachedThunk) *valueArray {
	return &valueArray{elements: elements}
}

func (a *arenaAllocator) newCallFrame() *callFrame {
	return &callFrame{}
}

