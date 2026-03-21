package useruploads_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/useruploads"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *sqlite.DB, string) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewUserAuth()
	uploadsDir := t.TempDir()

	router := testutil.NewRouter()
	api := router.Group("/api")
	user := api.Group("/user")
	user.Use(a.RequireAuth())
	user.Use(auth.RequireScope("user"))
	useruploads.Register(user, db, uploadsDir)

	return router, a, db, uploadsDir
}

func uploadFile(t *testing.T, router *fhttp.Router, a *auth.Auth, uid, filename string, content []byte) *httptest.ResponseRecorder {
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
	writer.Close()

	req := httptest.NewRequest("POST", "/api/user/uploads", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	testutil.AddUserAuth(t, req, a, uid)

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

	rec := uploadFile(t, router, a, "100", "hello.txt", []byte("hello world"))

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
	rec := uploadFile(t, router, a, "100", "photo.png", pngData)

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

func TestListUploads_Empty(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/user/uploads", nil)
	testutil.AddUserAuth(t, req, a, "100")
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

	for i := 0; i < 3; i++ {
		rec := uploadFile(t, router, a, "100", fmt.Sprintf("file%d.txt", i), []byte("data"))
		if rec.Code != 201 {
			t.Fatalf("upload %d: status = %d", i, rec.Code)
		}
	}

	req := httptest.NewRequest("GET", "/api/user/uploads?limit=2&offset=0", nil)
	testutil.AddUserAuth(t, req, a, "100")
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

	uploadFile(t, router, a, "100", "doc.txt", []byte("text"))
	uploadFile(t, router, a, "100", "photo.png", createTestPNG(t, 10, 10))

	req := httptest.NewRequest("GET", "/api/user/uploads?content_type=image", nil)
	testutil.AddUserAuth(t, req, a, "100")
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

	upRec := uploadFile(t, router, a, "100", "test.txt", []byte("test content"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)
	id := upload["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
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

	req := httptest.NewRequest("GET", "/api/user/uploads/999", nil)
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestDeleteUpload(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "100", "delete-me.txt", []byte("bye"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("delete status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	// Verify it's gone from list.
	listReq := httptest.NewRequest("GET", "/api/user/uploads", nil)
	testutil.AddUserAuth(t, listReq, a, "100")
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

	upRec := uploadFile(t, router, a, "100", "file.txt", []byte("x"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("DELETE", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
	testutil.Do(router, req)

	req2 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req2, a, "100")
	rec := testutil.Do(router, req2)

	if rec.Code != 404 {
		t.Fatalf("double-delete status = %d, want 404", rec.Code)
	}
}

func TestServeFile(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	content := []byte("file-content-here")
	upRec := uploadFile(t, router, a, "100", "serve.txt", content)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/file", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
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

func TestServeImage_Inline(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	pngData := createTestPNG(t, 100, 100)
	upRec := uploadFile(t, router, a, "100", "image.png", pngData)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/file", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "image/png" {
		t.Errorf("content-type = %s, want image/png", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Content-Disposition") != "" {
		t.Error("images should be served inline, got Content-Disposition header")
	}
}

func TestServeThumbnail(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	pngData := createTestPNG(t, 600, 400)
	upRec := uploadFile(t, router, a, "100", "big.png", pngData)
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	upload := upResp["upload"].(map[string]any)
	id := upload["id"].(float64)

	if upload["has_thumbnail"] != true {
		t.Fatal("expected has_thumbnail=true for 600x400 image")
	}

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/thumb", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
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

	body := rec.Body.Bytes()
	if len(body) < 2 || body[0] != 0xFF || body[1] != 0xD8 {
		t.Error("response is not valid JPEG data")
	}
}

func TestServeThumbnail_NoThumbnail(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "100", "readme.txt", []byte("no thumb"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/thumb", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestUpload_RequiresAuth(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/user/uploads", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestUpload_MissingFile(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.Close()

	req := httptest.NewRequest("POST", "/api/user/uploads", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestUpload_Isolation_BetweenUsers(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// User 100 uploads a file.
	rec := uploadFile(t, router, a, "100", "user100.txt", []byte("user100 data"))
	if rec.Code != 201 {
		t.Fatalf("user100 upload: status = %d", rec.Code)
	}

	// User 200 uploads a file.
	rec = uploadFile(t, router, a, "200", "user200.txt", []byte("user200 data"))
	if rec.Code != 201 {
		t.Fatalf("user200 upload: status = %d", rec.Code)
	}

	// User 100 should only see their own upload.
	req := httptest.NewRequest("GET", "/api/user/uploads", nil)
	testutil.AddUserAuth(t, req, a, "100")
	listRec := testutil.Do(router, req)

	var resp map[string]any
	testutil.DecodeJSON(t, listRec, &resp)

	if resp["total"] != float64(1) {
		t.Errorf("user100 total = %v, want 1", resp["total"])
	}
	uploads := resp["uploads"].([]any)
	if len(uploads) != 1 {
		t.Fatalf("user100 uploads count = %d, want 1", len(uploads))
	}
	if uploads[0].(map[string]any)["original_name"] != "user100.txt" {
		t.Errorf("user100 sees wrong file: %v", uploads[0].(map[string]any)["original_name"])
	}

	// User 200 should only see their own upload.
	req2 := httptest.NewRequest("GET", "/api/user/uploads", nil)
	testutil.AddUserAuth(t, req2, a, "200")
	listRec2 := testutil.Do(router, req2)

	var resp2 map[string]any
	testutil.DecodeJSON(t, listRec2, &resp2)

	if resp2["total"] != float64(1) {
		t.Errorf("user200 total = %v, want 1", resp2["total"])
	}
	uploads2 := resp2["uploads"].([]any)
	if uploads2[0].(map[string]any)["original_name"] != "user200.txt" {
		t.Errorf("user200 sees wrong file: %v", uploads2[0].(map[string]any)["original_name"])
	}
}

func TestUpload_Isolation_CannotAccessOtherUserUpload(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	// User 100 uploads a file.
	upRec := uploadFile(t, router, a, "100", "secret.txt", []byte("secret"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	// User 200 tries to access user 100's upload — should get 404.
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "200")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("cross-user detail: status = %d, want 404", rec.Code)
	}

	// User 200 tries to serve user 100's file — should get 404.
	req2 := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/file", int(id)), nil)
	testutil.AddUserAuth(t, req2, a, "200")
	rec2 := testutil.Do(router, req2)

	if rec2.Code != 404 {
		t.Fatalf("cross-user file: status = %d, want 404", rec2.Code)
	}

	// User 200 tries to delete user 100's upload — should get 404.
	req3 := httptest.NewRequest("DELETE", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, req3, a, "200")
	rec3 := testutil.Do(router, req3)

	if rec3.Code != 404 {
		t.Fatalf("cross-user delete: status = %d, want 404", rec3.Code)
	}
}

func TestServeFile_DeletedNotFound(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	upRec := uploadFile(t, router, a, "100", "file.txt", []byte("x"))
	var upResp map[string]any
	testutil.DecodeJSON(t, upRec, &upResp)
	id := upResp["upload"].(map[string]any)["id"].(float64)

	// Delete it.
	delReq := httptest.NewRequest("DELETE", fmt.Sprintf("/api/user/uploads/%d", int(id)), nil)
	testutil.AddUserAuth(t, delReq, a, "100")
	testutil.Do(router, delReq)

	// Try to serve it.
	req := httptest.NewRequest("GET", fmt.Sprintf("/api/user/uploads/%d/file", int(id)), nil)
	testutil.AddUserAuth(t, req, a, "100")
	rec := testutil.Do(router, req)

	if rec.Code != 404 {
		t.Fatalf("status = %d, want 404 for deleted file", rec.Code)
	}
}
