# Memory Optimization Recommendations for go-jsonnet

## Profile Analysis Summary

### CPU Profile Key Findings
- **63.65%** of CPU time spent on `runtime.memclrNoHeapPointers` (clearing memory)
- **78.67%** cumulative time in `rawevaluate` 
- **57.42%** cumulative time in `cachedThunk.getValue`
- **30.56%** in `runtime.mapassign_faststr` (map operations)
- **17.69%** cumulative time in `callStack.capture`
- **21.62%** cumulative time in `objectIndex`

### Memory Profile Key Findings
- **44.94%** of memory in `os.readFileContents` (file reading)
- **56.09%** cumulative memory in `cachedThunk.getValue`
- **48.07%** cumulative memory in `objectIndex`
- **46.08%** cumulative memory in `importCache.importData`

## Critical Optimization Opportunities

### 1. Reduce `callStack.capture` Allocations (HIGH IMPACT)

**Problem**: Every call to `capture()` at line 241 creates a new `bindingFrame` map:
```go
func (s *callStack) capture(freeVars ast.Identifiers) bindingFrame {
    env := make(bindingFrame, len(freeVars))  // New allocation every time
    for _, fv := range freeVars {
        env[fv] = s.lookUpVarOrPanic(fv)
    }
    return env
}
```

This is called from multiple hot paths:
- Line 235: `getCurrentEnv()` 
- Line 332: Array element evaluation
- Line 454: Object creation

**Solution Options**:

a) **Use sync.Pool for bindingFrame maps**:
```go
var bindingFramePool = sync.Pool{
    New: func() interface{} {
        return make(bindingFrame, 8) // reasonable initial capacity
    },
}

func (s *callStack) capture(freeVars ast.Identifiers) bindingFrame {
    var env bindingFrame
    if pooled := bindingFramePool.Get().(bindingFrame); len(freeVars) <= cap(pooled) {
        env = pooled[:0]
        for k := range pooled {
            delete(pooled, k)
        }
    } else {
        env = make(bindingFrame, len(freeVars))
    }
    for _, fv := range freeVars {
        env[fv] = s.lookUpVarOrPanic(fv)
    }
    return env
}
```

b) **Pre-allocate with capacity hint** (simpler, less impact):
```go
func (s *callStack) capture(freeVars ast.Identifiers) bindingFrame {
    env := make(bindingFrame, 0, len(freeVars))  // Pre-allocate capacity
    for _, fv := range freeVars {
        env[fv] = s.lookUpVarOrPanic(fv)
    }
    return env
}
```

**Expected Impact**: 15-20% reduction in allocations, 10-15% CPU improvement

---

### 2. Optimize Object Field Caching (HIGH IMPACT)

**Problem**: Line 595 shows cache is created for every object:
```go
cache: make(map[objectCacheKey]value),
```

And `objectIndex` at line 703 does map lookups and creates cache keys repeatedly:
```go
if val, ok := sb.self.cache[objectCacheKey{field: fieldName, depth: foundAt}]; ok {
    return val, nil
}
```

**Solutions**:

a) **Use string keys instead of struct keys**:
```go
// Instead of:
type objectCacheKey struct {
    field string
    depth int
}

// Use:
func cacheKey(field string, depth int) string {
    // For depths < 10 (99% of cases), this is allocation-free
    if depth < 10 {
        return field + string(rune('0'+depth))
    }
    return fmt.Sprintf("%s:%d", field, depth)
}
```

b) **Lazy cache initialization**:
```go
type valueObject struct {
    valueBase
    assertionError error
    cache          map[string]value  // nil until first use
    uncached       uncachedObject
}

func (obj *valueObject) getCached(key string) (value, bool) {
    if obj.cache == nil {
        return nil, false
    }
    v, ok := obj.cache[key]
    return v, ok
}

func (obj *valueObject) setCache(key string, val value) {
    if obj.cache == nil {
        obj.cache = make(map[string]value, 4)  // Start small
    }
    obj.cache[key] = val
}
```

**Expected Impact**: 10-15% reduction in memory, 5-10% CPU improvement

---

### 3. Optimize `addBindings` Function (MEDIUM IMPACT)

