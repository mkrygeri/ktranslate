# Cache-Based Rollup Implementation

This document describes the new cache-based rollup implementation that replaces the HyperLogLog (HLL) approach for improved efficiency and better resource management.

## Overview

The cache-based rollup system provides:

1. **Direct aggregation** - Instead of probabilistic cardinality estimation with HLL, uses exact counting and aggregation
2. **Memory management** - Configurable memory limits with emergency cleanup 
3. **Key cardinality control** - Limits the number of unique keys to prevent unbounded growth
4. **Better performance** - More efficient for typical network flow aggregation use cases

## New Command-Line Options

### `-rollup_max_memory_mb <MB>`
- **Default**: 100
- **Description**: Maximum memory usage for rollup cache in MB
- **Example**: `-rollup_max_memory_mb 500`

### `-rollup_max_keys <count>`
- **Default**: 5000  
- **Description**: Maximum number of keys in rollup cache
- **Example**: `-rollup_max_keys 10000`

### `-rollup_emergency_cleanup <true|false>`
- **Default**: true
- **Description**: Enable emergency cleanup of oldest cache entries when limits are reached
- **Example**: `-rollup_emergency_cleanup false`

### `-rollup_interval <seconds>` (existing option)
- **Default**: 0 (disabled)
- **Description**: Export timer for rollups in seconds
- **Example**: `-rollup_interval 30`

### `-rollup_top_k <count>` (existing option)
- **Default**: 10
- **Description**: Export only the top K values
- **Example**: `-rollup_top_k 1000`

## Usage Examples

### Basic Cache-Based Aggregation
```bash
./ktranslate \
  -listen 0.0.0.0:9995 \
  -rollups "sum,bytes_total,in_bytes+out_bytes,src_addr" \
  -rollup_interval 30 \
  -rollup_max_memory_mb 200 \
  -rollup_max_keys 10000 \
  -rollup_top_k 1000
```

### Aggressive Cleanup for High-Cardinality Data
```bash
./ktranslate \
  -listen 0.0.0.0:9995 \
  -rollups "unique,unique_dests,dst_addr,src_addr" \
  -rollup_interval 30 \
  -rollup_max_memory_mb 100 \
  -rollup_max_keys 5000 \
  -rollup_emergency_cleanup true \
  -rollup_top_k 1000
```

### Memory-Constrained Environment
```bash
./ktranslate \
  -listen 0.0.0.0:9995 \
  -rollups "sum,traffic_by_src,in_bytes,src_addr" \
  -rollup_interval 15 \
  -rollup_max_memory_mb 50 \
  -rollup_max_keys 2000 \
  -rollup_emergency_cleanup true \
  -rollup_top_k 500
```

## Emergency Cleanup Behavior

When either memory or key limits are exceeded:

1. **Age Calculation**: All cache entries are sorted by last update time
2. **Threshold Selection**: The oldest 50% of entries are identified for removal
3. **Cleanup Execution**: Entries older than the threshold are removed
4. **Logging**: Cleanup activity is logged with statistics

Emergency cleanup prevents memory exhaustion while preserving the most recently active data.

## Performance Characteristics

### Compared to HLL-Based System:

**Advantages:**
- **Exact Results**: No probabilistic approximation errors
- **Better Memory Efficiency**: Direct aggregation without HLL data structures
- **Predictable Performance**: Linear complexity with controlled resource usage
- **Real-time Monitoring**: Actual memory usage tracking

**Trade-offs:**
- **Memory Growth**: Linear growth with unique key count vs. constant HLL size
- **Cleanup Overhead**: Periodic cleanup operations vs. constant memory usage

### Recommended Settings by Use Case:

| Use Case | Memory Limit | Key Limit | Interval | Top-K |
|----------|-------------|-----------|----------|-------|
| Low-cardinality aggregation | 50MB | 2,000 | 60s | 100 |
| Medium-cardinality aggregation | 100MB | 5,000 | 30s | 500 |
| High-cardinality aggregation | 200MB | 10,000 | 15s | 1,000 |
| Memory-constrained | 25MB | 1,000 | 30s | 50 |

## Monitoring

The system logs emergency cleanup events with detailed statistics:

```
Starting emergency cleanup - current cache size: 6000 entries, memory: 150.42 MB
Emergency cleanup completed - removed 3000 entries older than 45.2s, new cache size: 3000 entries, memory: 75.21 MB
```

Monitor these logs to tune your memory and key limits appropriately for your traffic patterns.
