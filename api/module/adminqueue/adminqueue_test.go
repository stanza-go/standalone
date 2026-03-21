package adminqueue_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stanza-go/framework/pkg/auth"
	fhttp "github.com/stanza-go/framework/pkg/http"
	"github.com/stanza-go/framework/pkg/queue"
	"github.com/stanza-go/framework/pkg/sqlite"
	"github.com/stanza-go/standalone/module/adminqueue"
	"github.com/stanza-go/standalone/testutil"
)

func setup(t *testing.T) (*fhttp.Router, *auth.Auth, *queue.Queue, *sqlite.DB) {
	t.Helper()

	db := testutil.SetupDB(t)
	a := testutil.NewAdminAuth()
	logger := testutil.NewLogger(t)

	q := queue.New(db, queue.WithLogger(logger))
	if err := q.Start(context.Background()); err != nil {
		t.Fatalf("queue start: %v", err)
	}
	t.Cleanup(func() { q.Stop(context.Background()) })

	router := testutil.NewRouter()
	api := router.Group("/api")
	admin := api.Group("/admin")
	admin.Use(a.RequireAuth())
	admin.Use(auth.RequireScope("admin"))
	adminqueue.Register(admin, q, db)

	return router, a, q, db
}

func TestStats_Empty(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/queue/stats", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	for _, key := range []string{"pending", "running", "completed", "failed", "dead", "cancelled"} {
		if _, ok := resp[key]; !ok {
			t.Errorf("missing key %q in stats response", key)
		}
	}
}

