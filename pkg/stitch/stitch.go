package stitch

import (
	"flag"
	"fmt"

	go_metrics "github.com/kentik/go-metrics"
	"github.com/kentik/ktranslate"
	"github.com/kentik/ktranslate/pkg/eggs/logger"
	"github.com/kentik/ktranslate/pkg/kt"
	"github.com/kentik/ktranslate/pkg/stitch/ringbuffer"
)

var (
	enable bool
	bufLen int
)

func init() {
	flag.IntVar(&bufLen, "stitch.buffer.len", 10000, "How large of a buffer of flows to try and stitch together.")
	flag.BoolVar(&enable, "stitch.enable", false, "Turn on flow stitching.")
}

type stitchSnapshot struct {
	TcpFlags      uint32
	InBytes       uint64
	InPkts        uint64
	OutBytes      uint64
	OutPkts       uint64
	TcpRetransmit uint32
	Timestamp     int64
}

type Stitcher struct {
	logger.ContextL
	cache    *ringbuffer.RingBuffer[stitchSnapshot]
	registry go_metrics.Registry
	metrics  *StitchMetric
}

type StitchMetric struct {
	FlowsIn      go_metrics.Meter
	FlowsMatched go_metrics.Meter
}

func NewStitcher(log logger.Underlying, cfg *ktranslate.StitchConfig, registry go_metrics.Registry) (*Stitcher, error) {
	if !cfg.Enable {
		return nil, nil
	}

	s := &Stitcher{
		ContextL: logger.NewContextLFromUnderlying(logger.SContext{S: "flowStitch"}, log),
		cache:    ringbuffer.New[stitchSnapshot](cfg.BufLen),
		registry: registry,
		metrics: &StitchMetric{
			FlowsIn:      go_metrics.GetOrRegisterMeter(fmt.Sprintf("stitch.in^force=true"), registry),
			FlowsMatched: go_metrics.GetOrRegisterMeter(fmt.Sprintf("stitch.matched^force=true"), registry),
		},
	}

	s.Infof("Starting flow unification system with a buffer length of %d", cfg.BufLen)
	return s, nil
}

/*
If there's a matching ingress / egress flow, record it here.
*/
func (s *Stitcher) Stitch(msg *kt.JCHF) bool {

	key := msg.GetKey()
	s.metrics.FlowsIn.Mark(1)
	if nm, ok := s.cache.GetAndDelete(key); ok {
		msg.CustomInt[kt.StitchPairTcpFlags] = int32(nm.TcpFlags)
		msg.CustomBigInt[kt.StitchPairInBytes] = int64(nm.InBytes)
		msg.CustomBigInt[kt.StitchPairInPkts] = int64(nm.InPkts)
		msg.CustomBigInt[kt.StitchPairOutBytes] = int64(nm.OutBytes)
		msg.CustomBigInt[kt.StitchPairOutPkts] = int64(nm.OutPkts)
		msg.CustomInt[kt.StitchPairTcpRetransmit] = int32(nm.TcpRetransmit)
		msg.CustomBigInt[kt.StitchPairTimestamp] = int64(nm.Timestamp)
		s.metrics.FlowsMatched.Mark(1)
		return true
	}

	s.cache.Put(key, stitchSnapshot{
		TcpFlags:      msg.TcpFlags,
		InBytes:       msg.InBytes,
		InPkts:        msg.InPkts,
		OutBytes:      msg.OutBytes,
		OutPkts:       msg.OutPkts,
		TcpRetransmit: msg.TcpRetransmit,
		Timestamp:     msg.Timestamp,
	})
	return false
}

// NOOP for now.
func (s *Stitcher) Stop() {}

func (s *Stitcher) HttpInfo() map[string]float64 {
	if s == nil {
		return nil
	}

	return map[string]float64{
		"flows_in":      s.metrics.FlowsIn.Rate1(),
		"flows_matched": s.metrics.FlowsMatched.Rate1(),
	}
}
