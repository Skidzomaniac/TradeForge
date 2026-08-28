// Package metrics exposes Prometheus instrumentation for the bot fleet.
package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all bot-fleet Prometheus collectors.
type Metrics struct {
	OrdersSent     *prometheus.CounterVec
	OrdersCorrect  *prometheus.CounterVec
	OrdersTimedOut *prometheus.CounterVec
	EventsDropped  prometheus.Counter
	EventsSent     prometheus.Counter
	ActiveBots     *prometheus.GaugeVec
	ActiveTests    prometheus.Gauge
	BreakerState   *prometheus.GaugeVec
	OrderLatency   *prometheus.HistogramVec
	reg            *prometheus.Registry
}

// New builds and registers the collectors.
func New() *Metrics {
	reg := prometheus.NewRegistry()
	m := &Metrics{
		reg: reg,
		OrdersSent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bot_fleet_orders_sent_total", Help: "orders sent",
		}, []string{"persona", "order_type", "contestant_id"}),
		OrdersCorrect: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bot_fleet_orders_correct_total", Help: "orders correct",
		}, []string{"persona", "contestant_id"}),
		OrdersTimedOut: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bot_fleet_orders_timedout_total", Help: "orders timed out",
		}, []string{"persona", "contestant_id"}),
		EventsDropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bot_fleet_kafka_events_dropped_total", Help: "telemetry events dropped",
		}),
		EventsSent: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bot_fleet_kafka_events_emitted_total", Help: "telemetry events emitted",
		}),
		ActiveBots: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bot_fleet_active_bots", Help: "active bots",
		}, []string{"test_id", "contestant_id"}),
		ActiveTests: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bot_fleet_active_tests", Help: "active tests",
		}),
		BreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bot_fleet_circuit_breaker_state", Help: "0=closed 1=open 2=half_open",
		}, []string{"contestant_id"}),
		OrderLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "bot_fleet_order_latency_microseconds",
			Help:    "order latency distribution",
			Buckets: []float64{50, 100, 250, 500, 1000, 2500, 5000, 10000, 50000, 100000, 500000, 1000000},
		}, []string{"persona", "contestant_id"}),
	}
	reg.MustRegister(m.OrdersSent, m.OrdersCorrect, m.OrdersTimedOut, m.EventsDropped,
		m.EventsSent, m.ActiveBots, m.ActiveTests, m.BreakerState, m.OrderLatency)
	return m
}

// Handler returns the /metrics HTTP handler.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
