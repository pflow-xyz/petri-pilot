// Blob store for vet clinic attachments

package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pflow-xyz/petri-pilot/pkg/runtime/api"
)

// Blob represents a stored blob with metadata.
type Blob struct {
	ID             string            `json:"id"`
	StreamID       string            `json:"streamId"`
	OwnerID        string            `json:"ownerId"`
	Filename       string            `json:"filename"`
	ContentType    string            `json:"contentType"`
	Size           int64             `json:"size"`
	Data           []byte            `json:"-"`
	AttachmentType string            `json:"attachmentType,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// BlobStore manages blob storage in SQLite.
type BlobStore struct {
	db           *sql.DB
	maxSize      int64
	allowedTypes []string
}

// NewBlobStore creates a new BlobStore instance.
func NewBlobStore(db *sql.DB, maxSize int64, allowedTypes []string) *BlobStore {
	return &BlobStore{
		db:           db,
		maxSize:      maxSize,
		allowedTypes: allowedTypes,
	}
}

// InitSchema creates the blobs table if it doesn't exist.
func (bs *BlobStore) InitSchema() error {
	_, err := bs.db.Exec(`
		CREATE TABLE IF NOT EXISTS blobs (
			id TEXT PRIMARY KEY,
			stream_id TEXT NOT NULL,
			owner_id TEXT NOT NULL,
			filename TEXT NOT NULL,
			content_type TEXT NOT NULL,
			size INTEGER NOT NULL,
			data BLOB NOT NULL,
			attachment_type TEXT,
			created_at TEXT NOT NULL,
			metadata TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_blobs_stream ON blobs(stream_id);
		CREATE INDEX IF NOT EXISTS idx_blobs_owner ON blobs(owner_id);
		CREATE INDEX IF NOT EXISTS idx_blobs_type ON blobs(attachment_type);
		CREATE INDEX IF NOT EXISTS idx_blobs_created_at ON blobs(created_at);
	`)
	return err
}

// generateBlobID creates a unique blob ID.
func generateBlobID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return "blob_" + hex.EncodeToString(b)
}

// isTypeAllowed checks if a content type is allowed.
func (bs *BlobStore) isTypeAllowed(contentType string) bool {
	for _, allowed := range bs.allowedTypes {
		if allowed == "*/*" {
			return true
		}
		if allowed == contentType {
			return true
		}
		// Handle wildcard patterns like "image/*"
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "/*")
			if strings.HasPrefix(contentType, prefix+"/") {
				return true
			}
		}
	}
	return false
}

// Upload stores a new blob and returns its metadata.
func (bs *BlobStore) Upload(streamID, ownerID, filename, contentType, attachmentType string, data []byte, metadata map[string]string) (*Blob, error) {
	if int64(len(data)) > bs.maxSize {
		return nil, fmt.Errorf("blob size %d exceeds maximum allowed size %d", len(data), bs.maxSize)
	}

	if !bs.isTypeAllowed(contentType) {
		return nil, fmt.Errorf("content type %s is not allowed", contentType)
	}

	id := generateBlobID()
	now := time.Now().UTC()

	var metadataJSON []byte
	if metadata != nil {
		var err error
		metadataJSON, err = json.Marshal(metadata)
		if err != nil {
			return nil, fmt.Errorf("marshaling metadata: %w", err)
		}
	}

	_, err := bs.db.Exec(`
		INSERT INTO blobs (id, stream_id, owner_id, filename, content_type, size, data, attachment_type, created_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, streamID, ownerID, filename, contentType, len(data), data, attachmentType, now.Format(time.RFC3339), metadataJSON)
	if err != nil {
		return nil, fmt.Errorf("inserting blob: %w", err)
	}

	return &Blob{
		ID:             id,
		StreamID:       streamID,
		OwnerID:        ownerID,
		Filename:       filename,
		ContentType:    contentType,
		Size:           int64(len(data)),
		AttachmentType: attachmentType,
		CreatedAt:      now,
		Metadata:       metadata,
	}, nil
}

// Get retrieves a blob by ID including its data.
func (bs *BlobStore) Get(id string) (*Blob, error) {
	var blob Blob
	var createdAt string
	var metadataJSON, attachmentType sql.NullString

	err := bs.db.QueryRow(`
		SELECT id, stream_id, owner_id, filename, content_type, size, data, attachment_type, created_at, metadata
		FROM blobs WHERE id = ?
	`, id).Scan(&blob.ID, &blob.StreamID, &blob.OwnerID, &blob.Filename, &blob.ContentType, &blob.Size, &blob.Data, &attachmentType, &createdAt, &metadataJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("blob not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying blob: %w", err)
	}

	blob.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if attachmentType.Valid {
		blob.AttachmentType = attachmentType.String
	}
	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &blob.Metadata)
	}

	return &blob, nil
}

