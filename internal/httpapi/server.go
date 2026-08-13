// Package httpapi serves the embedded web dashboard and its JSON API.
//
// The server binds to 127.0.0.1 by default: this is a system-control surface
// and must never be exposed to the network.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"winforge"
	"winforge/internal/app"
	"winforge/internal/appmanager"
	"winforge/internal/platform"
)

// installJob tracks an in-flight winget install for progress polling.
type installJob struct {
	ID     string
	Status string // "running" | "done" | "error"
	Lines  []string
	Done   bool
	Err    string
}

// Server implements http.Handler for the dashboard and API.
type Server struct {
	App  *app.App
	mux  *http.ServeMux
	mu   sync.Mutex
	jobs map[string]*installJob
	seq  int
}

// New creates the HTTP server.
func New(a *app.App) *Server {
	s := &Server{App: a, jobs: map[string]*installJob{}}
	s.mux = s.routes()
	return s
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/tweaks", s.handleListTweaks)
	mux.HandleFunc("POST /api/tweaks/apply", s.handleApplyTweak)
	mux.HandleFunc("POST /api/tweaks/undo", s.handleUndoTweak)
	mux.HandleFunc("GET /api/apps", s.handleListApps)
	mux.HandleFunc("POST /api/apps/install", s.handleInstall)
	mux.HandleFunc("GET /api/apps/jobs/{id}", s.handleJobStatus)
	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/history/undo", s.handleUndoEntry)

	// Static dashboard.
	webFS, err := fs.Sub(winforge.Assets, "web")
	if err != nil {
		panic(err) // embedded assets are part of the binary; this cannot fail
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	return mux
}

// ServeHTTP satisfies http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	info := platform.GetOSInfo()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":   app.Version,
		"os":        info,
		"elevated":  platform.IsElevated(),
		"dataDir":   s.App.DataDir,
		"tweakCount": len(s.App.Tweaks),
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Health(0))
}

// tweakDTO is the API shape for a tweak with its applied state.
type tweakDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Risk        string `json:"risk"`
	Reversible  bool   `json:"reversible"`
	Applied     bool   `json:"applied"`
}

func (s *Server) handleListTweaks(w http.ResponseWriter, _ *http.Request) {
	applied := s.App.AppliedMap()
	out := make([]tweakDTO, 0, len(s.App.Tweaks))
	for _, t := range s.App.Tweaks {
		out = append(out, tweakDTO{
			ID: t.ID, Name: t.Name, Category: t.Category,
			Description: t.Description, Risk: string(t.Risk),
			Reversible: t.Reversible, Applied: applied[t.ID],
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleApplyTweak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string `json:"id"`
		DryRun bool   `json:"dryRun"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.App.Apply(req.ID, req.DryRun)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleUndoTweak(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	res, err := s.App.Undo(req.ID)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleListApps(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.App.Apps)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	job := s.startInstall(req.ID)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) startInstall(id string) *installJob {
	s.mu.Lock()
	s.seq++
	job := &installJob{ID: id, Status: "running"}
	s.jobs[id] = job
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		res, err := s.App.Packages.Install(ctx, id, func(p appmanager.Progress) {
			s.mu.Lock()
			job.Lines = append(job.Lines, p.Line)
			s.mu.Unlock()
		})

		s.mu.Lock()
		defer s.mu.Unlock()
		job.Done = true
		if err != nil {
			job.Status = "error"
			job.Err = err.Error()
		} else if res != nil && !res.Success {
			job.Status = "error"
			if res.Error != nil {
				job.Err = res.Error.Error()
			} else {
				job.Err = "winget reported failure"
			}
		} else {
			job.Status = "done"
		}
	}()

	return job
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	job, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("job %q not found", id))
		return
	}
	s.mu.Lock()
	out := *job
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleHistory(w http.ResponseWriter, _ *http.Request) {
	entries, err := s.App.History()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleUndoEntry(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if err := s.App.UndoEntry(req.ID); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
