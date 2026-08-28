package main

// StartTestEvent is consumed from orchestrator-events.
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

// StopTestEvent halts a test.
type StopTestEvent struct {
	Event  string `json:"event"`
	TestID string `json:"test_id"`
	Reason string `json:"reason"`
}

// ContainerCrashedEvent stops all tests for a contestant.
type ContainerCrashedEvent struct {
	Event        string `json:"event"`
	ContestantID string `json:"contestant_id"`
	Reason       string `json:"reason"`
}
