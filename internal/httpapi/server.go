// Package httpapi serves the embedded web dashboard and its JSON API.
//
// The server binds to 127.0.0.1 by default: this is a system-control surface
// and must never be exposed to the network.
package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"winforge"
	"winforge/internal/app"
	"winforge/internal/appmanager"
	"winforge/internal/audit"
	"winforge/internal/isobuilder"
	"winforge/internal/maintenance"
	"winforge/internal/platform"
	"winforge/internal/restorepoint"
	"winforge/internal/tweak"
	"winforge/internal/updater"
)

// Completed and live jobs have separate bounds. Live queued/running jobs are
// never discarded; new work is rejected once admission is full. Older finished
// entries are pruned when the completed-job retention limit is exceeded.
const (
	maxJobs             = 100
	maxActiveJobs       = 16
	maxJobLines         = 1000
	maxJobLineBytes     = 4096
	maxJobLogBytes      = 256 << 10
	maxJobErrorBytes    = 16 << 10
	maxRequestBodyBytes = 1 << 20
)

var (
	errJobQueueFull    = errors.New("job queue is full")
	errTooManyRequests = errors.New("too many requests; slow down and retry shortly")
)

// job tracks an in-flight async operation (winget install, maintenance fix,
// DISM feature change) for progress polling.
type job struct {
	ID           string   `json:"id"`
	Kind         string   `json:"kind"`
	Status       string   `json:"status"` // "queued" | "running" | "done" | "error"
	Lines        []string `json:"lines,omitempty"`
	LinesDropped int      `json:"linesDropped,omitempty"`
	Done         bool     `json:"done"`
	Error        string   `json:"error,omitempty"`
	linesBytes   int
	finished     chan struct{}
}

// Server implements http.Handler for the dashboard and API.
type Server struct {
	App             *app.App
	mux             *http.ServeMux
	elevated        bool
	sessionToken    string
	mu              sync.Mutex
	mutationMu      sync.Mutex
	jobs            map[string]*job
	seq             int
	mutationLimiter *rateLimiter
}

// New creates the HTTP server.
func New(a *app.App) *Server {
	elevated := a != nil && a.Elevated()
	return newServer(a, elevated)
}

func newServer(a *app.App, elevated bool) *Server {
	token, err := newSessionToken()
	if err != nil {
		panic("httpapi: cannot generate session token: " + err.Error())
	}
	s := &Server{
		App:             a,
		elevated:        elevated,
		sessionToken:    token,
		jobs:            map[string]*job{},
		mutationLimiter: newRateLimiter(defaultMutationRate, defaultMutationBurst),
	}
	s.mux = s.routes()
	return s
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/session-token", s.handleSessionToken)
	mux.HandleFunc("GET /api/tweaks", s.handleListTweaks)
	mux.HandleFunc("POST /api/tweaks/apply", s.handleApplyTweak)
	mux.HandleFunc("POST /api/tweaks/undo", s.handleUndoTweak)

	mux.HandleFunc("GET /api/plugins", s.handleListPlugins)
	mux.HandleFunc("GET /api/apps", s.handleListApps)
	mux.HandleFunc("POST /api/apps/install", s.handleInstall)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleJobStatus)

	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/history/undo", s.handleUndoEntry)

	mux.HandleFunc("GET /api/restore-points", s.handleListRestorePoints)
	mux.HandleFunc("POST /api/restore-point", s.handleRestorePoint)

	mux.HandleFunc("POST /api/maintenance/reset-windows-update", s.handleResetWindowsUpdate)
	mux.HandleFunc("POST /api/maintenance/repair-image", s.handleRepairImage)
	mux.HandleFunc("POST /api/maintenance/flush-dns", s.handleFlushDNS)
	mux.HandleFunc("POST /api/maintenance/network-reset", s.handleNetworkReset)
	mux.HandleFunc("POST /api/maintenance/run", s.handleRunMaintenance)
	mux.HandleFunc("POST /api/maintenance/schedule", s.handleScheduleMaintenance)
	mux.HandleFunc("DELETE /api/maintenance/schedule", s.handleUnscheduleMaintenance)

	mux.HandleFunc("GET /api/bloatware", s.handleBloatware)

	mux.HandleFunc("POST /api/iso/editions", s.handleISOEditions)
	mux.HandleFunc("POST /api/iso/build", s.handleISOBuild)

	mux.HandleFunc("POST /api/updates/search", s.handleUpdatesSearch)
	mux.HandleFunc("POST /api/updates/install", s.handleUpdatesInstall)

	mux.HandleFunc("GET /api/dns/presets", s.handleDnsPresets)
	mux.HandleFunc("POST /api/dns/apply", s.handleDnsApply)

	mux.HandleFunc("POST /api/features/enable", s.enableFeatureHandler)
	mux.HandleFunc("POST /api/features/disable", s.disableFeatureHandler)

	// Static dashboard.
	webFS, err := fs.Sub(winforge.Assets, "web")
	if err != nil {
		panic(err) // embedded assets are part of the binary; this cannot fail
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}

