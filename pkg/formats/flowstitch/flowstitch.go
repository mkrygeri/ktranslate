package flowstitch

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strings"
	"time"

	"github.com/kentik/ktranslate/pkg/eggs/logger"
	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/kentik/ktranslate/pkg/rollup"
)

// conversationKey identifies a conversation independently of ephemeral ports.
// Key dimensions: unordered ip pair + protocol + well-known port + device.
type conversationKey string

// directionalSnapshot aggregates one direction within a conversation.
type directionalSnapshot struct {
	bytes uint64
	pkts  uint64
	count int
}

// cachedConversation stores the rolling 5-minute aggregate.
type cachedConversation struct {
	sourceIP      string
	destinationIP string
	protocol      string
	wellKnownPort uint32
	deviceName    string
	sourceIface   int64
	destIface     int64

	srcToDst directionalSnapshot
	dstToSrc directionalSnapshot

	// tcpFlags is the union of TCP flags seen in both directions of the
	// conversation, collapsed into a single set.
	tcpFlags uint32

	totalCount          int
	sourceInitiatedHits int
	destInitiatedHits   int

	startTimestamp int64
	endTimestamp   int64
	lastSeen       time.Time
}

// FlowStitchFormat emits one record per conversation after 5 minutes of
// inactivity. It intentionally does not emit immediately on stitched matches.
type FlowStitchFormat struct {
	logger.ContextL
	compression kt.Compression
	doGz        bool
	cache       map[conversationKey]*cachedConversation
	ttl         time.Duration
}

// stitchRecord is the vendor-facing output schema.
type stitchRecord struct {
	StartTimestamp       int64  `json:"starttimestamp"`
	EndTimestamp         int64  `json:"endtimestamp"`
	SourceIPAddress      string `json:"sourceipaddress"`
	DestinationIPAddress string `json:"destinationipaddress"`
	SourceInitiated      bool   `json:"sourceinitiated"`
	Protocol             string `json:"protocol"`
	WellKnownPort        uint32 `json:"wellknown_port"`
	TCPFlags             string `json:"tcp_flags"`
	OctetDeltaCount      uint64 `json:"octetdeltacount"`
	OctetDeltaCountRev   uint64 `json:"octetdeltacount_rev"`
	PacketDeltaCount     uint64 `json:"packetdeltacount"`
	PacketDeltaCountRev  uint64 `json:"packetdeltacount_rev"`
	MaxDuration          int64  `json:"maxduration"`
	Cnt                  int    `json:"cnt"`
	CntRev               int    `json:"cnt_rev"`
	DeviceName           string `json:"devicename"`
	SourceInterface      int64  `json:"sourceinterface"`
	DestinationInterface int64  `json:"destinationinterface"`
}

func NewFormat(log logger.Underlying, compression kt.Compression) (*FlowStitchFormat, error) {
	f := &FlowStitchFormat{
		compression: compression,
		ContextL:    logger.NewContextLFromUnderlying(logger.SContext{S: "flowStitchFormat"}, log),
		doGz:        false,
		cache:       make(map[conversationKey]*cachedConversation),
		ttl:         5 * time.Minute,
	}

	switch compression {
	case kt.CompressionNone:
		f.doGz = false
	case kt.CompressionGzip:
		f.doGz = true
	default:
		return nil, fmt.Errorf("invalid compression (%s): format flow_stitch only supports none|gzip", compression)
	}

	return f, nil
}

