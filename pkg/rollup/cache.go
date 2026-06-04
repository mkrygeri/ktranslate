package rollup

import (
	"runtime"
	"sort"
	"sync"
	"time"
	"unsafe"

	"github.com/kentik/ktranslate"
	"github.com/kentik/ktranslate/pkg/eggs/logger"
	"github.com/kentik/ktranslate/pkg/kt"
)

const (
	// Estimated overhead per cache entry in bytes
	CACHE_ENTRY_OVERHEAD = 150
	// Age threshold multiplier for emergency cleanup
	EMERGENCY_AGE_THRESHOLD = 0.5
)

// CacheEntry represents a single cache entry with metrics and metadata
type CacheEntry struct {
	Key         string
	Sum         uint64
	Count       uint64
	Min         uint64
	Max         uint64
	Bytes       uint64 // Total bytes for this tuple (in_bytes + out_bytes), sample-rate adjusted.
	Packets     uint64 // Total packets for this tuple (in_pkts + out_pkts), sample-rate adjusted.
	TCPFlags    uint64 // Union (OR) of TCP flags seen for this flow.
	Provider    kt.Provider
	UniqueVals  map[string]bool // For unique rollups - simplified cardinality tracking
	LastUpdated time.Time
}

// CacheRollup implements a cache-based rollup system
type CacheRollup struct {
	logger.ContextL
	rollupBase
	cache        map[string]*CacheEntry
	config       *ktranslate.RollupConfig
	isUnique     bool
	stitchFlows  bool
	mux          sync.RWMutex
	exportKvs    chan chan []Rollup
	memoryUsage  int64 // Approximate memory usage in bytes
}

func newCacheRollup(log logger.Underlying, rd RollupDef, cfg *ktranslate.RollupConfig, isUnique bool) (*CacheRollup, error) {
	r := &CacheRollup{
		ContextL:     logger.NewContextLFromUnderlying(logger.SContext{S: "cacheRollup"}, log),
		cache:        make(map[string]*CacheEntry),
		config:       cfg,
		isUnique:     isUnique,
		exportKvs:    make(chan chan []Rollup),
		memoryUsage:  0,
	}

	r.keyJoin = cfg.JoinKey
	r.topK = cfg.TopK

	err := r.init(rd)
	if err != nil {
		return nil, err
	}

	// Enable RFC-5103 flow stitching only when requested and the dimensions
	// actually describe a reversible flow tuple.
	if cfg.StitchFlows {
		if r.canStitch {
			r.stitchFlows = true
		} else {
			r.Warnf("Flow stitching requested but dimensions are not a reversible flow tuple for %s; reverse metrics disabled", rd.String())
		}
	}

	// Start the export goroutine
	go r.exportLoop()

	r.Infof("New Cache Rollup: %s -> %s (unique=%v, stitch=%v)", r.eventType, rd.String(), isUnique, r.stitchFlows)
	return r, nil
}

func (r *CacheRollup) Add(in []map[string]interface{}) {
	if r.hasFilters {
		in = r.filter(in)
	}

	r.mux.Lock()
	defer r.mux.Unlock()

	now := time.Now()

	for _, mapr := range in {
		key := r.getKey(mapr)
		if key == "" {
			continue
		}

		// Get or create cache entry
		entry, exists := r.cache[key]
		if !exists {
			entry = &CacheEntry{
				Key:         key,
				Sum:         0,
				Count:       0,
				Min:         ^uint64(0), // Max uint64
				Max:         0,
				Provider:    mapr["provider"].(kt.Provider),
				LastUpdated: now,
			}

			if r.isUnique {
				entry.UniqueVals = make(map[string]bool)
			}

			r.cache[key] = entry
			r.memoryUsage += r.estimateEntrySize(entry)

			// Check if we need emergency cleanup
			if r.shouldDoEmergencyCleanup() {
				r.doEmergencyCleanup()
			}
		}

		entry.LastUpdated = now
		entry.Count++

		if r.isUnique {
			// Handle unique values for cardinality tracking
			r.addUniqueValues(entry, mapr)
		} else {
			// Handle sum, min, max calculations
			r.addStatValues(entry, mapr)
		}
	}
}

