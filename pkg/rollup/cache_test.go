package rollup

import (
	"testing"
	"time"

	"github.com/kentik/ktranslate"
	"github.com/kentik/ktranslate/pkg/eggs/logger"
	lt "github.com/kentik/ktranslate/pkg/eggs/logger/testing"
	"github.com/kentik/ktranslate/pkg/kt"
)

func TestCacheRollup(t *testing.T) {
	// Create test configuration
	cfg := &ktranslate.RollupConfig{
		JoinKey:          "^",
		TopK:             5,
		KeepUndefined:    false,
		MaxMemoryMB:      1, // Small limit for testing
		MaxKeys:          10, // Small limit for testing
		EmergencyCleanup: true,
	}

	// Create test logger
	l := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// Create rollup definition for sum aggregation
	rd := RollupDef{
		Method:     Sum,
		Name:       "test_rollup",
		Metrics:    []string{"bytes"},
		Dimensions: []string{"src_addr"},
	}

	// Create cache rollup
	rollup, err := newCacheRollup(l, rd, cfg, false)
	if err != nil {
		t.Fatalf("Failed to create cache rollup: %v", err)
	}

	// Test data - simulate network flows
	testData := []map[string]interface{}{
		{
			"bytes":       int64(100),
			"src_addr":    "192.168.1.1",
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		{
			"bytes":       int64(200),
			"src_addr":    "192.168.1.1", // Same source
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		{
			"bytes":       int64(150),
			"src_addr":    "192.168.1.2", // Different source
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
	}

	// Add test data
	rollup.Add(testData)

	// Export results
	results := rollup.Export()

	// Verify results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Check aggregation for first source (should be 300 total)
	found1 := false
	found2 := false
	for _, result := range results {
		if result.Dimension == "192.168.1.1" {
			if result.Metric != 300.0 {
				t.Errorf("Expected 300 for 192.168.1.1, got %f", result.Metric)
			}
			if result.Count != 2 {
				t.Errorf("Expected count 2 for 192.168.1.1, got %d", result.Count)
			}
			found1 = true
		}
		if result.Dimension == "192.168.1.2" {
			if result.Metric != 150.0 {
				t.Errorf("Expected 150 for 192.168.1.2, got %f", result.Metric)
			}
			if result.Count != 1 {
				t.Errorf("Expected count 1 for 192.168.1.2, got %d", result.Count)
			}
			found2 = true
		}
	}

	if !found1 {
		t.Error("Results for 192.168.1.1 not found")
	}
	if !found2 {
		t.Error("Results for 192.168.1.2 not found")
	}
}

func TestCacheRollupUnique(t *testing.T) {
	// Create test configuration
	cfg := &ktranslate.RollupConfig{
		JoinKey:          "^",
		TopK:             5,
		KeepUndefined:    false,
		MaxMemoryMB:      1,
		MaxKeys:          10,
		EmergencyCleanup: true,
	}

	// Create test logger
	l := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// Create rollup definition for unique aggregation
	rd := RollupDef{
		Method:     Unique,
		Name:       "test_unique_rollup",
		Metrics:    []string{"dst_addr"},
		Dimensions: []string{"src_addr"},
	}

	// Create unique cache rollup
	rollup, err := newCacheRollup(l, rd, cfg, true)
	if err != nil {
		t.Fatalf("Failed to create unique cache rollup: %v", err)
	}

	// Test data - simulate network flows with different destination addresses
	testData := []map[string]interface{}{
		{
			"dst_addr":    "10.0.0.1",
			"src_addr":    "192.168.1.1",
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		{
			"dst_addr":    "10.0.0.2",
			"src_addr":    "192.168.1.1", // Same source, different destination
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		{
			"dst_addr":    "10.0.0.1",
			"src_addr":    "192.168.1.1", // Duplicate destination
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
	}

	// Add test data
	rollup.Add(testData)

	// Export results
	results := rollup.Export()

	// Verify results
	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	// Check unique count for source (should be 2 unique destinations)
	if results[0].Dimension != "192.168.1.1" {
		t.Errorf("Expected dimension 192.168.1.1, got %s", results[0].Dimension)
	}
	if results[0].Metric != 2.0 {
		t.Errorf("Expected 2 unique destinations, got %f", results[0].Metric)
	}
}

func TestCacheRollupEmergencyCleanup(t *testing.T) {
	// Create test configuration with very small limits
	cfg := &ktranslate.RollupConfig{
		JoinKey:          "^",
		TopK:             100,
		KeepUndefined:    false,
		MaxMemoryMB:      0, // Disable memory limit for this test
		MaxKeys:          5,  // Very small key limit
		EmergencyCleanup: true,
	}

	// Create test logger
	l := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// Create rollup definition
	rd := RollupDef{
		Method:     Sum,
		Name:       "test_cleanup",
		Metrics:    []string{"bytes"},
		Dimensions: []string{"src_addr"},
	}

	// Create cache rollup
	rollup, err := newCacheRollup(l, rd, cfg, false)
	if err != nil {
		t.Fatalf("Failed to create cache rollup: %v", err)
	}

	// Add more entries than the limit to trigger emergency cleanup
	for i := 0; i < 10; i++ {
		testData := []map[string]interface{}{
			{
				"bytes":       int64(100),
				"src_addr":    string(rune('A' + i)), // Generate different source addresses
				"sample_rate": int64(1),
				"provider":    kt.Provider("pp"),
			},
		}
		rollup.Add(testData)
		
		// Add some delay to create age differences
		time.Sleep(1 * time.Millisecond)
	}

	// Check that emergency cleanup occurred
	rollup.mux.RLock()
	cacheSize := len(rollup.cache)
	rollup.mux.RUnlock()

	if cacheSize > cfg.MaxKeys {
		t.Errorf("Cache size %d exceeds limit %d - emergency cleanup should have occurred", cacheSize, cfg.MaxKeys)
	}

	t.Logf("Cache size after emergency cleanup: %d (limit: %d)", cacheSize, cfg.MaxKeys)
}

func TestCacheRollupStitchFlows(t *testing.T) {
	cfg := &ktranslate.RollupConfig{
		JoinKey:     "^",
		TopK:        100,
		StitchFlows: true,
	}

	l := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// A reversible flow tuple: src/dst addresses and ports.
	rd := RollupDef{
		Method:     Sum,
		Name:       "test_stitch",
		Metrics:    []string{"bytes"},
		Dimensions: []string{"src_addr", "dst_addr", "l4_src_port", "l4_dst_port"},
	}

	rollup, err := newCacheRollup(l, rd, cfg, false)
	if err != nil {
		t.Fatalf("Failed to create cache rollup: %v", err)
	}

	if !rollup.stitchFlows {
		t.Fatal("Expected stitchFlows to be enabled for a reversible flow tuple")
	}

	testData := []map[string]interface{}{
		// Forward flow A->B (initiator).
		{
			"bytes":       int64(1000),
			"src_addr":    "10.0.0.1",
			"dst_addr":    "10.0.0.2",
			"l4_src_port": int64(12345),
			"l4_dst_port": int64(80),
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		// Reverse flow B->A (responder).
		{
			"bytes":       int64(4000),
			"src_addr":    "10.0.0.2",
			"dst_addr":    "10.0.0.1",
			"l4_src_port": int64(80),
			"l4_dst_port": int64(12345),
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
		// A one-directional flow with no reverse counterpart.
		{
			"bytes":       int64(500),
			"src_addr":    "10.0.0.3",
			"dst_addr":    "10.0.0.4",
			"l4_src_port": int64(5555),
			"l4_dst_port": int64(53),
			"sample_rate": int64(1),
			"provider":    kt.Provider("pp"),
		},
	}

	rollup.Add(testData)
	results := rollup.Export()

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	fwdKey := "10.0.0.1^10.0.0.2^12345^80"
	revKey := "10.0.0.2^10.0.0.1^80^12345"
	loneKey := "10.0.0.3^10.0.0.4^5555^53"

	for _, result := range results {
		if !result.IsBiflow {
			t.Errorf("Expected IsBiflow=true for %s", result.Dimension)
		}
		switch result.Dimension {
		case fwdKey:
			if result.Metric != 1000.0 {
				t.Errorf("Forward metric: expected 1000, got %f", result.Metric)
			}
			if result.MetricRev != 4000.0 {
				t.Errorf("Forward reverse metric: expected 4000, got %f", result.MetricRev)
			}
			if result.CountRev != 1 {
				t.Errorf("Forward reverse count: expected 1, got %d", result.CountRev)
			}
		case revKey:
			if result.Metric != 4000.0 {
				t.Errorf("Reverse metric: expected 4000, got %f", result.Metric)
			}
			if result.MetricRev != 1000.0 {
				t.Errorf("Reverse reverse metric: expected 1000, got %f", result.MetricRev)
			}
		case loneKey:
			if result.Metric != 500.0 {
				t.Errorf("Lone metric: expected 500, got %f", result.Metric)
			}
			if result.MetricRev != 0.0 {
				t.Errorf("Lone reverse metric: expected 0 (no reverse flow), got %f", result.MetricRev)
			}
			if result.CountRev != 0 {
				t.Errorf("Lone reverse count: expected 0, got %d", result.CountRev)
			}
		default:
			t.Errorf("Unexpected result dimension: %s", result.Dimension)
		}
	}
}

func TestCacheRollupStitchFlowsNotViable(t *testing.T) {
	cfg := &ktranslate.RollupConfig{
		JoinKey:     "^",
		TopK:        100,
		StitchFlows: true,
	}

	l := lt.NewTestContextL(logger.NilContext, t).GetLogger().GetUnderlyingLogger()

	// src_addr has no dst_addr counterpart -> not a reversible tuple.
	rd := RollupDef{
		Method:     Sum,
		Name:       "test_stitch_bad",
		Metrics:    []string{"bytes"},
		Dimensions: []string{"src_addr", "protocol"},
	}

	rollup, err := newCacheRollup(l, rd, cfg, false)
	if err != nil {
		t.Fatalf("Failed to create cache rollup: %v", err)
	}

	if rollup.stitchFlows {
		t.Fatal("Expected stitchFlows to be disabled when dimensions are not reversible")
	}
}
