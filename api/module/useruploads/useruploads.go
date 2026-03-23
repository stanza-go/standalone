// Package useruploads provides user-facing endpoints for file upload management.
// Files are stored at {DATA_DIR}/uploads/YYYY/MM/DD/{UUID}/filename with
// automatic thumbnail generation for images. All queries are scoped to the
// authenticated user via entity_type="user" and entity_id=userID.
package useruploads

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
	"strings"
	"time"

	"github.com/stanza-go/framework/pkg/auth"
	"github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
)

const (
	maxUploadSize = 50 << 20 // 50 MB
	thumbMaxDim   = 300      // Max width or height for thumbnails.
	thumbFilename = "thumbnail.jpg"
	entityType    = "user"
)

// Register mounts the user upload management routes on the given group.
// The group should already have user auth middleware applied.
// Routes:
//
//	GET    /api/user/uploads            - list user's uploads with pagination and filters
//	POST   /api/user/uploads            - upload a file
//	GET    /api/user/uploads/{id}       - get upload metadata
//	DELETE /api/user/uploads/{id}       - soft-delete an upload
//	GET    /api/user/uploads/{id}/file  - serve the original file
//	GET    /api/user/uploads/{id}/thumb - serve the thumbnail (images only)
func Register(user *http.Group, db *sqlite.DB, uploadsDir string) {
	user.HandleFunc("GET /uploads", listHandler(db))
	user.HandleFunc("POST /uploads", uploadHandler(db, uploadsDir))
	user.HandleFunc("GET /uploads/{id}", detailHandler(db))
	user.HandleFunc("DELETE /uploads/{id}", deleteHandler(db))
	user.HandleFunc("GET /uploads/{id}/file", fileHandler(db, uploadsDir))
	user.HandleFunc("GET /uploads/{id}/thumb", thumbHandler(db, uploadsDir))
}

