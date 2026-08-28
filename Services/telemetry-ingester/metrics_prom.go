package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	eventsProcessed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_events_processed_total",
		Help: "Telemetry events processed through the pipeline.",
	})
	eventsCorrect = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telemetry_events_correct_total",
		Help: "Telemetry events validated correct.",
	})
	consumerLag = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telemetry_kafka_consumer_lag",
		Help: "Most recent measured Kafka consumer-group lag.",
	})
)
