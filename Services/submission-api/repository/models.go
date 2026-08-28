// Package repository holds the data-access layer for submission-api.
package repository

import "time"

// Submission mirrors the submissions table.
type Submission struct {
	ID             string    `json:"id" db:"id"`
	ContestantID   string    `json:"contestant_id" db:"contestant_id"`
	ContestantName string    `json:"contestant_name" db:"contestant_name"`
	Language       string    `json:"language" db:"language"`
	S3Key          string    `json:"s3_key" db:"s3_key"`
	Status         string    `json:"status" db:"status"`
	ContainerIP    *string   `json:"container_ip,omitempty" db:"container_ip"`
	ContainerPort  *int      `json:"container_port,omitempty" db:"container_port"`
	ContainerID    *string   `json:"container_id,omitempty" db:"container_id"`
	ErrorLog       *string   `json:"error_log,omitempty" db:"error_log"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Test mirrors the tests table.
type Test struct {
	ID                     string     `json:"id" db:"id"`
	SubmissionID           string     `json:"submission_id" db:"submission_id"`
	ContestantID           string     `json:"contestant_id" db:"contestant_id"`
	Status                 string     `json:"status" db:"status"`
	StartedAt              *time.Time `json:"started_at,omitempty" db:"started_at"`
	EndedAt                *time.Time `json:"ended_at,omitempty" db:"ended_at"`
	FinalScore             *float64   `json:"final_score,omitempty" db:"final_score"`
	FailureReason          *string    `json:"failure_reason,omitempty" db:"failure_reason"`
	OrchestratorInstanceID *string    `json:"orchestrator_instance_id,omitempty" db:"orchestrator_instance_id"`
	LastHeartbeatAt        *time.Time `json:"last_heartbeat_at,omitempty" db:"last_heartbeat_at"`
	CreatedAt              time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at" db:"updated_at"`
}

// Contestant mirrors the contestants table. The API key is stored only as a
// SHA-256 hash; the plaintext key never touches the database and is not exposed
// in responses.
type Contestant struct {
	ID         string    `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	Email      string    `json:"email" db:"email"`
	APIKeyHash string    `json:"-" db:"api_key_hash"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
