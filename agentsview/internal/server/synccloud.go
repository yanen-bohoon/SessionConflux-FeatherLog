package server

import (
	"log"
	"net/http"

	"github.com/wesm/agentsview/internal/synccloud"

	sessionconflux "github.com/yanen-bohoon/session-conflux/pkg/sessionconflux"
)

// handleSyncCloudUpload streams an upload via SSE.
func (s *Server) handleSyncCloudUpload(w http.ResponseWriter, r *http.Request) {
	if s.db.ReadOnly() {
		writeError(w, http.StatusNotImplemented, "cloud sync unavailable in read-only mode")
		return
	}

	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err != nil {
		stream.SendJSON("error", map[string]string{"message": "loading state: " + err.Error()})
		return
	}

	stream.SendJSON("started", map[string]string{"operation": "upload"})

	stats, err := sessionconflux.Upload(scCfg, st)
	if err != nil {
		log.Printf("cloud sync upload: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	if err := st.Save(); err != nil {
		log.Printf("cloud sync save state: %v", err)
	}

	stream.SendJSON("done", stats)
}

// handleSyncCloudDownload streams a download via SSE.
func (s *Server) handleSyncCloudDownload(w http.ResponseWriter, r *http.Request) {
	if s.db.ReadOnly() {
		writeError(w, http.StatusNotImplemented, "cloud sync unavailable in read-only mode")
		return
	}

	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err != nil {
		stream.SendJSON("error", map[string]string{"message": "loading state: " + err.Error()})
		return
	}

	stream.SendJSON("started", map[string]string{"operation": "download"})

	stats, err := sessionconflux.Download(scCfg, st)
	if err != nil {
		log.Printf("cloud sync download: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	if err := st.Save(); err != nil {
		log.Printf("cloud sync save state: %v", err)
	}

	stream.SendJSON("done", stats)
}

// handleSyncCloudStatus returns the current sync state summary.
func (s *Server) handleSyncCloudStatus(w http.ResponseWriter, r *http.Request) {
	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "loading state: "+err.Error())
		return
	}
	info := sessionconflux.Status(st)
	writeJSON(w, http.StatusOK, info)
}

// handleSyncCloudTest verifies the transport connection.
func (s *Server) handleSyncCloudTest(w http.ResponseWriter, r *http.Request) {
	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	if err := sessionconflux.VerifyTransport(scCfg); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "connection successful",
	})
}