// GetMeta retrieves blob metadata without the data.
func (bs *BlobStore) GetMeta(id string) (*Blob, error) {
	var blob Blob
	var createdAt string
	var metadataJSON, attachmentType sql.NullString

	err := bs.db.QueryRow(`
		SELECT id, stream_id, owner_id, filename, content_type, size, attachment_type, created_at, metadata
		FROM blobs WHERE id = ?
	`, id).Scan(&blob.ID, &blob.StreamID, &blob.OwnerID, &blob.Filename, &blob.ContentType, &blob.Size, &attachmentType, &createdAt, &metadataJSON)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("blob not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("querying blob: %w", err)
	}

	blob.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if attachmentType.Valid {
		blob.AttachmentType = attachmentType.String
	}
	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &blob.Metadata)
	}

	return &blob, nil
}

// ListByStream returns all blobs for a workflow instance (appointment).
func (bs *BlobStore) ListByStream(streamID string) ([]*Blob, error) {
	rows, err := bs.db.Query(`
		SELECT id, stream_id, owner_id, filename, content_type, size, attachment_type, created_at, metadata
		FROM blobs WHERE stream_id = ?
		ORDER BY created_at DESC
	`, streamID)
	if err != nil {
		return nil, fmt.Errorf("querying blobs: %w", err)
	}
	defer rows.Close()

	var blobs []*Blob
	for rows.Next() {
		var blob Blob
		var createdAt string
		var metadataJSON, attachmentType sql.NullString

		if err := rows.Scan(&blob.ID, &blob.StreamID, &blob.OwnerID, &blob.Filename, &blob.ContentType, &blob.Size, &attachmentType, &createdAt, &metadataJSON); err != nil {
			return nil, fmt.Errorf("scanning blob: %w", err)
		}

		blob.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		if attachmentType.Valid {
			blob.AttachmentType = attachmentType.String
		}
		if metadataJSON.Valid {
			json.Unmarshal([]byte(metadataJSON.String), &blob.Metadata)
		}

		blobs = append(blobs, &blob)
	}

	return blobs, nil
}

