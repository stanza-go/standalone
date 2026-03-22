// Package adminuploads provides admin endpoints for file upload management.
// Files are stored at {DATA_DIR}/uploads/YYYY/MM/DD/{UUID}/filename with
// automatic thumbnail generation for images.
package adminuploads

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif" // Register GIF decoder for image.Decode.
	"image/jpeg"
	_ "image/png" // Register PNG decoder for image.Decode.
	"io"
	nethttp "net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminaudit"
)

const (
	maxUploadSize = 50 << 20 // 50 MB
	thumbMaxDim   = 300      // Max width or height for thumbnails.
	thumbFilename = "thumbnail.jpg"
)

// Register mounts the upload management routes on the given admin group.
// The group should already have auth middleware applied.
// Routes:
//
//	GET    /api/admin/uploads            — list uploads with pagination and filters
//	POST   /api/admin/uploads            — upload a file
//	GET    /api/admin/uploads/{id}       — get upload metadata
//	DELETE /api/admin/uploads/{id}       — soft-delete an upload
//	GET    /api/admin/uploads/{id}/file  — serve the original file
//	GET    /api/admin/uploads/{id}/thumb — serve the thumbnail (images only)
func Register(admin *http.Group, db *sqlite.DB, uploadsDir string) {
	admin.HandleFunc("GET /uploads", listHandler(db))
	admin.HandleFunc("GET /uploads/export", exportHandler(db))
	admin.HandleFunc("POST /uploads", uploadHandler(db, uploadsDir))
	admin.HandleFunc("POST /uploads/bulk-delete", bulkDeleteHandler(db))
	admin.HandleFunc("GET /uploads/{id}", detailHandler(db))
	admin.HandleFunc("DELETE /uploads/{id}", deleteHandler(db))
	admin.HandleFunc("GET /uploads/{id}/file", fileHandler(db, uploadsDir))
	admin.HandleFunc("GET /uploads/{id}/thumb", thumbHandler(db, uploadsDir))
}

