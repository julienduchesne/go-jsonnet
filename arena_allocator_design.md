# Using Go's Arena Allocator for go-jsonnet

## Go Arena Support

Go 1.20+ has **experimental arena allocation** via the [`arena` package](https://go.dev/src/arena/arena.go):

```go
//go:build goexperiment.arenas

import "arena"

// Create arena
a := arena.NewArena()
defer a.Free()  // Free all at once!

// Allocate in arena
ptr := arena.New[MyStruct](a)
slice := arena.MakeSlice[int](a, len, cap)
```

**Benefits:**
- Bulk allocation/deallocation
- Reduced GC pressure
- Manual memory management
- Perfect for scoped allocations

## Key Insight from Your Profile

**Top allocations (by count):**
```
441M objects  - rawevaluate (general)
389M objects  - makeValueString (18.49%)
247M objects  - callStack.capture (11.72%)
207M objects  - callStack.newCall (9.82%)
 82M objects  - addBindings (3.90%)
```

**These are all short-lived allocations within a single evaluation!**

## Perfect Use Case for Arenas

Each Jsonnet evaluation:
1. Allocates millions of objects
2. Uses them during evaluation
3. Discards them all when done

**This is exactly what arenas are for!**

```
┌─────────────────────────────────────────┐
│ Evaluation 1                            │
│ ┌─────────────────────────────────────┐ │
│ │ Arena 1                             │ │
│ │  - 100K bindingFrames              │ │
│ │  - 200K valueStrings               │ │
│ │  - 150K callFrames                 │ │
│ │  - All evaluation objects          │ │
│ └─────────────────────────────────────┘ │
│ Result: "{ json output }"               │
│ arena.Free() → All memory freed!        │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│ Evaluation 2                            │
│ ┌─────────────────────────────────────┐ │
│ │ Arena 2 (reuses memory!)            │ │
│ │  - New objects for this eval        │ │
│ └─────────────────────────────────────┘ │
│ arena.Free() → Instant cleanup!         │
└─────────────────────────────────────────┘
```

## Implementation Design

### Option 1: Arena per Evaluation (RECOMMENDED)

```go
type interpreter struct {
    traceOut    io.Writer
    extVars     map[string]*cachedThunk
    nativeFuncs map[string]*NativeFunction
    baseStd     *valueObject
    importCache *importCache
    stack       callStack
    evalHook    EvalHook
    arena       *arena.Arena  // NEW: Arena for this evaluation
}

func evaluate(node ast.Node, ext vmExtMap, tla vmExtMap, ...) (string, error) {
    // Create arena for this evaluation
    a := arena.NewArena()
    defer a.Free()  // Free everything when done!
    
    i, err := buildInterpreter(ext, nativeFuncs, maxStack, ic, traceOut, evalHook)
    if err != nil {
        return "", err
    }
    i.arena = a  // Attach arena
    
    result, err := evaluateAux(i, node, tla)
    if err != nil {
        return "", err
    }
    
    // Serialize result (copies out of arena)
    var buf bytes.Buffer
    err = i.manifestAndSerializeJSON(&buf, result, true, "")
    
    // After this point, arena.Free() is called automatically
    // All evaluation objects freed in bulk!
    return buf.String(), nil
}
```

### Option 2: Shared Arena Across Evaluations

```go
type VM struct {
    MaxStack       int
    ext            vmExtMap
    tla            vmExtMap
    nativeFuncs    map[string]*NativeFunction
    importer       Importer
    ErrorFormatter ErrorFormatter
    StringOutput   bool
    importCache    *importCache
    traceOut       io.Writer
    EvalHook       EvalHook
    arenaPool      *ArenaPool  // NEW: Pool of reusable arenas
}

type ArenaPool struct {
    arenas chan *arena.Arena
}

func NewArenaPool(size int) *ArenaPool {
    pool := &ArenaPool{arenas: make(chan *arena.Arena, size)}
    for i := 0; i < size; i++ {
        pool.arenas <- arena.NewArena()
    }
    return pool
}

func (p *ArenaPool) Get() *arena.Arena {
    return <-p.arenas
}

func (p *ArenaPool) Put(a *arena.Arena) {
    a.Free()  // Clear it
    // Recreate for reuse
    p.arenas <- arena.NewArena()
}

// In VM:
func (vm *VM) EvaluateFile(filename string) (json string, err error) {
    a := vm.arenaPool.Get()
    defer vm.arenaPool.Put(a)
    
    // Use arena for evaluation
    // ...
}
```

## What to Allocate in Arena

### High-Value Targets (Millions of Allocations)

#### 1. bindingFrame Maps (247M allocations, 11.72%)

```go
// Current:
func (s *callStack) capture(freeVars ast.Identifiers) bindingFrame {
    env := make(bindingFrame, len(freeVars))
    // ...
}

// With Arena:
func (s *callStack) capture(freeVars ast.Identifiers, a *arena.Arena) bindingFrame {
    if len(freeVars) == 0 {
        return bindingFrame{}
    }
    // Allocate map in arena
    env := arena.MakeMap[ast.Identifier, *cachedThunk](a, len(freeVars))
    // ...
}
```

**Note:** Maps can't be directly allocated in arenas (yet), but we can use arena-allocated slices to build maps:

```go
type arenaBindingFrame struct {
    keys   []ast.Identifier
    values []*cachedThunk
}

func makeArenaBindingFrame(a *arena.Arena, size int) *arenaBindingFrame {
    bf := arena.New[arenaBindingFrame](a)
    bf.keys = arena.MakeSlice[ast.Identifier](a, 0, size)
    bf.values = arena.MakeSlice[*cachedThunk](a, 0, size)
    return bf
}
```

#### 2. callFrame Structs (207M allocations, 9.82%)

```go
// Current:
func (s *callStack) newCall(env environment, trimmable bool) {
    s.stack = append(s.stack, &callFrame{
        cleanEnv:  true,
        trace:     s.currentTrace,
        env:       env,
        trimmable: trimmable,
    })
}

// With Arena:
func (s *callStack) newCall(env environment, trimmable bool, a *arena.Arena) {
    frame := arena.New[callFrame](a)  // Allocate in arena!
    frame.cleanEnv = true
    frame.trace = s.currentTrace
    frame.env = env
    frame.trimmable = trimmable
    s.stack = append(s.stack, frame)
}
```

#### 3. Value Objects (389M strings, 42M booleans, 28M numbers)

```go
// Current:
func makeValueString(v string) valueString {
    return &valueFlatString{value: []rune(v)}
}

// With Arena:
func makeValueString(v string, a *arena.Arena) valueString {
    vfs := arena.New[valueFlatString](a)
    vfs.value = arena.MakeSlice[rune](a, len([]rune(v)), len([]rune(v)))
    copy(vfs.value, []rune(v))
    return vfs
}

func makeValueBoolean(v bool, a *arena.Arena) *valueBoolean {
    vb := arena.New[valueBoolean](a)
    vb.value = v
    return vb
}

func makeValueNumber(v float64, a *arena.Arena) *valueNumber {
    vn := arena.New[valueNumber](a)
    vn.value = v
    return vn
}
```

## Performance Impact

### Current (With VM Reuse, No Arena)

```
Evaluation of 10 environments:
- Total allocations: ~2 billion objects
- GC pauses: ~500ms (distributed)
- Memory pressure: High
- Total time: ~2500ms

Memory pattern:
Alloc → Use → GC marks → GC sweeps → Repeat
```

### With Arena Allocation

```
Evaluation of 10 environments:
- Arena allocations: ~2 billion objects
- GC pauses: ~50ms (much less frequent!)
- Memory pressure: Low (bulk free)
- Total time: ~1500ms (40% faster!)

Memory pattern:
Arena alloc → Use → arena.Free() → Done!
(No GC scanning needed for arena memory)
```

**Expected improvements:**
- 30-50% faster (reduced GC overhead)
- Lower memory pressure
- More predictable performance
- Better cache locality (arena memory is contiguous)

## Implementation Complexity

### Phase 1: Basic Arena Integration (~200 lines)

```go
// Add arena field to interpreter
type interpreter struct {
    // ... existing fields
    arena *arena.Arena  // NEW
}

// Add arena parameter to allocation functions
func makeValueString(v string, a *arena.Arena) valueString
func makeValueNumber(v float64, a *arena.Arena) *valueNumber
func makeValueBoolean(v bool, a *arena.Arena) *valueBoolean

// Modify evaluate function
func evaluate(...) (string, error) {
    a := arena.NewArena()
    defer a.Free()
    
    i, err := buildInterpreter(...)
    i.arena = a
    
    // ... rest of evaluation
}
```

### Phase 2: Optimize Hot Paths (~500 lines)

- Arena-allocate callFrames
- Arena-allocate binding frames
- Arena-allocate slices for arrays
- Arena-allocate object field maps

### Phase 3: Shared Arena Pool (~100 lines)

- Create ArenaPool type
- Integrate with VM
- Handle concurrent evaluations

## Caveats and Solutions

### 1. Result Must Escape Arena

The final result must be copied out of arena before `Free()`:

```go
func evaluate(...) (string, error) {
    a := arena.NewArena()
    defer a.Free()
    
    // ... evaluation in arena ...
    
    // Serialize to string (copies data out)
    var buf bytes.Buffer
    i.manifestAndSerializeJSON(&buf, result, true, "")
    
    // buf.String() is NOT in arena - safe to return!
    return buf.String(), nil
}
```

### 2. Cached Values Must Not Use Arena

Values in `importCache.codeCache` persist across evaluations:

```go
func (cache *importCache) importCode(...) (value, error) {
    // ... 
    
    // Don't use arena for cached values!
    if cachedPV, isCached := cache.codeCache[foundAt]; !isCached {
        env := makeInitialEnv(foundAt, i.baseStd)
        // Use heap allocation (not arena) for cached thunks
        pv = &cachedThunk{env: &env, body: node}
        cache.codeCache[foundAt] = pv
    }
    
    // ...
}
```

**Solution:** Have two allocation modes:
```go
type allocMode int
const (
    allocHeap  allocMode = iota  // Normal heap (for cached values)
    allocArena                    // Arena (for evaluation)
)

func makeValueString(v string, mode allocMode, a *arena.Arena) valueString {
    if mode == allocArena && a != nil {
        // Arena allocation
        return arenaAllocValueString(v, a)
    }
    // Heap allocation
    return &valueFlatString{value: []rune(v)}
}
```

### 3. Maps Don't Directly Support Arenas

Go 1.22's arena package doesn't support map allocation yet. Options:

**A) Use slices instead of maps (for small sizes)**
```go
type arenaBindingFrame struct {
    entries []bindingEntry
}

type bindingEntry struct {
    key ast.Identifier
    val *cachedThunk
}
```

