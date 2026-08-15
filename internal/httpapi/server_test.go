package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"winforge/internal/app"
	"winforge/internal/audit"
)

func TestServeHTTPRejectsNonLoopbackHost(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest(http.MethodGet, "http://attacker.example/", nil)
	req.Host = "attacker.example"
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestServeHTTPRejectsCrossOriginMutation(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8696/not-found", nil)
	req.Host = "127.0.0.1:8696"
	req.Header.Set("Origin", "https://attacker.example")
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusForbidden)
	}
}

func TestServeHTTPAllowsSameOriginMutation(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8696/not-found", nil)
	req.Host = "127.0.0.1:8696"
	req.Header.Set("Origin", "http://127.0.0.1:8696")
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)
	if res.Code == http.StatusForbidden {
		t.Fatalf("same-origin request was forbidden: %s", res.Body.String())
	}
}

func TestServeHTTPSetsSecurityHeaders(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/not-found", nil)
	res := httptest.NewRecorder()
	s.ServeHTTP(res, req)

	for _, header := range []string{"Content-Security-Policy", "X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy"} {
		if res.Header().Get(header) == "" {
			t.Errorf("security header %s was not set", header)
		}
	}
}

func TestPruneJobsRetainsRunningAndNewestCompleted(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	for i := 1; i <= maxJobs+2; i++ {
		id := fmt.Sprintf("job-%d", i)
		s.jobs[id] = &job{ID: id, Done: true}
	}
	runningID := fmt.Sprintf("job-%d", maxJobs+3)
	s.jobs[runningID] = &job{ID: runningID, Status: "running"}

	s.pruneJobs()
	if _, ok := s.jobs[runningID]; !ok {
		t.Fatal("running job was pruned")
	}
	if _, ok := s.jobs["job-1"]; ok {
		t.Fatal("oldest completed job was retained")
	}
	if _, ok := s.jobs[fmt.Sprintf("job-%d", maxJobs+2)]; !ok {
		t.Fatal("newest completed job was pruned")
	}
	if len(s.jobs) != maxJobs+1 {
		t.Fatalf("retained %d jobs, want %d completed plus one running", len(s.jobs), maxJobs)
	}
}

func TestJobLogIsBoundedAndSnapshotsDoNotShareLines(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	done := make(chan struct{})
	initial, err := s.startJob("test", "test", func(log func(string)) error {
		for i := 0; i < maxJobLines+5; i++ {
			log(fmt.Sprintf("line-%d", i))
		}
		close(done)
		return nil
	})
	if err != nil {
		t.Fatalf("startJob: %v", err)
	}
	<-done

	s.mu.Lock()
	live := cloneJob(s.jobs[initial.ID])
	s.mu.Unlock()
	if len(initial.Lines) != 0 {
		t.Fatalf("initial response mutated after return: %d lines", len(initial.Lines))
	}
	if len(live.Lines) != maxJobLines || live.LinesDropped != 5 {
		t.Fatalf("bounded log = %d lines, %d dropped; want %d and 5", len(live.Lines), live.LinesDropped, maxJobLines)
	}
}

func TestJobLogHasAggregateByteBound(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	done := make(chan struct{})
	lineCount := maxJobLogBytes/maxJobLineBytes + 5
	initial, err := s.startJob("test", "test", func(log func(string)) error {
		for i := 0; i < lineCount; i++ {
			log(strings.Repeat("x", maxJobLineBytes))
		}
		close(done)
		return nil
	})
	if err != nil {
		t.Fatalf("startJob: %v", err)
	}
	<-done

	s.mu.Lock()
	live := cloneJob(s.jobs[initial.ID])
	bytesRetained := s.jobs[initial.ID].linesBytes
	s.mu.Unlock()
	if bytesRetained > maxJobLogBytes {
		t.Fatalf("job retained %d log bytes, limit is %d", bytesRetained, maxJobLogBytes)
	}
	if live.LinesDropped != 5 || len(live.Lines) != maxJobLogBytes/maxJobLineBytes {
		t.Fatalf("bounded log = %d lines, %d dropped", len(live.Lines), live.LinesDropped)
	}
}

func TestJobLogTruncationPreservesUTF8(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	done := make(chan struct{})
	initial, err := s.startJob("test", "test", func(log func(string)) error {
		// The byte limit lands inside the four-byte rune unless truncation backs
		// up to a valid UTF-8 boundary.
		log(strings.Repeat("x", maxJobLineBytes-len("…")-1) + "😀" + strings.Repeat("y", 10))
		close(done)
		return nil
	})
	if err != nil {
		t.Fatalf("startJob: %v", err)
	}
	<-done

	s.mu.Lock()
	live := cloneJob(s.jobs[initial.ID])
	s.mu.Unlock()
	if len(live.Lines) != 1 {
		t.Fatalf("job has %d lines, want 1", len(live.Lines))
	}
	if !utf8.ValidString(live.Lines[0]) {
		t.Fatalf("truncated line is invalid UTF-8: %q", live.Lines[0])
	}
	if len(live.Lines[0]) > maxJobLineBytes {
		t.Fatalf("truncated line has %d bytes, limit is %d", len(live.Lines[0]), maxJobLineBytes)
	}
	if !strings.HasSuffix(live.Lines[0], "…") {
		t.Fatalf("truncated line does not end in an ellipsis: %q", live.Lines[0])
	}
}

