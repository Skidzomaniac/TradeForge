package handlers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"

	"github.com/trade-eval/submission-api/apierrors"
	"github.com/trade-eval/submission-api/middleware"
	"github.com/trade-eval/submission-api/repository"
	"github.com/trade-eval/submission-api/security"
)

var validLanguages = map[string]bool{"cpp": true, "rust": true, "go": true, "python": true}

// allowedExt reports whether the file extension is valid for the language.
func allowedExt(language, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == ".zip" {
		return true // all languages accept a zip
	}
	if language == "cpp" && ext == ".cpp" {
		return true
	}
	return false
}

type buildJob struct {
	SubmissionID string `json:"submission_id"`
	S3Key        string `json:"s3_key"`
	Language     string `json:"language"`
	ContestantID string `json:"contestant_id"`
}

// CreateSubmission handles POST /v1/submissions.
func (h *Handlers) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	contestant, ok := middleware.ContestantFromContext(r.Context())
	if !ok {
		apierrors.WriteError(w, &apierrors.ErrUnauthorized{Message: "no contestant in context"})
		return
	}

	// STEP 1 - parse multipart form (cap memory; stream the file).
	if err := r.ParseMultipartForm(h.d.MaxUploadBytes); err != nil {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "file", Message: "could not parse multipart form"})
		return
	}

	language := strings.ToLower(r.FormValue("language"))
	if !validLanguages[language] {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "language", Message: "language must be one of cpp, rust, go, python"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "file", Message: "file field is required"})
		return
	}
	defer file.Close()

	if header.Size > h.d.MaxUploadBytes {
		apierrors.WriteJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "File too large"})
		return
	}
	if err := security.ValidateFilename(header.Filename); err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if !allowedExt(language, header.Filename) {
		apierrors.WriteError(w, &apierrors.ErrValidation{Field: "file", Message: "file extension does not match language"})
		return
	}

	// STEP 2 - generate ID.
	subID := "sub_" + uuid.Must(uuid.NewV7()).String()
	s3Key := "submissions/" + contestant.ID + "/" + subID + "/" + filepath.Base(header.Filename)

	// STEP 3 - stream to MinIO.
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if _, err := h.d.Minio.PutObject(r.Context(), h.d.Bucket, s3Key, file, header.Size,
		minio.PutObjectOptions{ContentType: contentType}); err != nil {
		h.d.Logger.Error("minio put failed", "error", err)
		apierrors.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "upload failed"})
		return
	}

	// STEP 4 - insert to Postgres.
	sub := &repository.Submission{
		ID:             subID,
		ContestantID:   contestant.ID,
		ContestantName: contestant.Name,
		Language:       language,
		S3Key:          s3Key,
		Status:         "pending",
	}
	if err := h.d.Submissions.Create(r.Context(), sub); err != nil {
		h.d.Logger.Error("submission insert failed", "error", err)
		apierrors.WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not persist submission"})
		return
	}

	// STEP 5 - publish build job (best effort).
	job := buildJob{SubmissionID: subID, S3Key: s3Key, Language: language, ContestantID: contestant.ID}
	if b, err := json.Marshal(job); err == nil && h.d.Producer != nil {
		if err := h.d.Producer.Produce(r.Context(), h.d.BuildJobsTopic, []byte(subID), b); err != nil {
			h.d.Logger.Warn("kafka publish failed; build-worker fallback will pick it up", "error", err)
		}
	}

	// STEP 6 - return 202.
	apierrors.WriteJSON(w, http.StatusAccepted, map[string]string{
		"submission_id": subID,
		"status":        "pending",
		"message":       "Build queued",
	})
}

// GetSubmission handles GET /v1/submissions/{id}. It returns 404 for a
// submission owned by a different contestant (no existence leak).
func (h *Handlers) GetSubmission(w http.ResponseWriter, r *http.Request) {
	contestant, _ := middleware.ContestantFromContext(r.Context())
	id := chi.URLParam(r, "id")
	sub, err := h.d.Submissions.GetByID(r.Context(), id)
	if err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if contestant == nil || sub.ContestantID != contestant.ID {
		apierrors.WriteError(w, &apierrors.ErrNotFound{Message: "submission not found"})
		return
	}
	apierrors.WriteJSON(w, http.StatusOK, sub)
}

// GetSubmissionLogs handles GET /v1/submissions/{id}/logs.
func (h *Handlers) GetSubmissionLogs(w http.ResponseWriter, r *http.Request) {
	contestant, _ := middleware.ContestantFromContext(r.Context())
	id := chi.URLParam(r, "id")
	sub, err := h.d.Submissions.GetByID(r.Context(), id)
	if err != nil {
		apierrors.WriteError(w, err)
		return
	}
	if contestant == nil || sub.ContestantID != contestant.ID {
		apierrors.WriteError(w, &apierrors.ErrNotFound{Message: "submission not found"})
		return
	}
	logs := ""
	if sub.ErrorLog != nil {
		logs = *sub.ErrorLog
	}
	apierrors.WriteJSON(w, http.StatusOK, map[string]string{"logs": logs})
}
