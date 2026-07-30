package flowstitch

import (
	"testing"
	"time"

	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/stretchr/testify/assert"
)

func baseFlow() *kt.JCHF {
	m := kt.NewJCHF()
	m.EventType = kt.KENTIK_EVENT_TYPE
	m.Timestamp = 1761647280
	m.Protocol = "TCP"
	m.SrcAddr = "10.160.55.141"
	m.DstAddr = "10.178.32.52"
	m.L4SrcPort = 53660
	m.L4DstPort = 9997
	m.InBytes = 90
	m.InPkts = 2
	m.OutBytes = 0
	m.OutPkts = 0
	m.TcpFlags = 2
	m.DeviceName = "router1"
	m.InputPort = kt.IfaceID(20)
	m.OutputPort = kt.IfaceID(10)
	m.CustomStr = map[string]string{}
	m.CustomInt = map[string]int32{}
	m.CustomBigInt = map[string]int64{}
	return m
}

func reverseFlow() *kt.JCHF {
	m := baseFlow()
	m.SrcAddr = "10.178.32.52"
	m.DstAddr = "10.160.55.141"
	m.L4SrcPort = 9997
	m.L4DstPort = 53660
	m.InBytes = 52
	m.InPkts = 1
	m.TcpFlags = 16
	return m
}

func TestNoImmediateEmitBeforeTTL(t *testing.T) {
	a := assert.New(t)

	f, err := NewFormat(nil, kt.CompressionNone)
	a.NoError(err)

	res, err := f.To([]*kt.JCHF{baseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.Nil(res)
	a.Equal(1, len(f.cache))

	// Even with reverse direction present, still no immediate output.
	res, err = f.To([]*kt.JCHF{reverseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.Nil(res)
	a.Equal(1, len(f.cache))
}

func TestEmitAfterTTLExpiry(t *testing.T) {
	a := assert.New(t)

	f, err := NewFormat(nil, kt.CompressionNone)
	a.NoError(err)

	_, err = f.To([]*kt.JCHF{baseFlow(), reverseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.Equal(1, len(f.cache))

	// Force expiration without waiting 5 minutes.
	for _, conv := range f.cache {
		conv.lastSeen = time.Now().Add(-6 * time.Minute)
	}

	trigger := baseFlow()
	trigger.Timestamp += 1
	res, err := f.To([]*kt.JCHF{trigger}, make([]byte, 0))
	a.NoError(err)
	a.NotNil(res)

	out, err := f.From(res)
	a.NoError(err)
	a.Equal(1, len(out))

	rec := out[0]
	a.Equal("10.160.55.141", rec["sourceipaddress"])
	a.Equal("10.178.32.52", rec["destinationipaddress"])
	a.Equal("TCP", rec["protocol"])
	a.Equal(int64(9997), int64(rec["wellknown_port"].(float64)))
	a.Equal(true, rec["sourceinitiated"])

	// Source->dest and reverse counters are directional aggregates.
	a.Equal(int64(90), int64(rec["octetdeltacount"].(float64)))
	a.Equal(int64(52), int64(rec["octetdeltacount_rev"].(float64)))
	a.Equal(int64(2), int64(rec["packetdeltacount"].(float64)))
	a.Equal(int64(1), int64(rec["packetdeltacount_rev"].(float64)))

	// Both flows counted in the same conversation.
	a.Equal(int64(1), int64(rec["cnt"].(float64)))
	a.Equal(int64(1), int64(rec["cnt_rev"].(float64)))
	// TCP flags from both directions collapse into a single set.
	a.Equal("SYN,ACK", rec["tcp_flags"])
	_, hasFlagsRev := rec["tcp_flags_rev"]
	a.False(hasFlagsRev)

	// Transport ports are intentionally no longer exported.
	_, hasSrcPort := rec["sourcetransportport"]
	_, hasDstPort := rec["destinationtransportport"]
	a.False(hasSrcPort)
	a.False(hasDstPort)
}

func TestAggregatesAcrossEphemeralPortChanges(t *testing.T) {
	a := assert.New(t)

	f, err := NewFormat(nil, kt.CompressionNone)
	a.NoError(err)

	f1 := baseFlow()
	f2 := baseFlow()
	f2.L4SrcPort = 55001 // different ephemeral source port, same conversation/service
	f2.InBytes = 30
	f2.InPkts = 1

	_, err = f.To([]*kt.JCHF{f1, f2}, make([]byte, 0))
	a.NoError(err)
	a.Equal(1, len(f.cache))

	for _, conv := range f.cache {
		conv.lastSeen = time.Now().Add(-6 * time.Minute)
	}

	res, err := f.To([]*kt.JCHF{reverseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.NotNil(res)

	out, err := f.From(res)
	a.NoError(err)
	a.Equal(1, len(out))

	rec := out[0]
	a.Equal(int64(120), int64(rec["octetdeltacount"].(float64)))
	a.Equal(int64(3), int64(rec["packetdeltacount"].(float64)))
	a.Equal(int64(2), int64(rec["cnt"].(float64)))
	a.Equal(int64(0), int64(rec["cnt_rev"].(float64)))
}

func TestTCPFlagsCollapseIntoSingleSet(t *testing.T) {
	a := assert.New(t)

	f, err := NewFormat(nil, kt.CompressionNone)
	a.NoError(err)

	f1 := baseFlow()
	f1.TcpFlags = 2 // SYN
	f2 := baseFlow()
	f2.TcpFlags = 16 // ACK

	_, err = f.To([]*kt.JCHF{f1, f2}, make([]byte, 0))
	a.NoError(err)

	for _, conv := range f.cache {
		conv.lastSeen = time.Now().Add(-6 * time.Minute)
	}

	res, err := f.To([]*kt.JCHF{reverseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.NotNil(res)

	out, err := f.From(res)
	a.NoError(err)
	a.Equal(1, len(out))

	rec := out[0]
	// SYN + ACK collapse into a single set for the conversation.
	a.Equal("SYN,ACK", rec["tcp_flags"])
	_, hasFlagsRev := rec["tcp_flags_rev"]
	a.False(hasFlagsRev)
	a.Equal(int64(2), int64(rec["cnt"].(float64)))
	a.Equal(int64(0), int64(rec["cnt_rev"].(float64)))
}

func TestGzipRoundTrip(t *testing.T) {
	a := assert.New(t)

	f, err := NewFormat(nil, kt.CompressionGzip)
	a.NoError(err)

	_, err = f.To([]*kt.JCHF{baseFlow()}, make([]byte, 0))
	a.NoError(err)
	for _, conv := range f.cache {
		conv.lastSeen = time.Now().Add(-6 * time.Minute)
	}

	res, err := f.To([]*kt.JCHF{reverseFlow()}, make([]byte, 0))
	a.NoError(err)
	a.NotNil(res)

	out, err := f.From(res)
	a.NoError(err)
	a.Equal(1, len(out))
	a.Equal("TCP", out[0]["protocol"])
	a.Equal(int64(9997), int64(out[0]["wellknown_port"].(float64)))
}