// ServeHTTP satisfies http.Handler. The dashboard is a privileged local
// control surface, so reject DNS-rebinding hosts and cross-origin mutations
// even if a caller accidentally binds the server beyond loopback.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'none'")

	if !isLoopbackAuthority(r.Host) {
		writeErr(w, http.StatusForbidden, fmt.Errorf("request Host must be loopback"))
		return
	}
	if isMutation(r.Method) {
		if !isSameOrigin(r) {
			writeErr(w, http.StatusForbidden, fmt.Errorf("cross-origin mutation rejected"))
			return
		}
		// ADR-002: mutating requests must carry the per-instance session
		// token, closing the "any local process can POST to loopback" gap
		// that same-origin leaves open for non-browser clients.
		provided := r.Header.Get(sessionTokenHeader)
		if provided == "" ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(s.sessionToken)) != 1 {
			writeErr(w, http.StatusUnauthorized, errInvalidToken)
			return
		}
		// Defense-in-depth: throttle a flood of authenticated mutations.
		if !s.limitMutation(w, r) {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)
	}
	s.mux.ServeHTTP(w, r)
}

// handleSessionToken returns the per-instance session token a same-origin
// client must echo in X-WinForge-Token on mutations. It is a read endpoint and
// is itself reachable only from loopback (and same-origin when called from a
// browser). The embedded dashboard fetches it on load; the Next.js bridge
// fetches it through the /engine proxy.
func (s *Server) handleSessionToken(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"token": s.sessionToken})
}