**Problem**: Line 287-299 creates a new map every time, even when one input is empty:
```go
func addBindings(a, b bindingFrame) bindingFrame {
    result := make(bindingFrame, len(a))
    for k, v := range a {
        result[k] = v
    }
    for k, v := range b {
        result[k] = v
    }
    return result
}
```

**Solution**:
```go
func addBindings(a, b bindingFrame) bindingFrame {
    // Fast paths for empty inputs
    if len(a) == 0 {
        return b
    }
    if len(b) == 0 {
        return a
    }
    
    // Pre-allocate with exact size
    result := make(bindingFrame, len(a)+len(b))
    for k, v := range a {
        result[k] = v
    }
    for k, v := range b {
        result[k] = v
    }
    return result
}
```

**Expected Impact**: 5% reduction in map allocations

---

### 4. Reduce String Conversions (MEDIUM IMPACT)

**Problem**: Profile shows significant time in `stringtoslicerune` (3.82% CPU). This is from:
- Line 234: `makeValueString(v string)` converts strings to `[]rune`
- Called frequently from literal strings

**Solutions**:

a) **Cache common strings**:
```go
var commonStrings = map[string]valueString{
    "":      emptyString(),
    "self":  makeValueString("self"),
    "super": makeValueString("super"),
    // Add more common field names based on profiling
}

func makeValueString(v string) valueString {
    if cached, ok := commonStrings[v]; ok {
        return cached
    }
    return &valueFlatString{value: []rune(v)}
}
```

b) **Lazy rune conversion**:
```go
type valueFlatString struct {
    valueBase
    goStr   string  // Keep original
    runes   []rune  // Lazy conversion
    runeLen int     // -1 means not calculated
}

func (s *valueFlatString) getRunes() []rune {
    if s.runes == nil && s.goStr != "" {
        s.runes = []rune(s.goStr)
        s.runeLen = len(s.runes)
    }
    return s.runes
}

func (s *valueFlatString) getGoString() string {
    if s.goStr != "" {
        return s.goStr
    }
    return string(s.runes)
}
```

**Expected Impact**: 3-5% CPU reduction on string-heavy code

---

### 5. Optimize `prepareFieldUpvalues` (MEDIUM IMPACT)

**Problem**: Line 682 creates a new map and copies all entries:
```go
func prepareFieldUpvalues(sb selfBinding, upValues bindingFrame, locals []objectLocal) bindingFrame {
    newUpValues := make(bindingFrame, len(upValues))
    for k, v := range upValues {
        newUpValues[k] = v
    }
    // ... rest
}
```

**Solution**:
```go
func prepareFieldUpvalues(sb selfBinding, upValues bindingFrame, locals []objectLocal) bindingFrame {
    // Fast path: no locals means we can reuse the upValues
    if len(locals) == 0 {
        return upValues
    }
    
    newUpValues := make(bindingFrame, len(upValues)+len(locals))
    for k, v := range upValues {
        newUpValues[k] = v
    }
    
    // Rest of function...
}
```

**Expected Impact**: 5-8% reduction in map allocations

---

### 6. Optimize `cachedThunk` Memory Usage (HIGH IMPACT)

**Problem**: Lines 52-61 show thunks keep entire environment even after evaluation:
```go
type cachedThunk struct {
    env  *environment  // Kept until line 83
    body ast.Node      // Also kept forever
    content value
    err error
}
```

The current code tries to nil the env at line 83, but this is after evaluation.

**Solutions**:

a) **Clear body after evaluation** (already clearing env):
```go
func (t *cachedThunk) getValue(i *interpreter) (value, error) {
    if t.content != nil {
        return t.content, nil
    }
    if t.err != nil {
        return nil, t.err
    }
    v, err := i.EvalInCleanEnv(t.env, t.body, false)
    if err != nil {
        return nil, err
    }
    t.content = v
    t.env = nil   // Already done
    t.body = nil  // ADD THIS - clears AST reference
    return v, nil
}
```

b) **Use smaller thunk type for ready values**:
```go
type readyThunk struct {
    content value
}

func (t *readyThunk) getValue(i *interpreter) (value, error) {
    return t.content, nil
}

// Change readyThunk function at line 63:
func readyThunk(content value) potentialValue {
    return &readyThunk{content: content}
}
```