**B) Wait for map support (future Go versions)**

**C) Use heap maps, arena everything else**
```go
// Keep using heap maps for now
// Still get benefits from arena-allocating:
// - callFrames (10% of objects)
// - value objects (50% of objects)  
// - slices (30% of objects)
// Total: ~90% of allocations in arena!
```

## Build and Usage

### Enable Arena Support

```bash
# Build with arena support
GOEXPERIMENT=arenas go build

# Or add build tag to files
//go:build goexperiment.arenas
```

### Backward Compatibility

```go
//go:build goexperiment.arenas
// +build goexperiment.arenas

package jsonnet

import "arena"

func evaluate(...) (string, error) {
    a := arena.NewArena()
    defer a.Free()
    // ... use arena
}
```

```go
//go:build !goexperiment.arenas
// +build !goexperiment.arenas

package jsonnet

func evaluate(...) (string, error) {
    // Regular heap allocation
}
```

## Expected Performance

### Benchmark Estimates

```
Without arena (current):
- 10 evaluations
- 2B allocations
- GC: 500ms cumulative
- Total: 2500ms

With arena:
- 10 evaluations  
- 2B arena allocations
- GC: 50ms cumulative (10x less!)
- Total: 1500ms (40% faster)

Additional benefits:
- Better cache locality
- Predictable performance
- Lower tail latencies
```