func (r *CacheRollup) addStatValues(entry *CacheEntry, mapr map[string]interface{}) {
	sr := uint64(mapr["sample_rate"].(int64))
	value := uint64(0)

	// Calculate value from metrics
	for _, metric := range r.metrics {
		if mm, ok := mapr[metric]; ok {
			if m, ok := mm.(int64); ok {
				value += uint64(m)
			}
		}
	}

	for _, m := range r.multiMetrics {
		if m1, ok := mapr[m[0]]; ok {
			switch mm := m1.(type) {
			case map[string]int32:
				value += uint64(mm[m[1]])
			case map[string]int64:
				value += uint64(mm[m[1]])
			}
		}
	}

	// Apply sample rate if configured
	if r.sample && sr > 0 {
		value *= sr
	}

	// Update stats
	entry.Sum += value
	if entry.Min > value {
		entry.Min = value
	}
	if entry.Max < value {
		entry.Max = value
	}

	// Accumulate auxiliary per-flow accounting fields used for biflow records.
	// Each flow record reports its bytes/packets in EITHER the in_* or out_*
	// fields depending on the exporter's ingress/egress direction flag, so we sum
	// both to get the true total for this tuple direction. TCP flags are unioned.
	var bytes, pkts uint64
	if v, ok := mapr["in_bytes"].(int64); ok {
		bytes += uint64(v)
	}
	if v, ok := mapr["out_bytes"].(int64); ok {
		bytes += uint64(v)
	}
	if v, ok := mapr["in_pkts"].(int64); ok {
		pkts += uint64(v)
	}
	if v, ok := mapr["out_pkts"].(int64); ok {
		pkts += uint64(v)
	}
	if r.sample && sr > 0 {
		bytes *= sr
		pkts *= sr
	}
	entry.Bytes += bytes
	entry.Packets += pkts
	if tf, ok := mapr["tcp_flags"].(int64); ok {
		entry.TCPFlags |= uint64(tf)
	}
}

func (r *CacheRollup) addUniqueValues(entry *CacheEntry, mapr map[string]interface{}) {
	// Add unique values for cardinality estimation
	for _, metric := range r.metrics {
		if mm, ok := mapr[metric]; ok {
			switch m := mm.(type) {
			case string:
				entry.UniqueVals[m] = true
			case int64:
				entry.UniqueVals[string(rune(m))] = true
			}
		}
	}

	for _, m := range r.multiMetrics {
		if m1, ok := mapr[m[0]]; ok {
			switch mm := m1.(type) {
			case map[string]string:
				entry.UniqueVals[mm[m[1]]] = true
			case map[string]int32:
				entry.UniqueVals[string(rune(mm[m[1]]))] = true
			case map[string]int64:
				entry.UniqueVals[string(rune(mm[m[1]]))] = true
			}
		}
	}
}

func (r *CacheRollup) Export() []Rollup {
	rc := make(chan []Rollup)
	r.exportKvs <- rc
	return <-rc
}

func (r *CacheRollup) exportLoop() {
	for {
		select {
		case rc := <-r.exportKvs:
			go r.doExport(rc)
		}
	}
}