type uploadJSON struct {
	ID           int64  `json:"id"`
	UUID         string `json:"uuid"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	HasThumbnail bool   `json:"has_thumbnail"`
	UploadedBy   string `json:"uploaded_by"`
	EntityType   string `json:"entity_type"`
	EntityID     string `json:"entity_id"`
	CreatedAt    string `json:"created_at"`
	DeletedAt    string `json:"deleted_at"`
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		pg := http.ParsePagination(r, 50, 100)
		contentType := http.QueryParam(r, "content_type")
		entityType := http.QueryParam(r, "entity_type")
		includeDeleted := http.QueryParam(r, "include_deleted") == "true"

		q := sqlite.Select("id", "uuid", "original_name", "content_type",
			"size_bytes", "has_thumbnail", "uploaded_by",
			"entity_type", "entity_id", "created_at", "COALESCE(deleted_at, '')").
			From("uploads")
		if !includeDeleted {
			q.Where("deleted_at IS NULL")
		}
		if contentType != "" {
			q.Where("content_type LIKE ?", contentType+"%")
		}
		if entityType != "" {
			q.Where("entity_type = ?", entityType)
		}

		var total int
		sql, args := sqlite.CountFrom(q).Build()
		_ = db.QueryRow(sql, args...).Scan(&total)

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "original_name", "content_type", "size_bytes", "created_at"},
			"id", "DESC")
		sql, args = q.OrderBy(sortCol, sortDir).Limit(pg.Limit).Offset(pg.Offset).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to list uploads")
			return
		}
		defer rows.Close()

		uploads := make([]uploadJSON, 0)
		for rows.Next() {
			var u uploadJSON
			var hasThumbnail int
			if err := rows.Scan(&u.ID, &u.UUID, &u.OriginalName, &u.ContentType,
				&u.SizeBytes, &hasThumbnail, &u.UploadedBy,
				&u.EntityType, &u.EntityID, &u.CreatedAt, &u.DeletedAt); err != nil {
				http.WriteError(w, http.StatusInternalServerError, "failed to scan upload")
				return
			}
			u.HasThumbnail = hasThumbnail == 1
			uploads = append(uploads, u)
		}
		if err := rows.Err(); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to iterate uploads")
			return
		}

		http.PaginatedResponse(w, "uploads", uploads, total)
	}
}

func exportHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := http.QueryParam(r, "content_type")
		entityType := http.QueryParam(r, "entity_type")
		includeDeleted := http.QueryParam(r, "include_deleted") == "true"

		q := sqlite.Select("id", "uuid", "original_name", "content_type",
			"size_bytes", "has_thumbnail", "uploaded_by",
			"entity_type", "entity_id", "created_at", "COALESCE(deleted_at, '')").
			From("uploads")
		if !includeDeleted {
			q.Where("deleted_at IS NULL")
		}
		if contentType != "" {
			q.Where("content_type LIKE ?", contentType+"%")
		}
		if entityType != "" {
			q.Where("entity_type = ?", entityType)
		}

		sortCol, sortDir := http.QueryParamSort(r,
			[]string{"id", "original_name", "content_type", "size_bytes", "created_at"},
			"id", "DESC")

		sql, args := q.OrderBy(sortCol, sortDir).Build()
		rows, err := db.Query(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to export uploads")
			return
		}
		defer rows.Close()

		http.WriteCSV(w, "uploads", []string{"ID", "UUID", "Original Name", "Content Type", "Size (bytes)", "Has Thumbnail", "Uploaded By", "Entity Type", "Entity ID", "Created At", "Deleted At"}, func() []string {
			if !rows.Next() {
				return nil
			}
			var id, sizeBytes int64
			var uuid, originalName, ct, uploadedBy, et, entityID, createdAt, deletedAt string
			var hasThumbnail int
			if err := rows.Scan(&id, &uuid, &originalName, &ct, &sizeBytes, &hasThumbnail, &uploadedBy, &et, &entityID, &createdAt, &deletedAt); err != nil {
				return nil
			}
			thumb := "No"
			if hasThumbnail == 1 {
				thumb = "Yes"
			}
			return []string{strconv.FormatInt(id, 10), uuid, originalName, ct, strconv.FormatInt(sizeBytes, 10), thumb, uploadedBy, et, entityID, createdAt, deletedAt}
		})
	}
}

func uploadHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = nethttp.MaxBytesReader(w, r.Body, maxUploadSize)
		if err := r.ParseMultipartForm(maxUploadSize); err != nil {
			http.WriteError(w, http.StatusBadRequest, "file too large or invalid multipart form")
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.WriteError(w, http.StatusBadRequest, "missing file field")
			return
		}
		defer file.Close()

		// Generate UUID for this upload.
		uuidBytes := make([]byte, 16)
		if _, err := rand.Read(uuidBytes); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to generate uuid")
			return
		}
		uploadUUID := hex.EncodeToString(uuidBytes)

		// Build storage path: uploads/YYYY/MM/DD/{UUID}/
		now := time.Now().UTC()
		datePath := filepath.Join(
			now.Format("2006"),
			now.Format("01"),
			now.Format("02"),
		)
		dirPath := filepath.Join(uploadsDir, datePath, uploadUUID)
		if err := os.MkdirAll(dirPath, 0o755); err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to create upload directory")
			return
		}

		// Sanitize filename — keep only the base name.
		originalName := filepath.Base(header.Filename)
		storedName := originalName
		filePath := filepath.Join(dirPath, storedName)

		// Write file to disk.
		dst, err := os.Create(filePath)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to create file")
			return
		}

		written, err := io.Copy(dst, file)
		dst.Close()
		if err != nil {
			_ = os.Remove(filePath)
			http.WriteError(w, http.StatusInternalServerError, "failed to write file")
			return
		}

		// Detect content type from extension.
		contentType := detectContentType(originalName)

		// Relative storage path for DB (relative to uploads dir).
		storagePath := filepath.Join(datePath, uploadUUID, storedName)

		// Generate thumbnail for images.
		hasThumbnail := 0
		if isImage(contentType) {
			if generateThumbnail(filePath, filepath.Join(dirPath, thumbFilename)) {
				hasThumbnail = 1
			}
		}

		// Get uploader identity from JWT claims.
		var uploadedBy string
		claims, ok := auth.ClaimsFromContext(r.Context())
		if ok {
			uploadedBy = claims.UID
		}

		// Optional entity association from form fields.
		entityType := r.FormValue("entity_type")
		entityID := r.FormValue("entity_id")

		// Insert into DB.
		sql, args := sqlite.Insert("uploads").
			Set("uuid", uploadUUID).
			Set("original_name", originalName).
			Set("stored_name", storedName).
			Set("content_type", contentType).
			Set("size_bytes", written).
			Set("storage_path", storagePath).
			Set("has_thumbnail", hasThumbnail).
			Set("uploaded_by", uploadedBy).
			Set("entity_type", entityType).
			Set("entity_id", entityID).
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			_ = os.RemoveAll(dirPath)
			http.WriteError(w, http.StatusInternalServerError, "failed to save upload record")
			return
		}

		adminaudit.Log(db, r, "upload.create", "upload", strconv.FormatInt(result.LastInsertID, 10),
			fmt.Sprintf("file=%s size=%d type=%s", originalName, written, contentType))

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"upload": uploadJSON{
				ID:           result.LastInsertID,
				UUID:         uploadUUID,
				OriginalName: originalName,
				ContentType:  contentType,
				SizeBytes:    written,
				HasThumbnail: hasThumbnail == 1,
				UploadedBy:   uploadedBy,
				EntityType:   entityType,
				EntityID:     entityID,
				CreatedAt:    now.Format(time.RFC3339),
			},
		})
	}
}

func detailHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		u, err := findUpload(db, id)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "upload not found")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"upload": u,
		})
	}
}

func deleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		sql, args := sqlite.Update("uploads").
			Set("deleted_at", now).
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		result, err := db.Exec(sql, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to delete upload")
			return
		}
		if result.RowsAffected == 0 {
			http.WriteError(w, http.StatusNotFound, "upload not found or already deleted")
			return
		}

		adminaudit.Log(db, r, "upload.delete", "upload", strconv.FormatInt(id, 10), "")

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func fileHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var storagePath, originalName, contentType string
		sql, args := sqlite.Select("storage_path", "original_name", "content_type").
			From("uploads").
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		if err := db.QueryRow(sql, args...).Scan(&storagePath, &originalName, &contentType); err != nil {
			http.WriteError(w, http.StatusNotFound, "upload not found")
			return
		}

		filePath := filepath.Join(uploadsDir, storagePath)
		f, err := os.Open(filePath)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "file not found on disk")
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to stat file")
			return
		}

		// Serve inline for images, as attachment for everything else.
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		if !isImage(contentType) {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, originalName))
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.WriteHeader(200)
		_, _ = io.Copy(w, f)
	}
}

func thumbHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var storagePath string
		var hasThumbnail int
		sql, args := sqlite.Select("storage_path", "has_thumbnail").
			From("uploads").
			Where("id = ?", id).
			Where("deleted_at IS NULL").
			Build()
		if err := db.QueryRow(sql, args...).Scan(&storagePath, &hasThumbnail); err != nil {
			http.WriteError(w, http.StatusNotFound, "upload not found")
			return
		}

		if hasThumbnail == 0 {
			http.WriteError(w, http.StatusNotFound, "no thumbnail available")
			return
		}

		thumbPath := filepath.Join(uploadsDir, filepath.Dir(storagePath), thumbFilename)
		f, err := os.Open(thumbPath)
		if err != nil {
			http.WriteError(w, http.StatusNotFound, "thumbnail file not found")
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to stat thumbnail")
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(200)
		_, _ = io.Copy(w, f)
	}
}

func bulkDeleteHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IDs []int64 `json:"ids"`
		}
		if err := http.ReadJSON(r, &req); err != nil {
			http.WriteError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if !http.CheckBulkIDs(w, req.IDs, 100) {
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		ids := make([]any, len(req.IDs))
		for i, id := range req.IDs {
			ids[i] = id
		}

		query, args := sqlite.Update("uploads").
			Set("deleted_at", now).
			Where("deleted_at IS NULL").
			WhereIn("id", ids...).
			Build()
		result, err := db.Exec(query, args...)
		if err != nil {
			http.WriteError(w, http.StatusInternalServerError, "failed to bulk delete uploads")
			return
		}

		for _, id := range req.IDs {
			adminaudit.Log(db, r, "upload.delete", "upload", strconv.FormatInt(id, 10), "bulk")
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok":       true,
			"affected": result.RowsAffected,
		})
	}
}

// findUpload loads a single upload record by ID.
func findUpload(db *sqlite.DB, id int64) (uploadJSON, error) {
	sql, args := sqlite.Select("id", "uuid", "original_name", "content_type",
		"size_bytes", "has_thumbnail", "uploaded_by",
		"entity_type", "entity_id", "created_at", "COALESCE(deleted_at, '')").
		From("uploads").
		Where("id = ?", id).
		Build()

	var u uploadJSON
	var hasThumbnail int
	err := db.QueryRow(sql, args...).Scan(&u.ID, &u.UUID, &u.OriginalName, &u.ContentType,
		&u.SizeBytes, &hasThumbnail, &u.UploadedBy,
		&u.EntityType, &u.EntityID, &u.CreatedAt, &u.DeletedAt)
	if err != nil {
		return uploadJSON{}, err
	}
	u.HasThumbnail = hasThumbnail == 1
	return u, nil
}

// generateThumbnail creates a JPEG thumbnail for an image file.
// Returns true on success, false if thumbnail generation fails (non-fatal).
func generateThumbnail(srcPath, dstPath string) bool {
	src, err := os.Open(srcPath)
	if err != nil {
		return false
	}
	defer src.Close()

	img, _, err := image.Decode(src)
	if err != nil {
		return false
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	// Calculate thumbnail dimensions maintaining aspect ratio.
	dstW, dstH := thumbDimensions(srcW, srcH, thumbMaxDim)
	if dstW <= 0 || dstH <= 0 {
		return false
	}

	thumb := resizeNearestNeighbor(img, dstW, dstH)

	out, err := os.Create(dstPath)
	if err != nil {
		return false
	}
	defer out.Close()

	if err := jpeg.Encode(out, thumb, &jpeg.Options{Quality: 80}); err != nil {
		_ = os.Remove(dstPath)
		return false
	}

	return true
}

// thumbDimensions computes thumbnail width/height to fit within maxDim
// while preserving aspect ratio.
func thumbDimensions(srcW, srcH, maxDim int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return 0, 0
	}
	if srcW <= maxDim && srcH <= maxDim {
		return srcW, srcH
	}
	if srcW >= srcH {
		return maxDim, srcH * maxDim / srcW
	}
	return srcW * maxDim / srcH, maxDim
}

// resizeNearestNeighbor scales an image using nearest-neighbor interpolation.
// This is fast and uses only Go stdlib — no external image processing libraries.
func resizeNearestNeighbor(src image.Image, dstW, dstH int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := bounds.Min.Y + y*srcH/dstH
		for x := 0; x < dstW; x++ {
			srcX := bounds.Min.X + x*srcW/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

// isImage returns true if the content type indicates an image that
// Go's standard library can decode (JPEG, PNG, GIF).
func isImage(contentType string) bool {
	switch contentType {
	case "image/jpeg", "image/png", "image/gif":
		return true
	}
	return false
}

// detectContentType guesses the MIME type from a filename extension.
func detectContentType(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".pdf":
		return "application/pdf"
	case ".zip":
		return "application/zip"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	case ".txt":
		return "text/plain"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "application/javascript"
	case ".mp4":
		return "video/mp4"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".doc", ".docx":
		return "application/msword"
	case ".xls", ".xlsx":
		return "application/vnd.ms-excel"
	default:
		return "application/octet-stream"
	}
}