// Delete removes a blob.
func (bs *BlobStore) Delete(id, userID string, isAdmin bool) error {
	// Get current blob to verify ownership
	blob, err := bs.GetMeta(id)
	if err != nil {
		return err
	}

	if !isAdmin && blob.OwnerID != userID {
		return fmt.Errorf("not authorized to delete this blob")
	}

	_, err = bs.db.Exec(`DELETE FROM blobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deleting blob: %w", err)
	}

	return nil
}

// HTTP Handlers

// HandleBlobUpload handles blob upload requests.
func HandleBlobUpload(bs *BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get user ID from context
		userID := getBlobUserID(r)
		if userID == "" {
			api.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		// Parse multipart form (max 10MB)
		if err := r.ParseMultipartForm(bs.maxSize); err != nil {
			api.Error(w, http.StatusBadRequest, "PARSE_ERROR", "failed to parse multipart form: "+err.Error())
			return
		}

		// Get stream ID (appointment ID)
		streamID := r.FormValue("stream_id")
		if streamID == "" {
			api.Error(w, http.StatusBadRequest, "MISSING_STREAM_ID", "stream_id is required")
			return
		}

		// Get attachment type
		attachmentType := r.FormValue("attachment_type")

		// Get the file
		file, header, err := r.FormFile("file")
		if err != nil {
			api.Error(w, http.StatusBadRequest, "MISSING_FILE", "file is required")
			return
		}
		defer file.Close()

		// Read file content
		data, err := io.ReadAll(io.LimitReader(file, bs.maxSize+1))
		if err != nil {
			api.Error(w, http.StatusBadRequest, "READ_ERROR", "failed to read file")
			return
		}

		if int64(len(data)) > bs.maxSize {
			api.Error(w, http.StatusRequestEntityTooLarge, "TOO_LARGE", fmt.Sprintf("file exceeds maximum size of %d bytes", bs.maxSize))
			return
		}

		contentType := header.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}

		// Extract metadata from form values with "meta_" prefix
		var metadata map[string]string
		for key := range r.Form {
			if strings.HasPrefix(key, "meta_") {
				if metadata == nil {
					metadata = make(map[string]string)
				}
				metaKey := strings.TrimPrefix(key, "meta_")
				metadata[metaKey] = r.FormValue(key)
			}
		}

		blob, err := bs.Upload(streamID, userID, header.Filename, contentType, attachmentType, data, metadata)
		if err != nil {
			api.Error(w, http.StatusBadRequest, "UPLOAD_FAILED", err.Error())
			return
		}

		api.JSON(w, http.StatusCreated, blob)
	}
}

// HandleBlobDownload handles blob retrieval requests.
func HandleBlobDownload(bs *BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			api.Error(w, http.StatusBadRequest, "MISSING_ID", "blob ID is required")
			return
		}

		blob, err := bs.Get(id)
		if err != nil {
			api.Error(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		w.Header().Set("Content-Type", blob.ContentType)
		w.Header().Set("Content-Length", fmt.Sprintf("%d", blob.Size))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, blob.Filename))
		w.Header().Set("X-Blob-Owner", blob.OwnerID)
		w.WriteHeader(http.StatusOK)
		w.Write(blob.Data)
	}
}

// HandleBlobMeta handles blob metadata requests.
func HandleBlobMeta(bs *BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			api.Error(w, http.StatusBadRequest, "MISSING_ID", "blob ID is required")
			return
		}

		blob, err := bs.GetMeta(id)
		if err != nil {
			api.Error(w, http.StatusNotFound, "NOT_FOUND", err.Error())
			return
		}

		api.JSON(w, http.StatusOK, blob)
	}
}

// HandleListAttachments handles listing attachments for an appointment.
func HandleListAttachments(bs *BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		streamID := r.PathValue("id")
		if streamID == "" {
			api.Error(w, http.StatusBadRequest, "MISSING_ID", "appointment ID is required")
			return
		}

		blobs, err := bs.ListByStream(streamID)
		if err != nil {
			api.Error(w, http.StatusInternalServerError, "LIST_FAILED", err.Error())
			return
		}

		api.JSON(w, http.StatusOK, map[string]interface{}{
			"attachments": blobs,
		})
	}
}

// HandleBlobDelete handles blob deletion requests.
func HandleBlobDelete(bs *BlobStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			api.Error(w, http.StatusBadRequest, "MISSING_ID", "blob ID is required")
			return
		}

		userID := getBlobUserID(r)
		if userID == "" {
			api.Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
			return
		}

		if err := bs.Delete(id, userID, isBlobAdmin(r)); err != nil {
			api.Error(w, http.StatusForbidden, "DELETE_FAILED", err.Error())
			return
		}

		api.JSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
		})
	}
}

// Helper functions to extract user info from context

func getBlobUserID(r *http.Request) string {
	user := UserFromContext(r.Context())
	if user == nil {
		return ""
	}
	return fmt.Sprintf("%d", user.ID)
}

func isBlobAdmin(r *http.Request) bool {
	user := UserFromContext(r.Context())
	if user == nil {
		return false
	}
	for _, role := range user.Roles {
		if role == "office_manager" {
			return true
		}
	}
	return false
}
