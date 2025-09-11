# Cache-Based Rollup Implementation

## Summary

I have successfully replaced the HyperLogLog (HLL) based rollup system in ktranslate with a more efficient cache-based aggregation system. This change improves memory usage and provides better control over resource consumption.

## Changes Made

### 1. Configuration Changes
- **File**: `config.go`
- **Added new fields** to `RollupConfig`:
  - `MaxMemoryMB`: Maximum memory usage for rollup cache in MB (default: 100)
  - `MaxKeys`: Maximum number of keys in rollup cache (default: 5000)
  - `EmergencyCleanup`: Enable emergency cleanup of oldest cache entries (default: true)

### 2. Command-Line Options
- **File**: `cmd/ktranslate/main.go`
- **Added new flags**:
  - `-rollup_max_memory_mb 100`: Memory safety limit in MB
  - `-rollup_max_keys 5000`: Maximum number of cache entries
  - `-rollup_emergency_cleanup true`: Enable automatic cleanup when limits are reached
  - Existing `-rollup_interval 30`: Export interval in seconds (already existed)
  - Existing `-rollup_top_k 1000`: Limit key cardinality (already existed)

### 3. Cache-Based Rollup Implementation
- **New file**: `pkg/rollup/cache.go`
- **Key features**:
  - **Memory tracking**: Estimates and tracks memory usage of cache entries
  - **Emergency aging**: Automatically removes oldest entries when memory/key limits are exceeded
  - **Unified implementation**: Handles both unique and statistical rollups in one system
  - **Better performance**: Direct aggregation instead of probabilistic HLL estimation

### 4. Integration
- **File**: `pkg/rollup/rollup.go`
- **Modified** `GetRollups()` function to use cache-based system for both unique and stats rollups
- **Backward compatibility**: Existing rollup configurations continue to work unchanged

## Technical Details

### Cache Entry Structure
```go
type CacheEntry struct {
    Key         string           // Rollup key
    Sum         uint64          // Aggregated sum
    Count       uint64          // Number of records
    Min         uint64          // Minimum value
    Max         uint64          // Maximum value
    Provider    kt.Provider     // Data provider
    UniqueVals  map[string]bool // For cardinality tracking (unique rollups)
    LastUpdated time.Time       // For age-based cleanup
}
```

### Memory Management
- **Proactive monitoring**: Tracks estimated memory usage in real-time
- **Emergency cleanup**: Removes oldest 50% of entries when limits are exceeded
- **Configurable limits**: Both memory (MB) and key count limits
- **Detailed logging**: Reports cleanup actions and memory statistics

### Performance Improvements
1. **Direct aggregation** vs. HLL probabilistic estimation
2. **Memory-bounded operation** prevents unbounded memory growth
3. **Age-based eviction** keeps cache fresh and relevant
4. **Reduced complexity** - single implementation for all rollup types

## Usage Examples

### Basic Usage (with defaults)
```bash
./ktranslate -rollups "sum,bytes_total,in_bytes+out_bytes,src_addr,dst_addr"
```

### Memory-Constrained Environment
```bash
./ktranslate \
  -rollup_interval 30 \
  -rollup_max_memory_mb 50 \
  -rollup_max_keys 1000 \
  -rollup_emergency_cleanup true \
  -rollups "sum,bytes_total,in_bytes+out_bytes,src_addr,dst_addr"
```

### High-Volume Environment
```bash
./ktranslate \
  -rollup_interval 60 \
  -rollup_max_memory_mb 500 \
  -rollup_max_keys 50000 \
  -rollup_top_k 10000 \
  -rollups "sum,bytes_total,in_bytes+out_bytes,src_addr,dst_addr"
```

## Benefits

1. **Predictable Memory Usage**: Hard limits prevent memory exhaustion
2. **Better Accuracy**: Direct counting vs. probabilistic HLL estimation
3. **Operational Control**: Configurable limits and emergency cleanup
4. **Performance**: Simpler, more direct aggregation logic
5. **Monitoring**: Built-in memory and performance tracking
6. **Backward Compatibility**: Existing configurations work unchanged

## Testing

- ✅ All existing rollup tests pass
- ✅ New cache implementation tests pass
- ✅ Emergency cleanup functionality verified
- ✅ Memory tracking accuracy confirmed
- ✅ Command-line flags properly integrated
- ✅ Full build and compilation successful

The implementation is production-ready and provides the requested cache-based aggregation with memory and key limits, along with emergency aging of old records.
