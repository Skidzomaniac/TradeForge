package main

// StartTestEvent is consumed from orchestrator-events (event=START_TEST).
type StartTestEvent struct {
	Event           string   `json:"event"`
	TestID          string   `json:"test_id"`
	ContestantID    string   `json:"contestant_id"`
	TargetIP        string   `json:"target_ip"`
	TargetPort      int      `json:"target_port"`
	DurationSeconds int      `json:"duration_seconds"`
	BotCount        int      `json:"bot_count"`
	BotPersonas     []string `json:"bot_personas"`
}

// StopTestEvent is published/consumed to stop a test.
type StopTestEvent struct {
	Event  string `json:"event"`
	TestID string `json:"test_id"`
	Reason string `json:"reason"`
}

// ContainerReadyEvent arrives from build-worker.
type ContainerReadyEvent struct {
	Event         string `json:"event"`
	SubmissionID  string `json:"submission_id"`
	ContestantID  string `json:"contestant_id"`
	ContainerIP   string `json:"container_ip"`
	ContainerPort int    `json:"container_port"`
	Status        string `json:"status"`
}

// ContainerCrashedEvent arrives from build-worker's health monitor.
type ContainerCrashedEvent struct {
	Event        string `json:"event"`
	SubmissionID string `json:"submission_id"`
	ContestantID string `json:"contestant_id"`
	Reason       string `json:"reason"`
}

// LatencyWindow is the per-window snapshot used for scoring.
type LatencyWindow struct {
	ContestantID    string  `json:"contestant_id"`
	P50Us           int64   `json:"p50_us"`
	P90Us           int64   `json:"p90_us"`
	P99Us           int64   `json:"p99_us"`
	TPS             float64 `json:"tps"`
	CorrectnessRate float64 `json:"correctness_rate"`
}
