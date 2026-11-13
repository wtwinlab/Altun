package metrics

import (
	"time"

	"github.com/relab/hotstuff/client"
	"github.com/relab/hotstuff/eventloop"
	"github.com/relab/hotstuff/logging"
	"github.com/relab/hotstuff/metrics/types"
	"github.com/relab/hotstuff/modules"
	"google.golang.org/protobuf/types/known/durationpb"
)

func init() {
	RegisterClientMetric("client-latency", func() any {
		return &ClientLatency{}
	})
}

// ClientLatency processes LatencyMeasurementEvents, and writes LatencyMeasurements to the metrics logger.
type ClientLatency struct {
	metricsLogger Logger
	opts          *modules.Options

	wf Welford
}

// InitModule gives the module access to the other modules.
func (lr *ClientLatency) InitModule(mods *modules.Core) {
	var (
		eventLoop *eventloop.EventLoop
		logger    logging.Logger
	)

	mods.Get(
		&lr.metricsLogger,
		&lr.opts,
		&eventLoop,
		&logger,
	)

	eventLoop.RegisterHandler(client.LatencyMeasurementEvent{}, func(event any) {
		latencyEvent := event.(client.LatencyMeasurementEvent)
		lr.addLatency(latencyEvent.Latency)
	})

	eventLoop.RegisterObserver(types.TickEvent{}, func(event any) {
		lr.tick(event.(types.TickEvent))
	})

	// Timed closed-loop window summary (client.WindowEvent): logged as a client-side
	// ThroughputMeasurement (Commands = completed in window, Commits = sent in window,
	// Duration = window) plus a LatencyMeasurement with the same Event (mean/variance
	// of exactly those commands). Runs inside AddEvent so the event queue cannot drop it.
	eventLoop.RegisterHandler(client.WindowEvent{}, func(event any) {
		lr.logWindow(event.(client.WindowEvent))
	}, eventloop.UnsafeRunInAddEvent())

	logger.Info("Client Latency metric enabled")
}

// AddLatency adds a latency data point to the current measurement.
func (lr *ClientLatency) addLatency(latency time.Duration) {
	millis := float64(latency) / float64(time.Millisecond)
	lr.wf.Update(millis)
}

func (lr *ClientLatency) tick(_ types.TickEvent) {
	mean, variance, count := lr.wf.Get()
	event := &types.LatencyMeasurement{
		Event:    types.NewClientEvent(uint32(lr.opts.ID()), time.Now()),
		Latency:  mean,
		Variance: variance,
		Count:    count,
	}
	lr.metricsLogger.Log(event)
	lr.wf.Reset()
}

func (lr *ClientLatency) logWindow(w client.WindowEvent) {
	ev := types.NewClientEvent(uint32(lr.opts.ID()), time.Now())
	mean, variance := 0.0, 0.0
	if w.Completed > 0 {
		n := float64(w.Completed)
		mean = w.LatencySumMs / n
		if w.Completed > 1 {
			variance = (w.LatencySqSum - n*mean*mean) / (n - 1)
		}
	}
	lr.metricsLogger.Log(&types.ThroughputMeasurement{
		Event:    ev,
		Commits:  w.Sent,
		Commands: w.Completed,
		Duration: durationpb.New(w.Window),
	})
	lr.metricsLogger.Log(&types.LatencyMeasurement{
		Event:    ev,
		Latency:  mean,
		Variance: variance,
		Count:    w.Completed,
	})
}
