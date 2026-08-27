package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	testsStarted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_tests_started_total",
		Help: "Total tests started.",
	})
	testsCompleted = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_tests_completed_total",
		Help: "Total tests completed.",
	})
	testsFailed = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_tests_failed_total",
		Help: "Total tests failed.",
	})
	orphansRecovered = promauto.NewCounter(prometheus.CounterOpts{
		Name: "orchestrator_orphans_recovered_total",
		Help: "Total orphaned tests recovered after a crash.",
	})
	activeTestsGauge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "orchestrator_active_tests",
		Help: "Locally-owned running tests.",
	})
)