func TestStats_Unauthorized(t *testing.T) {
	t.Parallel()
	router, _, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/queue/stats", nil)
	rec := testutil.Do(router, req)

	if rec.Code != 401 {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestJobs_Empty(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs, ok := resp["jobs"].([]any)
	if !ok {
		t.Fatal("missing jobs in response")
	}
	if len(jobs) != 0 {
		t.Errorf("expected 0 jobs, got %d", len(jobs))
	}
}

func TestJobs_WithEnqueuedJob(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	_, err := q.Enqueue(context.Background(), "send_email", []byte(`{"to":"test@example.com"}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}

	job := jobs[0].(map[string]any)
	if job["type"] != "send_email" {
		t.Errorf("type = %v, want send_email", job["type"])
	}
	if job["status"] != "pending" {
		t.Errorf("status = %v, want pending", job["status"])
	}
}

func TestJobs_FilterByStatus(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	_, err := q.Enqueue(context.Background(), "send_email", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs?status=completed", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 0 {
		t.Errorf("expected 0 completed jobs, got %d", len(jobs))
	}
}

func TestCancel_PendingJob(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	jobID, err := q.Enqueue(context.Background(), "send_email", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/queue/jobs/%d/cancel", jobID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)
	if resp["ok"] != true {
		t.Error("expected ok: true")
	}
}

func TestCancel_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/queue/jobs/abc/cancel", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRetry_InvalidID(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/queue/jobs/abc/retry", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestRetry_NonExistentJob(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/queue/jobs/99999/retry", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestCancel_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, q, db := setup(t)

	jobID, err := q.Enqueue(context.Background(), "send_email", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/queue/jobs/%d/cancel", jobID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType, entityID string
	err = db.QueryRow("SELECT action, entity_type, entity_id FROM audit_log WHERE action = 'job.cancel'").
		Scan(&action, &entityType, &entityID)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "job" {
		t.Errorf("entity_type = %q, want job", entityType)
	}
	if entityID != fmt.Sprintf("%d", jobID) {
		t.Errorf("entity_id = %q, want %d", entityID, jobID)
	}
}

func TestJobs_WithPagination(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	// Enqueue 3 jobs.
	for i := 0; i < 3; i++ {
		_, err := q.Enqueue(context.Background(), fmt.Sprintf("job_%d", i), []byte(`{}`))
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	// Fetch with limit=2.
	req := httptest.NewRequest("GET", "/api/admin/queue/jobs?limit=2", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 2 {
		t.Errorf("got %d jobs, want 2", len(jobs))
	}

	total := int(resp["total"].(float64))
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func TestJobs_WithOffset(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	for i := 0; i < 3; i++ {
		_, err := q.Enqueue(context.Background(), fmt.Sprintf("offset_%d", i), []byte(`{}`))
		if err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs?limit=50&offset=2", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 1 {
		t.Errorf("got %d jobs with offset=2, want 1", len(jobs))
	}
}

func TestJobs_FilterByType(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	_, _ = q.Enqueue(context.Background(), "send_email", []byte(`{}`))
	_, _ = q.Enqueue(context.Background(), "process_image", []byte(`{}`))
	_, _ = q.Enqueue(context.Background(), "send_email", []byte(`{}`))

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs?type=send_email", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 2 {
		t.Errorf("got %d send_email jobs, want 2", len(jobs))
	}

	for _, j := range jobs {
		job := j.(map[string]any)
		if job["type"] != "send_email" {
			t.Errorf("type = %v, want send_email", job["type"])
		}
	}
}

func TestJobs_ResponseStructure(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	_, err := q.Enqueue(context.Background(), "test_job", []byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/queue/jobs", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) == 0 {
		t.Fatal("expected at least 1 job")
	}

	job := jobs[0].(map[string]any)
	for _, field := range []string{"id", "queue", "type", "payload", "status", "attempts", "max_attempts", "created_at", "updated_at"} {
		if _, ok := job[field]; !ok {
			t.Errorf("missing field %q in job response", field)
		}
	}

	if job["payload"] != `{"key":"value"}` {
		t.Errorf("payload = %v, want {\"key\":\"value\"}", job["payload"])
	}
}

func TestStats_WithJobs(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	_, err := q.Enqueue(context.Background(), "stat_job", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/admin/queue/stats", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, rec, &resp)

	pending := int(resp["pending"].(float64))
	if pending != 1 {
		t.Errorf("pending = %d, want 1", pending)
	}
}

func TestCancel_NonExistentJob(t *testing.T) {
	t.Parallel()
	router, a, _, _ := setup(t)

	req := httptest.NewRequest("POST", "/api/admin/queue/jobs/99999/cancel", nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400 for non-existent job\nbody: %s", rec.Code, rec.Body.String())
	}
}

func TestRetry_AuditLog(t *testing.T) {
	t.Parallel()
	router, a, q, db := setup(t)

	jobID, err := q.Enqueue(context.Background(), "retry_audit", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Mark the job as failed so we can retry it.
	_, _ = db.Exec("UPDATE _queue_jobs SET status = 'failed' WHERE id = ?", jobID)

	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/queue/jobs/%d/retry", jobID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	rec := testutil.Do(router, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200\nbody: %s", rec.Code, rec.Body.String())
	}

	var action, entityType, entityID string
	err = db.QueryRow("SELECT action, entity_type, entity_id FROM audit_log WHERE action = 'job.retry'").
		Scan(&action, &entityType, &entityID)
	if err != nil {
		t.Fatalf("audit log query: %v", err)
	}
	if entityType != "job" {
		t.Errorf("entity_type = %q, want job", entityType)
	}
	if entityID != fmt.Sprintf("%d", jobID) {
		t.Errorf("entity_id = %q, want %d", entityID, jobID)
	}
}

func TestCancel_VerifyJobStatus(t *testing.T) {
	t.Parallel()
	router, a, q, _ := setup(t)

	jobID, err := q.Enqueue(context.Background(), "cancel_verify", []byte(`{}`))
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Cancel the job.
	req := httptest.NewRequest("POST", fmt.Sprintf("/api/admin/queue/jobs/%d/cancel", jobID), nil)
	testutil.AddAdminAuth(t, req, a, "1")
	testutil.Do(router, req)

	// Verify via jobs list — cancelled filter.
	listReq := httptest.NewRequest("GET", "/api/admin/queue/jobs?status=cancelled", nil)
	testutil.AddAdminAuth(t, listReq, a, "1")
	listRec := testutil.Do(router, listReq)

	if listRec.Code != 200 {
		t.Fatalf("status = %d, want 200", listRec.Code)
	}

	var resp map[string]any
	testutil.DecodeJSON(t, listRec, &resp)

	jobs := resp["jobs"].([]any)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 cancelled job, got %d", len(jobs))
	}
	if jobs[0].(map[string]any)["status"] != "cancelled" {
		t.Errorf("status = %v, want cancelled", jobs[0].(map[string]any)["status"])
	}
}