### Combined with All Optimizations

```
Baseline (no optimization):        12000ms
+ VM reuse:                         2500ms (4.8x)
+ Tier 1 optimizations:             2100ms (5.7x)
+ Arena allocator:                  1200ms (10x)
+ Result cache (pure files):         400ms (30x!)
```

## Recommendation

**Phase 1: Prototype with Value Objects**

Start with the easiest, highest-impact items:
1. Arena-allocate valueString, valueNumber, valueBoolean (50% of objects)
2. Arena-allocate callFrames (10% of objects)
3. Measure improvement

**Expected: 30-40% improvement with ~200 lines of code**

**Phase 2: Full Arena Integration**

If Phase 1 works well:
1. Arena-allocate slices
2. Find workaround for maps (slice-based or wait for Go support)
3. Integrate with VM pool

**Expected: 40-50% total improvement**

**Phase 3: Arena Pool**

For concurrent evaluations:
1. Create arena pool
2. Reuse arenas across evaluations
3. Tune arena sizes

## Summary

Yes, Go has arena support and it's **perfect** for go-jsonnet!

**Why it works so well:**
- Evaluation is naturally scoped
- Millions of short-lived allocations
- All discarded at once
- Reduces GC pressure dramatically

**Implementation:**
- ~200 lines for Phase 1
- Backward compatible with build tags
- Can be done incrementally

**Expected improvement:**
- 30-50% faster
- Lower GC overhead
- More predictable performance

**Combined with all optimizations: 30x total speedup possible!**

Would you like me to implement a prototype?

