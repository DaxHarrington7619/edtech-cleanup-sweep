package cleanupcron

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCronCreateRetriesAndPreservesRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/cron/create" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") == "" {
			t.Fatal("missing request headers")
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != `{"cron_expr":"15 2 * * *","task":"https://worker.example.edu/cleanup"}` {
			t.Fatalf("unexpected body: %s", body)
		}
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"job_id":"job_482"},"error":null,"metadata":{}}`))
	}))
	defer server.Close()

	client := newClient(server.URL, "test-key", server.Client())
	client.sleep = func(time.Duration) {}
	jobID, err := client.CronCreate(context.Background(), "15 2 * * *", "https://worker.example.edu/cleanup")
	if err != nil {
		t.Fatal(err)
	}
	if jobID != "job_482" || requests != 2 {
		t.Fatalf("got job %q after %d requests", jobID, requests)
	}
	if strings.TrimSpace(jobID) == "" {
		t.Fatal("job id is empty")
	}
}