func isMutation(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isLoopbackAuthority(authority string) bool {
	host, _, err := splitAuthority(authority)
	if err != nil {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	addr, err := netip.ParseAddr(host)
	return err == nil && addr.IsLoopback()
}

func splitAuthority(authority string) (host, port string, err error) {
	if authority == "" {
		return "", "", fmt.Errorf("empty authority")
	}
	if strings.HasPrefix(authority, "[") && strings.HasSuffix(authority, "]") {
		return strings.TrimSuffix(strings.TrimPrefix(authority, "["), "]"), "", nil
	}
	if h, p, splitErr := net.SplitHostPort(authority); splitErr == nil {
		return h, p, nil
	}
	if strings.Contains(authority, ":") {
		return "", "", fmt.Errorf("invalid authority %q", authority)
	}
	return authority, "", nil
}

func decodeJSON(body io.Reader, dst any, optional bool) error {
	dec := json.NewDecoder(body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		if optional && errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("invalid request: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("invalid request: body contains multiple JSON values")
		}
		return fmt.Errorf("invalid request after first JSON value: %w", err)
	}
	return nil
}

func isSameOrigin(r *http.Request) bool {
	if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		// Non-browser clients generally omit Origin; Host enforcement still
		// prevents DNS rebinding and browser requests carry Origin/Sec-Fetch-Site.
		return true
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" ||
		(u.Scheme != "http" && u.Scheme != "https") {
		return false
	}
	requestHost, requestPort, err := splitAuthority(r.Host)
	if err != nil {
		return false
	}
	originHost, originPort, err := splitAuthority(u.Host)
	if err != nil || !strings.EqualFold(requestHost, originHost) {
		return false
	}
	requestScheme := "http"
	if r.TLS != nil {
		requestScheme = "https"
	}
	if u.Scheme != requestScheme {
		return false
	}
	if requestPort == "" {
		requestPort = map[string]string{"http": "80", "https": "443"}[requestScheme]
	}
	if originPort == "" {
		originPort = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	return requestPort == originPort
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) withMutation(fn func()) {
	s.mutationMu.Lock()
	defer s.mutationMu.Unlock()
	fn()
}

func truncateUTF8(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	text = text[:limit-len("…")]
	for !utf8.ValidString(text) {
		text = text[:len(text)-1]
	}
	return text + "…"
}

// startJob admits fn to the bounded mutation queue and streams a bounded log
// into its job record. A full queue is rejected before an ID or goroutine is
// allocated; already-admitted queued/running jobs are never discarded.
func (s *Server) startJob(kind, name string, fn func(log func(string)) error) (*job, error) {
	s.mu.Lock()
	active := 0
	for _, existing := range s.jobs {
		if !existing.Done {
			active++
		}
	}
	if active >= maxActiveJobs {
		s.mu.Unlock()
		return nil, fmt.Errorf("%w (%d active jobs)", errJobQueueFull, maxActiveJobs)
	}
	s.seq++
	j := &job{ID: fmt.Sprintf("job-%d", s.seq), Kind: kind, Status: "queued", finished: make(chan struct{})}
	s.jobs[j.ID] = j
	s.pruneJobs()
	initial := cloneJob(j)
	s.mu.Unlock()

	go func() {
		s.mutationMu.Lock()
		s.mu.Lock()
		j.Status = "running"
		s.mu.Unlock()
		err := func() (jobErr error) {
			defer s.mutationMu.Unlock()
			defer func() {
				if recovered := recover(); recovered != nil {
					jobErr = fmt.Errorf("job panicked: %v", recovered)
				}
			}()
			return fn(func(line string) {
				s.mu.Lock()
				line = truncateUTF8(line, maxJobLineBytes)
				for len(j.Lines) > 0 && (len(j.Lines) >= maxJobLines || j.linesBytes+len(line) > maxJobLogBytes) {
					j.linesBytes -= len(j.Lines[0])
					copy(j.Lines, j.Lines[1:])
					j.Lines = j.Lines[:len(j.Lines)-1]
					j.LinesDropped++
				}
				j.Lines = append(j.Lines, line)
				j.linesBytes += len(line)
				s.mu.Unlock()
			})
		}()
		s.mu.Lock()
		defer s.mu.Unlock()
		j.Done = true
		if err != nil {
			j.Status = "error"
			j.Error = truncateUTF8(err.Error(), maxJobErrorBytes)
		} else {
			j.Status = "done"
		}
		close(j.finished)
		s.pruneJobs()
	}()

	return initial, nil
}

func writeJobAdmission(w http.ResponseWriter, j *job, err error) bool {
	if err != nil {
		w.Header().Set("Retry-After", "1")
		writeErr(w, http.StatusTooManyRequests, err)
		return false
	}
	writeJSON(w, http.StatusAccepted, j)
	return true
}

// cloneJob returns a snapshot that does not share its Lines backing array with
// the live job. Caller must hold s.mu when the job can still be running.
func cloneJob(j *job) *job {
	out := *j
	out.Lines = append([]string(nil), j.Lines...)
	return &out
}

// pruneJobs removes the oldest completed/errored jobs when the count exceeds
// maxJobs. Caller must hold s.mu.
func (s *Server) pruneJobs() {
	// Collect IDs of finished jobs sorted by sequence (embedded in the ID
	// string "job-N") so we can drop the oldest.
	type entry struct {
		id  string
		seq int
	}
	var finished []entry
	for id, j := range s.jobs {
		if j.Done {
			var n int
			fmt.Sscanf(id, "job-%d", &n)
			finished = append(finished, entry{id: id, seq: n})
		}
	}
	// Sort ascending by seq (oldest first).
	for i := 1; i < len(finished); i++ {
		for j := i; j > 0 && finished[j].seq < finished[j-1].seq; j-- {
			finished[j], finished[j-1] = finished[j-1], finished[j]
		}
	}
	// Remove the oldest entries only when the number of finished jobs exceeds
	// the retention limit. Never evict a just-finished result merely because
	// many other jobs are still running.
	excess := len(finished) - maxJobs
	for i := 0; i < len(finished) && excess > 0; i++ {
		delete(s.jobs, finished[i].id)
		excess--
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	info := platform.GetOSInfo()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":        app.Version,
		"os":             info,
		"elevated":       s.elevated,
		"dataDir":        s.App.DataDir,
		"tweakCount":     len(s.App.Tweaks),
		"pluginCount":    len(s.App.Plugins),
		"bloatwareCount": s.App.BloatwareCount(),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Health(s.App.BloatwareCount()))
}

func (s *Server) handleBloatware(w http.ResponseWriter, _ *http.Request) {
	list := s.App.Bloatware()
	if list == nil {
		list = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(list), "apps": list})
}

// tweakDTO is the API shape for a tweak with its applied state.
type tweakDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
	Reversible  bool   `json:"reversible"`
	Verifiable  bool   `json:"verifiable"`
	Applied     bool   `json:"applied"`
}

func (s *Server) handleListTweaks(w http.ResponseWriter, _ *http.Request) {
	applied := s.App.AppliedMap()
	out := make([]tweakDTO, 0, len(s.App.Tweaks))
	for _, t := range s.App.Tweaks {
		out = append(out, tweakDTO{
			ID: t.ID, Name: t.Name, Category: t.Category,
			Description: t.Description, Risk: string(t.Risk),
			Reversible: t.Reversible, Verifiable: tweak.CanVerify(t), Applied: applied[t.ID],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleApplyTweak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		DryRun bool   `json:"dryRun"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, errors.New("tweak id is required"))
		return
	}
	var res tweak.Result
	var err error
	s.withMutation(func() {
		res, err = s.App.Apply(req.ID, req.DryRun)
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if failure := res.Failure(); failure != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": failure.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUndoTweak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, errors.New("tweak id is required"))
		return
	}
	var res tweak.Result
	var err error
	s.withMutation(func() {
		res, err = s.App.Undo(req.ID)
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	if failure := res.Failure(); failure != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": failure.Error(), "result": res})
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// pluginDTO is the API shape for an installed plugin.
type pluginDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
	Author      string `json:"author,omitempty"`
	Dir         string `json:"dir"`
	TweakCount  int    `json:"tweakCount"`
}

func (s *Server) handleListPlugins(w http.ResponseWriter, _ *http.Request) {
	out := make([]pluginDTO, 0, len(s.App.Plugins))
	for _, p := range s.App.Plugins {
		out = append(out, pluginDTO{
			ID:          p.ID,
			Name:        p.Name,
			Version:     p.Version,
			Description: p.Description,
			Author:      p.Author,
			Dir:         p.Dir,
			TweakCount:  len(p.Tweaks),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleListApps(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Apps)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, errors.New("package id is required"))
		return
	}
	if err := appmanager.ValidatePackageID(req.ID); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	j, err := s.startJob("install", req.ID, func(log func(string)) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		res, err := s.App.InstallPackage(ctx, req.ID, func(p appmanager.Progress) {
			if p.Line != "" {
				log(p.Line)
			}
		})
		if err != nil {
			return err
		}
		if !res.Success {
			return fmt.Errorf("winget reported failure")
		}
		return nil
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	j, ok := s.jobs[id]
	if ok {
		out := cloneJob(j)
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, out)
		return
	}
	s.mu.Unlock()
	writeErr(w, http.StatusNotFound, fmt.Errorf("job %q not found", id))
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	if s.elevated {
		writeJSON(w, http.StatusOK, map[string]any{
			"entries": []audit.Entry{},
			"warning": "History is unavailable while WinForge is elevated because user-profile audit files are not trusted for administrator operations.",
		})
		return
	}
	entries, err := s.App.History()
	if err != nil {
		if len(entries) == 0 {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusPartialContent, map[string]any{
			"entries": entries,
			"warning": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleUndoEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		writeErr(w, http.StatusBadRequest, errors.New("operation id is required"))
		return
	}
	var err error
	s.withMutation(func() {
		err = s.App.UndoEntry(req.ID)
	})
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleListRestorePoints(w http.ResponseWriter, _ *http.Request) {
	points, err := s.App.ListRestorePoints()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if points == nil {
		points = []restorepoint.Info{}
	}
	writeJSON(w, http.StatusOK, points)
}

func (s *Server) handleRestorePoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := decodeJSON(r.Body, &req, true); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Description == "" {
		req.Description = "WinForge restore point"
	}
	var info restorepoint.Info
	var err error
	s.withMutation(func() {
		info, err = s.App.CreateRestorePoint(req.Description)
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleResetWindowsUpdate(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("reset-windows-update", "Windows Update", func(log func(string)) error {
		s.App.EnsureRestorePoint("WinForge: reset Windows Update")
		return maintenance.ResetWindowsUpdate(log)
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleRepairImage(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("repair-image", "DISM", func(log func(string)) error {
		s.App.EnsureRestorePoint("WinForge: repair system image")
		return maintenance.RepairImage(log)
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleFlushDNS(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("flush-dns", "DNS", func(log func(string)) error {
		return maintenance.FlushDNS()
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleNetworkReset(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("network-reset", "Network", func(log func(string)) error {
		s.App.EnsureRestorePoint("WinForge: reset network")
		return maintenance.NetworkReset(log)
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleRunMaintenance(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("run-maintenance", "Maintenance", func(log func(string)) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		sum := s.App.RunMaintenance(ctx, log)
		if len(sum.TweakErrors) > 0 {
			return fmt.Errorf("maintenance: %d tweak(s) failed", len(sum.TweakErrors))
		}
		if sum.AppError != "" {
			return fmt.Errorf("maintenance: app update error: %s", sum.AppError)
		}
		if sum.AuditError != "" {
			return fmt.Errorf("maintenance completed, but recording its result failed: %s", sum.AuditError)
		}
		return nil
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleScheduleMaintenance(w http.ResponseWriter, _ *http.Request) {
	var err error
	s.withMutation(func() {
		err = s.App.ScheduleMaintenance()
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"task": app.MaintenanceTaskName})
}

func (s *Server) handleUnscheduleMaintenance(w http.ResponseWriter, _ *http.Request) {
	var err error
	s.withMutation(func() {
		err = s.App.UnscheduleMaintenance()
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleISOEditions(w http.ResponseWriter, r *http.Request) {
	if s.elevated {
		writeErr(w, http.StatusForbidden, errors.New("ISO inspection is disabled while WinForge is elevated"))
		return
	}
	var req struct {
		Source string `json:"source"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Source == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("source is required"))
		return
	}
	editions, err := isobuilder.ListEditions(req.Source)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if editions == nil {
		editions = []isobuilder.Edition{}
	}
	writeJSON(w, http.StatusOK, editions)
}

func (s *Server) handleISOBuild(w http.ResponseWriter, r *http.Request) {
	if s.elevated {
		writeErr(w, http.StatusForbidden, errors.New("ISO building is disabled while WinForge is elevated"))
		return
	}
	var req struct {
		Source   string   `json:"source"`
		Output   string   `json:"output"`
		Label    string   `json:"label"`
		Editions []string `json:"editions"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	opts := isobuilder.Options{
		SourceDir: req.Source,
		OutputISO: req.Output,
		Label:     req.Label,
		Editions:  req.Editions,
	}
	name := req.Output
	if name == "" {
		name = "Windows ISO"
	}
	j, err := s.startJob("build-iso", name, func(log func(string)) error {
		opts.Log = log
		res, err := isobuilder.Build(opts)
		if err != nil {
			return err
		}
		log("ISO built: " + res.ISO)
		return nil
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleUpdatesSearch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Installed bool `json:"installed"`
	}
	if err := decodeJSON(r.Body, &req, true); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	updates, err := updater.Search(req.Installed)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if updates == nil {
		updates = []updater.Update{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"updates": updates})
}

func (s *Server) handleUpdatesInstall(w http.ResponseWriter, _ *http.Request) {
	j, err := s.startJob("updates-install", "Windows Update", func(log func(string)) error {
		s.App.EnsureRestorePoint("WinForge: install Windows updates")
		res, err := updater.InstallAll()
		if err != nil {
			return err
		}
		log("result: " + res.ResultCode.String())
		if res.RebootRequired {
			log("reboot required")
		}
		if res.ResultCode != updater.ResultSucceeded && res.ResultCode != updater.ResultSucceededWithErrors {
			return fmt.Errorf("install result: %s", res.ResultCode)
		}
		return nil
	})
	writeJobAdmission(w, j, err)
}

func (s *Server) handleDnsPresets(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.DnsPresets)
}

func (s *Server) handleDnsApply(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Profile   string `json:"profile"`
		Primary   string `json:"primary"`
		Secondary string `json:"secondary"`
		Adapter   string `json:"adapter"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	primary, secondary := req.Primary, req.Secondary
	if req.Profile != "" {
		for _, p := range s.App.DnsPresets {
			if p.Profile == req.Profile {
				primary, secondary = p.Primary, p.Secondary
				break
			}
		}
	}
	if primary == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("no DNS server specified"))
		return
	}
	var validationErr error
	if req.Adapter != "" {
		validationErr = maintenance.ValidateDnsSettings(req.Adapter, primary, secondary)
	} else {
		validationErr = maintenance.ValidateDnsServers(primary, secondary)
	}
	if validationErr != nil {
		writeErr(w, http.StatusBadRequest, validationErr)
		return
	}

	var err error
	s.withMutation(func() {
		s.App.EnsureRestorePoint("WinForge: change DNS settings")
		if req.Adapter != "" {
			err = maintenance.SetDns(req.Adapter, primary, secondary)
		} else {
			err = maintenance.SetDnsOnAll(primary, secondary)
		}
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) enableFeatureHandler(w http.ResponseWriter, r *http.Request) {
	s.featureHandler(w, r, true)
}

func (s *Server) disableFeatureHandler(w http.ResponseWriter, r *http.Request) {
	s.featureHandler(w, r, false)
}

func (s *Server) featureHandler(w http.ResponseWriter, r *http.Request, enable bool) {
	var req struct {
		Name string `json:"name"`
	}
	if err := decodeJSON(r.Body, &req, false); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("feature name is required"))
		return
	}
	if err := maintenance.ValidateFeatureName(req.Name); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	verb := "disable"
	if enable {
		verb = "enable"
	}
	j, err := s.startJob("feature-"+verb, req.Name, func(log func(string)) error {
		s.App.EnsureRestorePoint("WinForge: " + verb + " Windows feature " + req.Name)
		if enable {
			return maintenance.EnableFeature(req.Name, log)
		}
		return maintenance.DisableFeature(req.Name, log)
	})
	writeJobAdmission(w, j, err)
}
