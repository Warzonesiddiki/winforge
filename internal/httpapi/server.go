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
	"winforge/internal/maintenance"
	"winforge/internal/platform"
)

// job tracks an in-flight async operation (winget install, maintenance fix,
// DISM feature change) for progress polling.
type job struct {
	ID     string   `json:"id"`
	Kind   string   `json:"kind"`
	Status string   `json:"status"` // "running" | "done" | "error"
	Lines  []string `json:"lines,omitempty"`
	Done   bool     `json:"done"`
	Error  string   `json:"error,omitempty"`
}

// Server implements http.Handler for the dashboard and API.
type Server struct {
	App  *app.App
	mux  *http.ServeMux
	mu   sync.Mutex
	jobs map[string]*job
	seq  int
}

// New creates the HTTP server.
func New(a *app.App) *Server {
	s := &Server{App: a, jobs: map[string]*job{}}
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

	mux.HandleFunc("GET /api/plugins", s.handleListPlugins)
	mux.HandleFunc("GET /api/apps", s.handleListApps)
	mux.HandleFunc("POST /api/apps/install", s.handleInstall)
	mux.HandleFunc("GET /api/jobs/{id}", s.handleJobStatus)

	mux.HandleFunc("GET /api/history", s.handleHistory)
	mux.HandleFunc("POST /api/history/undo", s.handleUndoEntry)

	mux.HandleFunc("POST /api/restore-point", s.handleRestorePoint)

	mux.HandleFunc("POST /api/maintenance/reset-windows-update", s.handleResetWindowsUpdate)
	mux.HandleFunc("POST /api/maintenance/repair-image", s.handleRepairImage)
	mux.HandleFunc("POST /api/maintenance/flush-dns", s.handleFlushDNS)
	mux.HandleFunc("POST /api/maintenance/network-reset", s.handleNetworkReset)

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

// startJob launches fn in a goroutine, streaming its log lines into the job.
func (s *Server) startJob(kind, name string, fn func(log func(string)) error) *job {
	s.mu.Lock()
	s.seq++
	j := &job{ID: fmt.Sprintf("job-%d", s.seq), Kind: kind, Status: "running"}
	s.jobs[j.ID] = j
	s.mu.Unlock()

	go func() {
		err := fn(func(line string) {
			s.mu.Lock()
			j.Lines = append(j.Lines, line)
			s.mu.Unlock()
		})
		s.mu.Lock()
		defer s.mu.Unlock()
		j.Done = true
		if err != nil {
			j.Status = "error"
			j.Error = err.Error()
		} else {
			j.Status = "done"
		}
	}()

	return j
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	info := platform.GetOSInfo()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":     app.Version,
		"os":          info,
		"elevated":    platform.IsElevated(),
		"dataDir":     s.App.DataDir,
		"tweakCount":  len(s.App.Tweaks),
		"pluginCount": len(s.App.Plugins),
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}

	j := s.startJob("install", req.ID, func(log func(string)) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		res, err := s.App.Packages.Install(ctx, req.ID, func(p appmanager.Progress) {
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
	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	j, ok := s.jobs[id]
	s.mu.Unlock()
	if !ok {
		writeErr(w, http.StatusNotFound, fmt.Errorf("job %q not found", id))
		return
	}
	s.mu.Lock()
	out := *j
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

func (s *Server) handleRestorePoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Description == "" {
		req.Description = "WinForge restore point"
	}
	info, err := s.App.CreateRestorePoint(req.Description)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (s *Server) handleResetWindowsUpdate(w http.ResponseWriter, _ *http.Request) {
	j := s.startJob("reset-windows-update", "Windows Update", func(log func(string)) error {
		return maintenance.ResetWindowsUpdate(log)
	})
	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleRepairImage(w http.ResponseWriter, _ *http.Request) {
	j := s.startJob("repair-image", "DISM", func(log func(string)) error {
		return maintenance.RepairImage(log)
	})
	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleFlushDNS(w http.ResponseWriter, _ *http.Request) {
	j := s.startJob("flush-dns", "DNS", func(log func(string)) error {
		return maintenance.FlushDNS()
	})
	writeJSON(w, http.StatusAccepted, j)
}

func (s *Server) handleNetworkReset(w http.ResponseWriter, _ *http.Request) {
	j := s.startJob("network-reset", "Network", func(log func(string)) error {
		return maintenance.NetworkReset(log)
	})
	writeJSON(w, http.StatusAccepted, j)
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	var err error
	if req.Adapter != "" {
		err = maintenance.SetDns(req.Adapter, primary, secondary)
	} else {
		err = maintenance.SetDnsOnAll(primary, secondary)
	}
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("feature name is required"))
		return
	}
	verb := "disable"
	if enable {
		verb = "enable"
	}
	j := s.startJob("feature-"+verb, req.Name, func(log func(string)) error {
		if enable {
			return maintenance.EnableFeature(req.Name, log)
		}
		return maintenance.DisableFeature(req.Name, log)
	})
	writeJSON(w, http.StatusAccepted, j)
}
