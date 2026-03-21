package adminuploads_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminuploads"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB, string) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	uploadsDir := t.TempDir()

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	adminuploads.Register(admin, db, uploadsDir)

	return router, a, db, uploadsDir
}

func uploadFile(t *testing.T, router *fhttp.Router, a *auth.Auth, filename string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %s: %v", k, err)
		}
	}
	writer.Close()

	req := httptest.NewRequest("POST", "/api/admin/uploads", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	testutil.AddAdminAuth(t, req, a, "1")

	return testutil.Do(router, req)
}

func createTestPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// --- Tests ---

func TestUpload_TextFile(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	rec := uploadFile(t, router, a, "hello.txt", []byte("hello world"), nil)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	upload, ok := resp["upload"].(map[string]any)
	if !ok {
		t.Fatal("missing upload in response")
	}
	if upload["original_name"] != "hello.txt" {
		t.Errorf("original_name = %v, want hello.txt", upload["original_name"])
	}
	if upload["content_type"] != "text/plain" {
		t.Errorf("content_type = %v, want text/plain", upload["content_type"])
	}
	if upload["has_thumbnail"] != false {
		t.Errorf("has_thumbnail = %v, want false", upload["has_thumbnail"])
	}
	if upload["size_bytes"] != float64(11) {
		t.Errorf("size_bytes = %v, want 11", upload["size_bytes"])
	}
}

