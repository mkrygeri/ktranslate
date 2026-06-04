package json

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"strconv"
	"strings"

	"github.com/kentik/ktranslate/pkg/formats/util"
	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/kentik/ktranslate/pkg/rollup"

	"github.com/kentik/ktranslate/pkg/eggs/logger"
)

const (
	InstNameNetflowEvent = "netflow-events"
	InstNameVPCEvent     = "vpc-flow-events"
	InstNameAWSVPCEvent  = "aws-vpc-flow-events"
	InstNameHttpEvent    = "http-events"
)

type JsonFormat struct {
	logger.ContextL
	compression kt.Compression
	doGz        bool
	doFlatten   bool
}

func NewFormat(log logger.Underlying, compression kt.Compression, doFlatten bool) (*JsonFormat, error) {
	jf := &JsonFormat{
		compression: compression,
		ContextL:    logger.NewContextLFromUnderlying(logger.SContext{S: "jsonFormat"}, log),
		doGz:        false,
		doFlatten:   doFlatten,
	}

	switch compression {
	case kt.CompressionNone:
		jf.doGz = false
	case kt.CompressionGzip:
		jf.doGz = true
	default:
		return nil, fmt.Errorf("Invalid compression (%s): format json only supports none|gzip", compression)
	}

	return jf, nil
}

func (f *JsonFormat) To(msgs []*kt.JCHF, serBuf []byte) (*kt.Output, error) {
	var target []byte
	if f.doFlatten {
		msgsNew := make([]map[string]interface{}, 0, len(msgs))
		for _, msg := range msgs {
			if msg.EventType == kt.KENTIK_EVENT_SNMP {
				continue
			}
			mm := msg.Flatten()
			strip(mm)
			msgsNew = append(msgsNew, mm)
		}
		t, err := json.Marshal(msgsNew)
		if err != nil {
			return nil, err
		}
		target = t
	} else {
		t, err := json.Marshal(msgs)
		if err != nil {
			return nil, err
		}
		target = t
	}

	if !f.doGz {
		return kt.NewOutputWithProvider(target, msgs[0].Provider, kt.EventOutput), nil
	}

	buf := bytes.NewBuffer(serBuf)
	buf.Reset()
	zw, err := gzip.NewWriterLevel(buf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}

	_, err = zw.Write(target)
	if err != nil {
		return nil, err
	}

	err = zw.Close()
	if err != nil {
		return nil, err
	}

	return kt.NewOutputWithProvider(buf.Bytes(), msgs[0].Provider, kt.EventOutput), nil
}

func (f *JsonFormat) From(raw *kt.Output) ([]map[string]interface{}, error) {
	msgs := []*kt.JCHF{}
	var err error

	if !f.doGz {
		err = json.Unmarshal(raw.Body, &msgs)
	} else {
		r, err := gzip.NewReader(bytes.NewBuffer(raw.Body))
		if err != nil {
			return nil, err
		}
		err = json.NewDecoder(r).Decode(&msgs)
	}
	if err != nil {
		return nil, err
	}

	values := make([]map[string]interface{}, len(msgs))
	for i, m := range msgs {
		m.SetMap()
		values[i] = m.ToMap()
	}

	return values, err
}

func (f *JsonFormat) Rollup(rolls []rollup.Rollup) (*kt.Output, error) {
	// When flow stitching is enabled, emit a clean RFC-5103 biflow accounting
	// record (named tuple fields + explicit forward/reverse metrics) instead of
	// the raw internal rollup struct.
	var payload interface{} = rolls
	if len(rolls) > 0 && rolls[0].IsBiflow {
		recs := make([]map[string]interface{}, 0, len(rolls))
		for i := range rolls {
			recs = append(recs, toBiflowRecord(&rolls[i]))
		}
		payload = recs
	}

	if !f.doGz {
		res, err := json.Marshal(payload)
		return kt.NewOutputWithProvider(res, rolls[0].Provider, kt.RollupOutput), err
	}

	serBuf := make([]byte, 0)
	buf := bytes.NewBuffer(serBuf)
	buf.Reset()
	zw, err := gzip.NewWriterLevel(buf, gzip.DefaultCompression)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	_, err = zw.Write(b)
	if err != nil {
		return nil, err
	}

	err = zw.Close()
	if err != nil {
		return nil, err
	}

	return kt.NewOutputWithProvider(buf.Bytes(), rolls[0].Provider, kt.RollupOutput), nil
}

// toBiflowRecord renders a stitched rollup as a flat RFC-5103 biflow record:
// the flow tuple is expanded into named dimension fields, and the measured
// metric is reported for both directions (forward and reverse) along with the
// flow counts. Reverse values are always present (0 when no reverse flow was
// seen), so each record is a self-contained bidirectional accounting entry.
func toBiflowRecord(roll *rollup.Rollup) map[string]interface{} {
	rec := make(map[string]interface{})

	dims := roll.GetDims()
	vals := strings.Split(roll.Dimension, roll.KeyJoin)
	for i, d := range dims {
		if i < len(vals) {
			rec[d] = coerceValue(vals[i])
		}
	}

	rec["name"] = roll.Name
	rec["is_biflow"] = true
	rec["bytes_fwd"] = roll.Bytes
	rec["bytes_rev"] = roll.BytesRev
	rec["packets_fwd"] = roll.Packets
	rec["packets_rev"] = roll.PacketsRev
	rec["count_fwd"] = roll.Count
	rec["count_rev"] = roll.CountRev
	rec["tcp_flags_fwd"] = roll.TCPFlags
	rec["tcp_flags_rev"] = roll.TCPFlagsRev

	return rec
}

// coerceValue renders a dimension value as an integer when it is purely numeric
// (e.g. ports, ASNs) and otherwise as the original string (e.g. IP addresses).
func coerceValue(v string) interface{} {
	if v == "" {
		return v
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		return n
	}
	return v
}


func strip(in map[string]interface{}) {
	for k, v := range in {
		switch tv := v.(type) {
		case string:
			if tv == "" || tv == "-" || tv == "--" {
				delete(in, k)
			}
		case int32:
			if tv == 0 {
				delete(in, k)
			}
		case int64:
			if tv == 0 {
				delete(in, k)
			}
		}
		if _, ok := util.DroppedAttrs[k]; ok {
			delete(in, k)
		}
	}
	in["instrumentation.provider"] = kt.InstProvider // Let them know who sent this.
	switch in["provider"] {
	case kt.ProviderVPC:
		switch in["kt.from"] {
		case kt.FromLambda:
			in["instrumentation.name"] = InstNameAWSVPCEvent
		default:
			in["instrumentation.name"] = InstNameVPCEvent
		}
	case kt.ProviderHttpDevice:
		in["instrumentation.name"] = InstNameHttpEvent
	default:
		in["instrumentation.name"] = InstNameNetflowEvent
	}
	in["collector.name"] = kt.CollectorName
}
