package main

// BuildJob is the JSON message consumed from the build-jobs topic.
type BuildJob struct {
	SubmissionID string `json:"submission_id"`
	S3Key        string `json:"s3_key"`
	Language     string `json:"language"`
	ContestantID string `json:"contestant_id"`
}

// ContainerReadyEvent is published to orchestrator-events when a sandbox is up.
type ContainerReadyEvent struct {
	Event         string `json:"event"`
	SubmissionID  string `json:"submission_id"`
	ContestantID  string `json:"contestant_id"`
	ContainerIP   string `json:"container_ip"`
	ContainerPort int    `json:"container_port"`
	Status        string `json:"status"`
}

// ContainerCrashedEvent is published when a sandbox dies unexpectedly.
type ContainerCrashedEvent struct {
	Event        string `json:"event"`
	SubmissionID string `json:"submission_id"`
	ContestantID string `json:"contestant_id"`
	Reason       string `json:"reason"`
}