func (r *CacheRollup) doExport(rc chan []Rollup) {
	r.mux.Lock()
	oldCache := r.cache
	r.cache = make(map[string]*CacheEntry)
	r.memoryUsage = 0
	r.mux.Unlock()

	if len(oldCache) == 0 {
		rc <- nil
		return
	}

	ot := r.dtime
	r.dtime = time.Now()
	
	keys := make([]Rollup, 0, len(oldCache))

	for _, entry := range oldCache {
		var metric float64
		if r.isUnique {
			metric = float64(len(entry.UniqueVals))
		} else {
			metric = float64(entry.Sum)
		}

		rollup := Rollup{
			Name:      r.name,
			EventType: r.eventType,
			Dimension: entry.Key,
			Metric:    metric,
			KeyJoin:   r.keyJoin,
			dims:      combo(r.multiDims),
			Interval:  r.dtime.Sub(ot),
			Count:     entry.Count,
			Min:       entry.Min,
			Max:       entry.Max,
			Provider:  entry.Provider,
		}

		// Forward-direction auxiliary accounting fields.
		rollup.Bytes = entry.Bytes
		rollup.Packets = entry.Packets
		rollup.TCPFlags = entry.TCPFlags

		// RFC-5103 biflow stitching: attach the reverse-direction metrics by
		// looking up the responder->initiator flow in the same cache window. If
		// no matching reverse flow exists, the reverse counters remain 0.
		if r.stitchFlows {
			rollup.IsBiflow = true
			if rev, ok := oldCache[r.reverseKey(entry.Key)]; ok {
				if r.isUnique {
					rollup.MetricRev = float64(len(rev.UniqueVals))
				} else {
					rollup.MetricRev = float64(rev.Sum)
				}
				rollup.CountRev = rev.Count
				rollup.MinRev = rev.Min
				rollup.MaxRev = rev.Max
				rollup.BytesRev = rev.Bytes
				rollup.PacketsRev = rev.Packets
				rollup.TCPFlagsRev = rev.TCPFlags
			}
		}

		keys = append(keys, rollup)
	}

	// Sort by metric value (descending)
	sort.Sort(byValue(keys))

	// Apply TopK limit
	if r.config.TopK > 0 && len(keys) > r.config.TopK {
		rc <- keys[0:r.config.TopK]
	} else {
		rc <- keys
	}
}

func (r *CacheRollup) shouldDoEmergencyCleanup() bool {
	if !r.config.EmergencyCleanup {
		return false
	}

	// Check memory limit
	if r.config.MaxMemoryMB > 0 {
		maxBytes := int64(r.config.MaxMemoryMB * 1024 * 1024)
		if r.memoryUsage > maxBytes {
			return true
		}
	}

	// Check key count limit
	if r.config.MaxKeys > 0 && len(r.cache) > r.config.MaxKeys {
		return true
	}

	return false
}

func (r *CacheRollup) doEmergencyCleanup() {
	if len(r.cache) == 0 {
		return
	}

	r.Infof("Starting emergency cleanup - current cache size: %d entries, memory: %.2f MB", 
		len(r.cache), float64(r.memoryUsage)/(1024*1024))

	// Calculate age threshold - remove entries older than this
	now := time.Now()
	var ages []time.Duration
	for _, entry := range r.cache {
		ages = append(ages, now.Sub(entry.LastUpdated))
	}

	sort.Slice(ages, func(i, j int) bool {
		return ages[i] > ages[j]
	})

	// Remove oldest entries (top EMERGENCY_AGE_THRESHOLD percentile)
	thresholdIndex := int(float64(len(ages)) * EMERGENCY_AGE_THRESHOLD)
	if thresholdIndex < 1 {
		thresholdIndex = 1
	}

	ageThreshold := ages[thresholdIndex]
	removed := 0

	for key, entry := range r.cache {
		if now.Sub(entry.LastUpdated) >= ageThreshold {
			r.memoryUsage -= r.estimateEntrySize(entry)
			delete(r.cache, key)
			removed++
		}
	}

	r.Infof("Emergency cleanup completed - removed %d entries older than %v, new cache size: %d entries, memory: %.2f MB", 
		removed, ageThreshold, len(r.cache), float64(r.memoryUsage)/(1024*1024))
}

func (r *CacheRollup) estimateEntrySize(entry *CacheEntry) int64 {
	size := CACHE_ENTRY_OVERHEAD
	size += len(entry.Key)
	
	if r.isUnique {
		// Estimate unique values map size
		size += len(entry.UniqueVals) * 20 // rough estimate per string key
		for k := range entry.UniqueVals {
			size += len(k)
		}
	}

	return int64(size)
}

func (r *CacheRollup) GetMemoryStats() (int64, int) {
	r.mux.RLock()
	defer r.mux.RUnlock()
	
	// Also get actual memory stats
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	
	actualMemory := int64(unsafe.Sizeof(r.cache))
	for k, v := range r.cache {
		actualMemory += int64(len(k))
		actualMemory += int64(unsafe.Sizeof(*v))
		if r.isUnique && v.UniqueVals != nil {
			for uk := range v.UniqueVals {
				actualMemory += int64(len(uk))
			}
		}
	}
	
	return actualMemory, len(r.cache)
}