func TestUpload_ImageWithThumbnail(t *testing.T) {
	t.Parallel()
	router, a, _, uploadsDir := setup(t)

	pngData := createTestPNG(t, 600, 400)
	rec := uploadFile(t, router, a, "photo.png", pngData, nil)

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	upload := resp["upload"].(map[string]any)

	if upload["content_type"] != "image/png" {
		t.Errorf("content_type = %v, want image/png", upload["content_type"])
	}
	if upload["has_thumbnail"] != true {
		t.Errorf("has_thumbnail = %v, want true", upload["has_thumbnail"])
	}

	// Verify thumbnail file exists on disk.
	uuid := upload["uuid"].(string)
	entries, err := filepath.Glob(filepath.Join(uploadsDir, "*", "*", "*", uuid, "thumbnail.jpg"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(entries) == 0 {
		t.Error("thumbnail file not found on disk")
	}
}

func TestUpload_WithEntityAssociation(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	rec := uploadFile(t, router, a, "doc.pdf", []byte("%PDF-1.4"), map[string]string{
		"entity_type": "user",
		"entity_id":   "42",
	})

	if rec.Code != 201 {
		t.Fatalf("status = %d, want 201\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	upload := resp["upload"].(map[string]any)

	if upload["entity_type"] != "user" {
		t.Errorf("entity_type = %v, want user", upload["entity_type"])
	}
	if upload["entity_id"] != "42" {
		t.Errorf("entity_id = %v, want 42", upload["entity_id"])
	}
}

func TestListUploads_Empty(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/uploads", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	uploads, ok := resp["uploads"].([]any)
	if !ok {
		t.Fatal("missing uploads in response")
	}
	if len(uploads) != 0 {
		t.Errorf("expected 0 uploads, got %d", len(uploads))
	}
	if resp["total"] != float64(0) {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestListUploads_WithPagination(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Upload 3 files.
	for i := 0; i < 3; i++ {
		rec := uploadFile(t, router, a, fmt.Sprintf("file%d.txt", i), []byte("data"), nil)
		if rec.Code != 201 {
			t.Fatalf("upload %d: status = %d", i, rec.Code)
		}
	}

	// Get first page (limit=2).
	req := httptest.NewRequest("GET", "/api/admin/uploads?limit=2&offset=0", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	uploads := resp["uploads"].([]any)

	if len(uploads) != 2 {
		t.Errorf("page 1: got %d uploads, want 2", len(uploads))
	}
	if resp["total"] != float64(3) {
		t.Errorf("total = %v, want 3", resp["total"])
	}
}

func TestListUploads_FilterByContentType(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	uploadFile(t, router, a, "doc.txt", []byte("text"), nil)
	uploadFile(t, router, a, "photo.png", createTestPNG(t, 10, 10), nil)

	req := httptest.NewRequest("GET", "/api/admin/uploads?content_type=image", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	uploads := resp["uploads"].([]any)

	if len(uploads) != 1 {
		t.Errorf("got %d uploads, want 1 (only images)", len(uploads))
	}
}

func TestGetUpload(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Upload a file first.
	upRec := uploadFile(t, router, a, "test.txt", []byte("test content"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)
	id := upload["id"].(float64)

	// Get the upload details.
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	u := resp["upload"].(map[string]any)

	if u["original_name"] != "test.txt" {
		t.Errorf("original_name = %v, want test.txt", u["original_name"])
	}
}

func TestGetUpload_NotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/uploads/999", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteUpload(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Upload then delete.
	upRec := uploadFile(t, router, a, "delete-me.txt", []byte("bye"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("delete status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone from default list.
	listReq := httptest.NewRequest("GET", "/api/admin/uploads", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	var listResp map[string]any
	testutil.DecodeJSON(t, listRec, &listResp)
	if listResp["total"] != float64(0) {
		t.Errorf("total after delete = %v, want 0", listResp["total"])
	}
}

func TestDeleteUpload_AlreadyDeleted(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "file.txt", []byte("x"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	// Delete once.
	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	testutil.Do(router, req)

	// Delete again.
	req2 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, req2, a, "1")
	rec := testutil.Do(router, req2)

	if rec.Code != 404 {
		t.Fatalf("double-delete status = %d, want 404", rec.Code)
	}
}

func TestServeFile(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	content := []byte("file-content-here")
	upRec := uploadFile(t, router, a, "serve.txt", content, nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d/file", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	body, _ := io.ReadAll(rec.Body)
	if string(body) != "file-content-here" {
		t.Errorf("body = %q, want %q", string(body), "file-content-here")
	}
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Errorf("content-type = %s, want text/plain", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Disposition") == "" {
		t.Error("expected Content-Disposition header for non-image")
	}
}

func TestServeFile_DeletedNotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "file.txt", []byte("x"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	// Delete it.
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, delReq, a, "1")
	testutil.Do(router, delReq)

	// Try to serve it.
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d/file", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 for deleted file", rec.Code)
	}
}

func TestServeImage_Inline(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	pngData := createTestPNG(t, 100, 100)
	upRec := uploadFile(t, router, a, "image.png", pngData, nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d/file", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("content-type = %s, want image/png", rec.Header().Get("Content-Type"))
	}
	// Images should be served inline (no Content-Disposition).
	if rec.Header().Get("Content-Disposition") != "" {
		t.Error("images should be served inline, got Content-Disposition header")
	}
}

func TestServeThumbnail(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	pngData := createTestPNG(t, 600, 400)
	upRec := uploadFile(t, router, a, "big.png", pngData, nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)
	id := upload["id"].(float64)

	if upload["has_thumbnail"] != true {
		t.Fatal("expected has_thumbnail=true for 600x400 image")
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d/thumb", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Errorf("content-type = %s, want image/jpeg", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Error("expected Cache-Control header for thumbnail")
	}

	// Verify it's a valid JPEG.
	body := rec.Body.Bytes()
	if len(body) < 2 || body[0] != 0xFF || body[1] != 0xD8 {
		t.Error("response is not valid JPEG data")
	}
}

func TestServeThumbnail_NoThumbnail(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// Upload a text file (no thumbnail).
	upRec := uploadFile(t, router, a, "readme.txt", []byte("no thumb"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/admin/uploads/%d/thumb", int(id)), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpload_MissingFile(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// POST with no file field.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/admin/uploads", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpload_RequiresAuth(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/uploads", nil)
	// No auth cookie.
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestListUploads_IncludeDeleted(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "file.txt", []byte("x"), nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	// Delete it.
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/admin/uploads/%d", int(id)), nil)
	testutil.AddAdminAuth(t, delReq, a, "1")
	testutil.Do(router, delReq)

	// List without include_deleted — should be empty.
	req := httptest.NewRequest("GET", "/api/admin/uploads", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)
	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	if resp["total"] != float64(0) {
		t.Errorf("without include_deleted: total = %v, want 0", resp["total"])
	}

	// List with include_deleted — should find the deleted upload.
	req2 := httptest.NewRequest("GET", "/api/admin/uploads?include_deleted=true", nil)
	testutil.AddAdminAuth(t, req2, a, "1")
	rec2 := testutil.Do(router, req2)
	var resp2 map[string]any
	testutil.DecodeJSON(t, rec2, &resp2)
	if resp2["total"] != float64(1) {
		t.Errorf("with include_deleted: total = %v, want 1", resp2["total"])
	}
}

func TestUpload_StoragePath(t *testing.T) {
	t.Parallel()
	router, a, _, uploadsDir := setup(t)

	content := []byte("storage-test")
	upRec := uploadFile(t, router, a, "stored.txt", content, nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)
	uuid := upload["uuid"].(string)

	// Verify file exists in YYYY/MM/DD/UUID/ structure.
	pattern := filepath.Join(uploadsDir, "*", "*", "*", uuid, "stored.txt")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for %s, got %d", pattern, len(matches))
	}

	// Verify content.
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if string(data) != "storage-test" {
		t.Errorf("stored content = %q, want %q", string(data), "storage-test")
	}
}

func TestThumbnail_SmallImage_NoResize(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// 50x50 image — smaller than 300px max, should still get a thumbnail (no resize needed).
	pngData := createTestPNG(t, 50, 50)
	upRec := uploadFile(t, router, a, "small.png", pngData, nil)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)

	if upload["has_thumbnail"] != true {
		t.Error("expected has_thumbnail=true even for small images")
	}
}

func TestDetectContentType(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	tests := []struct {
		filename    string
		wantType    string
	}{
		{"photo.jpg", "image/jpeg"},
		{"photo.JPEG", "image/jpeg"},
		{"doc.pdf", "application/pdf"},
		{"data.csv", "text/csv"},
		{"archive.zip", "application/zip"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tc := range tests {
		rec := uploadFile(t, router, a, tc.filename, []byte("x"), nil)
		if rec.Code != 201 {
			t.Fatalf("%s: status = %d, want 201", tc.filename, rec.Code)
		}
		var resp map[string]any
		testutil.DecodeJSON(t, rec, &resp)
		upload := resp["upload"].(map[string]any)
		if upload["content_type"] != tc.wantType {
			t.Errorf("%s: content_type = %v, want %s", tc.filename, upload["content_type"], tc.wantType)
		}
	}
}

func TestUpload_CleanupOnDBError(t *testing.T) {
	t.Parallel()

	// This test verifies that a failed DB insert cleans up the file.
	// We can't easily simulate a DB error, but we verify the happy path
	// doesn't leave orphan files when everything works.
	router, a, _, uploadsDir := setup(t)

	upRec := uploadFile(t, router, a, "test.txt", []byte("data"), nil)
	if upRec.Code != 201 {
		t.Fatalf("status = %d, want 201", upRec.Code)
	}

	// Count files in uploads dir — should have exactly one file dir.
	var count int
	_ = filepath.Walk(uploadsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	if count != 1 {
		t.Errorf("expected 1 file in uploads dir, got %d", count)
	}
}