func TestStartJobRecoversPanicAndBoundsError(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	initial, err := s.startJob("test", "test", func(func(string)) error {
		panic(strings.Repeat("x", maxJobErrorBytes+100))
	})
	if err != nil {
		t.Fatalf("startJob: %v", err)
	}
	select {
	case <-initial.finished:
	case <-time.After(time.Second):
		t.Fatal("panicking job did not finish")
	}

	s.mu.Lock()
	live := cloneJob(s.jobs[initial.ID])
	s.mu.Unlock()
	if !live.Done || live.Status != "error" || !strings.Contains(live.Error, "job panicked") {
		t.Fatalf("panicking job = %+v", live)
	}
	if len(live.Error) > maxJobErrorBytes || !utf8.ValidString(live.Error) {
		t.Fatalf("bounded panic error has %d bytes and valid=%v", len(live.Error), utf8.ValidString(live.Error))
	}
}

func TestStartJobSerializesOperations(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	firstStarted := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondStarted := make(chan struct{})

	if _, err := s.startJob("first", "first", func(func(string)) error {
		close(firstStarted)
		<-releaseFirst
		return nil
	}); err != nil {
		t.Fatalf("start first job: %v", err)
	}
	<-firstStarted
	if _, err := s.startJob("second", "second", func(func(string)) error {
		close(secondStarted)
		return nil
	}); err != nil {
		t.Fatalf("start second job: %v", err)
	}

	select {
	case <-secondStarted:
		t.Fatal("second operation started while first operation was still running")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	select {
	case <-secondStarted:
	case <-time.After(time.Second):
		t.Fatal("second operation did not start after first operation finished")
	}
}

func TestStartJobRejectsWorkBeyondActiveLimitWithoutDiscardingJobs(t *testing.T) {
	s := &Server{jobs: map[string]*job{}}
	release := make(chan struct{})
	admitted := make([]*job, 0, maxActiveJobs)

	for i := 0; i < maxActiveJobs; i++ {
		j, err := s.startJob("test", fmt.Sprintf("job %d", i), func(func(string)) error {
			<-release
			return nil
		})
		if err != nil {
			t.Fatalf("startJob(%d): %v", i, err)
		}
		if j == nil {
			t.Fatalf("startJob(%d) returned a nil job", i)
		}
		admitted = append(admitted, j)
	}
	rejectedRan := make(chan struct{}, 1)
	if j, err := s.startJob("rejected", "rejected", func(func(string)) error {
		rejectedRan <- struct{}{}
		return nil
	}); !errors.Is(err, errJobQueueFull) || j != nil {
		t.Fatalf("over-limit startJob() = (%#v, %v), want nil and queue-full error", j, err)
	}
	select {
	case <-rejectedRan:
		t.Fatal("rejected job ran")
	default:
	}

	s.mu.Lock()
	if len(s.jobs) != maxActiveJobs {
		t.Errorf("job map contains %d jobs, want %d", len(s.jobs), maxActiveJobs)
	}
	if s.seq != maxActiveJobs {
		t.Errorf("job sequence = %d, want %d; rejected work consumed an ID", s.seq, maxActiveJobs)
	}
	for id, j := range s.jobs {
		if j.Done {
			t.Errorf("live job %s was unexpectedly marked done", id)
		}
	}
	s.mu.Unlock()

	close(release)
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for i, j := range admitted {
		select {
		case <-j.finished:
		case <-deadline.C:
			t.Fatalf("only %d admitted jobs finished before timeout", i)
		}
	}

	replacementDone := make(chan struct{})
	if _, err := s.startJob("replacement", "replacement", func(func(string)) error {
		close(replacementDone)
		return nil
	}); err != nil {
		t.Fatalf("start replacement job after capacity became available: %v", err)
	}
	select {
	case <-replacementDone:
	case <-time.After(time.Second):
		t.Fatal("replacement job did not run")
	}
}

func TestWriteJobAdmissionReportsQueueFull(t *testing.T) {
	res := httptest.NewRecorder()
	if writeJobAdmission(res, nil, fmt.Errorf("%w (%d active jobs)", errJobQueueFull, maxActiveJobs)) {
		t.Fatal("writeJobAdmission accepted a queue-full error")
	}
	if res.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusTooManyRequests)
	}
	if res.Header().Get("Retry-After") == "" {
		t.Fatal("queue-full response omitted Retry-After")
	}
}