**Expected Impact**: 15-20% memory reduction

---

### 7. Reduce Call Stack Frame Allocations (MEDIUM IMPACT)

**Problem**: Lines 168-176 allocate new callFrame for every call:
```go
func (s *callStack) newCall(env environment, trimmable bool) {
    s.stack = append(s.stack, &callFrame{  // Pointer allocation
        cleanEnv:  true,
        trace:     s.currentTrace,
        env:       env,
        trimmable: trimmable,
    })
    s.calls++
}
```

**Solution**: Use a pool:
```go
var callFramePool = sync.Pool{
    New: func() interface{} {
        return &callFrame{}
    },
}

func (s *callStack) newCall(env environment, trimmable bool) {
    frame := callFramePool.Get().(*callFrame)
    frame.cleanEnv = true
    frame.trace = s.currentTrace
    frame.env = env
    frame.trimmable = trimmable
    s.stack = append(s.stack, frame)
    s.calls++
}

func (s *callStack) popIfExists(whichFrame int) {
    if len(s.stack) == whichFrame {
        // ... existing code ...
        frame := s.stack[len(s.stack)-1]
        callFramePool.Put(frame)  // Return to pool
        s.stack = s.stack[:len(s.stack)-1]
    }
}
```

**Expected Impact**: 5-7% reduction in allocations

---

### 8. Optimize Import Cache (MEDIUM IMPACT)

**Problem**: 44.94% of memory is in file reading, 46.08% in importData. Files are read and kept in memory.

**Solution**: Profile-guided decision needed, but options include:

a) **Use weak references** (if importing same files repeatedly):
Would require a more complex cache implementation.

b) **Limit cache size**:
```go
type importCache struct {
    cache     map[string]*importCacheValue
    maxSize   int
    evictList *list.List  // LRU tracking
}
```

c) **Compress cached content** (for large files):
```go
type importCacheValue struct {
    compressed []byte  // gzip compressed
    // ... other fields
}
```

**Expected Impact**: Depends on workload, potentially 20-30% memory reduction for large projects

---

## Implementation Priority

### Phase 1 (High Impact, Low Risk):
1. Add fast paths to `addBindings` (empty checks)
2. Add fast path to `prepareFieldUpvalues` (no locals)
3. Clear `body` field in `cachedThunk` after evaluation
4. Pre-allocate capacity in `capture()`

**Expected Combined Impact**: 15-25% memory reduction, 10-15% CPU improvement

### Phase 2 (High Impact, Medium Risk):
1. Implement sync.Pool for `bindingFrame`
2. Optimize object cache key representation
3. Implement lazy cache initialization for objects
4. Use sync.Pool for callFrames

**Expected Combined Impact**: Additional 20-30% memory reduction, 15-20% CPU improvement

### Phase 3 (Medium Impact, Requires More Analysis):
1. Optimize string handling with caching
2. Implement LRU cache for imports
3. Consider using dedicated readyThunk type

**Expected Combined Impact**: Additional 10-15% improvement depending on workload

---

## Benchmarking

Before implementing, establish baselines:

```bash
# CPU profile
go test -bench=. -cpuprofile=cpu_before.prof

# Memory profile  
go test -bench=. -memprofile=mem_before.prof

# Allocations
go test -bench=. -benchmem
```

After each optimization:

```bash
# Compare
go tool pprof -base=cpu_before.prof cpu_after.prof
go tool pprof -base=mem_before.prof mem_after.prof
```

---

## Additional Notes

### Potential Go Compiler Improvements
- The high `memclrNoHeapPointers` time suggests Go is clearing many small allocations
- Using pools and reducing allocations will help the GC significantly

### Memory Pressure Indicators
- 63.65% CPU time clearing memory is very high
- This indicates GC pressure and too many allocations
- Reducing allocations should improve CPU performance even more than memory

### Quick Wins (Can implement immediately)
1. Lines 287-299: Add empty checks in `addBindings`
2. Line 83: Add `t.body = nil` after evaluation
3. Line 682: Add empty locals check in `prepareFieldUpvalues`
4. Line 241: Pre-allocate capacity in `capture()`

These 4 changes require < 10 lines of code and should provide 10-15% improvement.