func (f *FlowStitchFormat) To(msgs []*kt.JCHF, serBuf []byte) (*kt.Output, error) {
	buf := bytes.NewBuffer(serBuf[:0])

	// First emit all expired conversations; no non-expired record is emitted.
	expired, err := f.collectExpired(time.Now())
	if err != nil {
		return nil, err
	}
	for _, line := range expired {
		if buf.Len() > 0 {
			buf.WriteByte('\n')
		}
		buf.Write(line)
	}

	// Aggregate current batch into the cache only.
	now := time.Now()
	for _, msg := range msgs {
		if msg.EventType == kt.KENTIK_EVENT_SNMP {
			continue
		}
		f.aggregate(msg, now)
	}

	if buf.Len() == 0 {
		return nil, nil
	}

	if !f.doGz {
		out := make([]byte, buf.Len())
		copy(out, buf.Bytes())
		return kt.NewOutputWithProvider(out, msgs[0].Provider, kt.EventOutput), nil
	}

	gzBuf := bytes.NewBuffer(make([]byte, 0, buf.Len()))
	zw, err := gzip.NewWriterLevel(gzBuf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err = zw.Write(buf.Bytes()); err != nil {
		return nil, err
	}
	if err = zw.Close(); err != nil {
		return nil, err
	}

	return kt.NewOutputWithProvider(gzBuf.Bytes(), msgs[0].Provider, kt.EventOutput), nil
}

func (f *FlowStitchFormat) aggregate(msg *kt.JCHF, now time.Time) {
	src := normalizeIP(msg.SrcAddr)
	dst := normalizeIP(msg.DstAddr)
	protocol := msg.Protocol
	wellKnown := pickWellKnownPort(msg.L4SrcPort, msg.L4DstPort)

	key, canonicalSrc, canonicalDst := makeConversationKey(src, dst, protocol, wellKnown, msg.DeviceName)
	conv, ok := f.cache[key]
	if !ok {
		conv = &cachedConversation{
			sourceIP:       canonicalSrc,
			destinationIP:  canonicalDst,
			protocol:       protocol,
			wellKnownPort:  wellKnown,
			deviceName:     msg.DeviceName,
			sourceIface:    int64(msg.InputPort),
			destIface:      int64(msg.OutputPort),
			startTimestamp: msg.Timestamp,
			endTimestamp:   msg.Timestamp,
			lastSeen:       now,
		}
		f.cache[key] = conv
	}

	if msg.Timestamp < conv.startTimestamp {
		conv.startTimestamp = msg.Timestamp
	}
	if msg.Timestamp > conv.endTimestamp {
		conv.endTimestamp = msg.Timestamp
	}
	conv.lastSeen = now

	bytesTotal := msg.InBytes + msg.OutBytes
	pktsTotal := msg.InPkts + msg.OutPkts

	// TCP flags collapse into a single set for the whole conversation,
	// regardless of direction.
	conv.tcpFlags |= msg.TcpFlags

	if src == conv.sourceIP && dst == conv.destinationIP {
		conv.srcToDst.bytes += bytesTotal
		conv.srcToDst.pkts += pktsTotal
		conv.srcToDst.count++
	} else {
		conv.dstToSrc.bytes += bytesTotal
		conv.dstToSrc.pkts += pktsTotal
		conv.dstToSrc.count++
	}
	conv.totalCount++

	if sourceInitiatedForMessage(src, dst, msg.L4SrcPort, msg.L4DstPort, conv.sourceIP) {
		conv.sourceInitiatedHits++
	} else {
		conv.destInitiatedHits++
	}
}

func (f *FlowStitchFormat) collectExpired(now time.Time) ([][]byte, error) {
	lines := make([][]byte, 0)
	for key, conv := range f.cache {
		if now.Sub(conv.lastSeen) < f.ttl {
			continue
		}
		rec := buildRecord(conv)
		line, err := json.Marshal(rec)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
		delete(f.cache, key)
	}
	return lines, nil
}

func buildRecord(conv *cachedConversation) stitchRecord {
	maxDuration := conv.endTimestamp - conv.startTimestamp
	if maxDuration < 0 {
		maxDuration = 0
	}

	return stitchRecord{
		StartTimestamp:       conv.startTimestamp,
		EndTimestamp:         conv.endTimestamp,
		SourceIPAddress:      conv.sourceIP,
		DestinationIPAddress: conv.destinationIP,
		SourceInitiated:      conv.sourceInitiatedHits >= conv.destInitiatedHits,
		Protocol:             conv.protocol,
		WellKnownPort:        conv.wellKnownPort,
		TCPFlags:             tcpFlagsToString(conv.tcpFlags),
		OctetDeltaCount:      conv.srcToDst.bytes,
		OctetDeltaCountRev:   conv.dstToSrc.bytes,
		PacketDeltaCount:     conv.srcToDst.pkts,
		PacketDeltaCountRev:  conv.dstToSrc.pkts,
		MaxDuration:          maxDuration,
		Cnt:                  conv.srcToDst.count,
		CntRev:               conv.dstToSrc.count,
		DeviceName:           conv.deviceName,
		SourceInterface:      conv.sourceIface,
		DestinationInterface: conv.destIface,
	}
}

func sourceInitiatedForMessage(msgSrc, msgDst string, srcPort, dstPort uint32, canonicalSource string) bool {
	msgSrcInitiated := isInitiator(msgSrc, msgDst, srcPort, dstPort)
	if msgSrc == canonicalSource {
		return msgSrcInitiated
	}
	if msgDst == canonicalSource {
		return !msgSrcInitiated
	}
	return false
}

// isInitiator determines whether msg src initiated the flow.
func isInitiator(_ string, _ string, srcPort, dstPort uint32) bool {
	srcWellKnown := isWellKnownPort(srcPort)
	dstWellKnown := isWellKnownPort(dstPort)
	if srcWellKnown && !dstWellKnown {
		return false
	}
	if !srcWellKnown && dstWellKnown {
		return true
	}
	return srcPort > dstPort
}

func isWellKnownPort(port uint32) bool {
	return port <= 49151
}

func pickWellKnownPort(srcPort, dstPort uint32) uint32 {
	srcWellKnown := isWellKnownPort(srcPort)
	dstWellKnown := isWellKnownPort(dstPort)
	if srcWellKnown && !dstWellKnown {
		return srcPort
	}
	if !srcWellKnown && dstWellKnown {
		return dstPort
	}
	if srcPort < dstPort {
		return srcPort
	}
	return dstPort
}

func makeConversationKey(src, dst, protocol string, wellKnownPort uint32, device string) (conversationKey, string, string) {
	canonicalSrc := src
	canonicalDst := dst
	if canonicalDst < canonicalSrc {
		canonicalSrc, canonicalDst = canonicalDst, canonicalSrc
	}
	key := conversationKey(fmt.Sprintf("%s|%s|%s|%d|%s", canonicalSrc, canonicalDst, protocol, wellKnownPort, device))
	return key, canonicalSrc, canonicalDst
}

func tcpFlagsToString(flags uint32) string {
	if flags == 0 {
		return ""
	}
	names := make([]string, 0, 8)
	if flags&1 != 0 {
		names = append(names, "FIN")
	}
	if flags&2 != 0 {
		names = append(names, "SYN")
	}
	if flags&4 != 0 {
		names = append(names, "RST")
	}
	if flags&8 != 0 {
		names = append(names, "PSH")
	}
	if flags&16 != 0 {
		names = append(names, "ACK")
	}
	if flags&32 != 0 {
		names = append(names, "URG")
	}
	if flags&64 != 0 {
		names = append(names, "ECE")
	}
	if flags&128 != 0 {
		names = append(names, "CWR")
	}
	return strings.Join(names, ",")
}

func normalizeIP(addr string) string {
	if addr == "" || addr == "<nil>" {
		return ""
	}
	return addr
}

func (f *FlowStitchFormat) From(raw *kt.Output) ([]map[string]interface{}, error) {
	values := make([]map[string]interface{}, 0)

	body := raw.Body
	if f.doGz {
		r, err := gzip.NewReader(bytes.NewBuffer(raw.Body))
		if err != nil {
			return nil, err
		}
		var out bytes.Buffer
		if _, err := out.ReadFrom(r); err != nil {
			return nil, err
		}
		body = out.Bytes()
	}

	for _, line := range bytes.Split(body, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		m := map[string]interface{}{}
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, err
		}
		values = append(values, m)
	}

	return values, nil
}

func (f *FlowStitchFormat) Rollup(rolls []rollup.Rollup) (*kt.Output, error) {
	return nil, nil
}