func TestDecodeJSONRejectsInvalidRequestShapes(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"known":1,"extra":2}`},
		{name: "multiple values", body: `{"known":1} {"known":2}`},
		{name: "trailing junk", body: `{"known":1} trailing`},
		{name: "empty required body", body: ``},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var dst struct {
				Known int `json:"known"`
			}
			if err := decodeJSON(strings.NewReader(tt.body), &dst, false); err == nil {
				t.Fatal("decodeJSON accepted invalid request")
			}
		})
	}
}

func TestDecodeJSONAllowsEmptyOptionalBody(t *testing.T) {
	var dst struct {
		Value string `json:"value"`
	}
	if err := decodeJSON(strings.NewReader("  \n\t"), &dst, true); err != nil {
		t.Fatalf("decodeJSON rejected optional empty body: %v", err)
	}
}

func TestMutationHandlersRejectInvalidInputs(t *testing.T) {
	tests := []struct {
		path string
		body string
	}{
		{path: "/api/tweaks/apply", body: `{"id":"  "}`},
		{path: "/api/tweaks/undo", body: `{"id":"\t"}`},
		{path: "/api/apps/install", body: `{"id":""}`},
		{path: "/api/apps/install", body: `{"id":"--source.Evil"}`},
		{path: "/api/history/undo", body: `{}`},
		{path: "/api/dns/apply", body: `{"primary":"not-an-address"}`},
		{path: "/api/features/enable", body: `{"name":"NetFx3/Disable-Feature"}`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			s := New(nil)
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1"+tt.path, strings.NewReader(tt.body))
			res := httptest.NewRecorder()
			s.ServeHTTP(res, req)
			if res.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusBadRequest, res.Body.String())
			}
		})
	}
}

func TestOptionalJSONBodyRejectsMalformedInput(t *testing.T) {
	s := New(nil)
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/api/restore-point", strings.NewReader("{"))
	res := httptest.NewRecorder()

	s.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusBadRequest, res.Body.String())
	}
}

func TestElevatedISOEndpointsRejectBeforeUsingRequestPaths(t *testing.T) {
	for _, path := range []string{"/api/iso/editions", "/api/iso/build"} {
		t.Run(path, func(t *testing.T) {
			s := newServer(nil, true)
			req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1"+path, strings.NewReader(`{"source":"untrusted","output":"untrusted.iso"}`))
			res := httptest.NewRecorder()
			s.ServeHTTP(res, req)
			if res.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusForbidden, res.Body.String())
			}
			if len(s.jobs) != 0 {
				t.Fatalf("elevated request admitted %d ISO jobs", len(s.jobs))
			}
		})
	}
}

func TestElevatedHistoryReturnsEmptyEntriesAndWarning(t *testing.T) {
	logger := audit.NewLogger(t.TempDir())
	if err := logger.Append(audit.Entry{
		ID: "untrusted-entry", Timestamp: time.Now(), OperationType: "test", Success: true,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}

	s := newServer(&app.App{Logger: logger}, true)
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/history", nil)
	res := httptest.NewRecorder()
	s.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusOK, res.Body.String())
	}
	var response struct {
		Entries []audit.Entry `json:"entries"`
		Warning string        `json:"warning"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Entries == nil || len(response.Entries) != 0 {
		t.Fatalf("elevated entries = %#v, want a non-nil empty array", response.Entries)
	}
	if !strings.Contains(response.Warning, "not trusted") {
		t.Fatalf("elevated warning = %q", response.Warning)
	}
}

func TestHistoryReturnsValidEntriesWithCorruptionWarning(t *testing.T) {
	dir := t.TempDir()
	logger := audit.NewLogger(dir)
	now := time.Now()
	if err := logger.Append(audit.Entry{
		ID: "valid-entry", Timestamp: now, OperationType: "test", Success: true,
	}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	path := filepath.Join(dir, "operations-"+now.Format("2006-01-02")+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	if _, err := f.WriteString("{malformed\n"); err != nil {
		_ = f.Close()
		t.Fatalf("corrupt audit log: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close audit log: %v", err)
	}

	s := New(&app.App{Logger: logger})
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1/api/history", nil)
	res := httptest.NewRecorder()
	s.ServeHTTP(res, req)

	if res.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want %d; body = %s", res.Code, http.StatusPartialContent, res.Body.String())
	}
	body := res.Body.String()
	if !strings.Contains(body, `"id":"valid-entry"`) || !strings.Contains(body, `"warning":`) {
		t.Fatalf("partial history did not include valid entry and warning: %s", body)
	}
}