type uploadJSON struct {
	ID           int64  `json:"id"`
	UUID         string `json:"uuid"`
	OriginalName string `json:"original_name"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	HasThumbnail bool   `json:"has_thumbnail"`
	CreatedAt    string `json:"created_at"`
}

func scanUserUpload(rows *sqlite.Rows) (uploadJSON, error) {
	var u uploadJSON
	if err := rows.Scan(&u.ID, &u.UUID, &u.OriginalName, &u.ContentType,
		&u.SizeBytes, &u.HasThumbnail, &u.CreatedAt); err != nil {
		return u, err
	}
	return u, nil
}

func listHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		pg := http.ParsePagination(r, 50, 100)
		contentType := http.QueryParam(r, "content_type")

		q := sqlite.Select("id", "uuid", "original_name", "content_type",
			"size_bytes", "has_thumbnail", "created_at").
			From("uploads").
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			WhereNull("deleted_at")
		if contentType != "" {
			q.Where("content_type LIKE ?", contentType+"%")
		}

		total, _ := db.Count(q)

		sql, args := q.OrderBy("id", "DESC").Limit(pg.Limit).Offset(pg.Offset).Build()
		uploads, err := sqlite.QueryAll(db, sql, args, scanUserUpload)
		if err != nil {
			http.WriteServerError(w, r, "failed to list uploads", err)
			return
		}

		http.PaginatedResponse(w, "uploads", uploads, total)
	}
}

func uploadHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

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
		defer func() { _ = file.Close() }()

		// Generate UUID for this upload.
		uuidBytes := make([]byte, 16)
		if _, err := rand.Read(uuidBytes); err != nil {
			http.WriteServerError(w, r, "failed to generate uuid", err)
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
			http.WriteServerError(w, r, "failed to create upload directory", err)
			return
		}

		// Sanitize filename.
		originalName := filepath.Base(header.Filename)
		storedName := originalName
		filePath := filepath.Join(dirPath, storedName)

		// Write file to disk.
		dst, err := os.Create(filePath)
		if err != nil {
			http.WriteServerError(w, r, "failed to create file", err)
			return
		}

		written, err := io.Copy(dst, file)
		if closeErr := dst.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(filePath)
			http.WriteServerError(w, r, "failed to write file", err)
			return
		}

		// Detect content type from extension.
		ct := detectContentType(originalName)

		// Relative storage path for DB.
		storagePath := filepath.Join(datePath, uploadUUID, storedName)

		// Generate thumbnail for images.
		hasThumbnail := false
		if isImage(ct) {
			if generateThumbnail(filePath, filepath.Join(dirPath, thumbFilename)) {
				hasThumbnail = true
			}
		}

		// Insert into DB with entity_type="user" and entity_id=userID.
		id, err := db.Insert(sqlite.Insert("uploads").
			Set("uuid", uploadUUID).
			Set("original_name", originalName).
			Set("stored_name", storedName).
			Set("content_type", ct).
			Set("size_bytes", written).
			Set("storage_path", storagePath).
			Set("has_thumbnail", hasThumbnail).
			Set("uploaded_by", userID).
			Set("entity_type", entityType).
			Set("entity_id", userID))
		if err != nil {
			_ = os.RemoveAll(dirPath)
			http.WriteServerError(w, r, "failed to save upload record", err)
			return
		}

		http.WriteJSON(w, http.StatusCreated, map[string]any{
			"upload": uploadJSON{
				ID:           id,
				UUID:         uploadUUID,
				OriginalName: originalName,
				ContentType:  ct,
				SizeBytes:    written,
				HasThumbnail: hasThumbnail,
				CreatedAt:    now.Format(time.RFC3339),
			},
		})
	}
}

func detailHandler(db *sqlite.DB) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		u, err := findUserUpload(db, id, userID)
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
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		now := sqlite.Now()
		n, err := db.Update(sqlite.Update("uploads").
			Set("deleted_at", now).
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			WhereNull("deleted_at"))
		if err != nil {
			http.WriteServerError(w, r, "failed to delete upload", err)
			return
		}
		if n == 0 {
			http.WriteError(w, http.StatusNotFound, "upload not found or already deleted")
			return
		}

		http.WriteJSON(w, http.StatusOK, map[string]any{
			"ok": true,
		})
	}
}

func fileHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var storagePath, originalName, ct string
		sql, args := sqlite.Select("storage_path", "original_name", "content_type").
			From("uploads").
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			WhereNull("deleted_at").
			Build()
		if err := db.QueryRow(sql, args...).Scan(&storagePath, &originalName, &ct); err != nil {
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
			http.WriteServerError(w, r, "failed to stat file", err)
			return
		}

		if ct != "" {
			w.Header().Set("Content-Type", ct)
		} else {
			w.Header().Set("Content-Type", "application/octet-stream")
		}
		if !isImage(ct) {
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, originalName))
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.WriteHeader(200)
		_, _ = io.Copy(w, f)
	}
}

func thumbHandler(db *sqlite.DB, uploadsDir string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, _ := auth.ClaimsFromContext(r.Context())
		userID := claims.UID

		id, ok := http.PathParamInt64(w, r, "id")
		if !ok {
			return
		}

		var storagePath string
		var hasThumbnail bool
		sql, args := sqlite.Select("storage_path", "has_thumbnail").
			From("uploads").
			Where("id = ?", id).
			Where("entity_type = ?", entityType).
			Where("entity_id = ?", userID).
			WhereNull("deleted_at").
			Build()
		if err := db.QueryRow(sql, args...).Scan(&storagePath, &hasThumbnail); err != nil {
			http.WriteError(w, http.StatusNotFound, "upload not found")
			return
		}

		if !hasThumbnail {
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
			http.WriteServerError(w, r, "failed to stat thumbnail", err)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.WriteHeader(200)
		_, _ = io.Copy(w, f)
	}
}

// findUserUpload loads a single upload record by ID, scoped to the given user.
func findUserUpload(db *sqlite.DB, id int64, userID string) (uploadJSON, error) {
	sql, args := sqlite.Select("id", "uuid", "original_name", "content_type",
		"size_bytes", "has_thumbnail", "created_at").
		From("uploads").
		Where("id = ?", id).
		Where("entity_type = ?", entityType).
		Where("entity_id = ?", userID).
		WhereNull("deleted_at").
		Build()
	return sqlite.QueryOne(db, sql, args, scanUserUpload)
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
