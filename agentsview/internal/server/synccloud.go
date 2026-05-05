package server

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/wesm/agentsview/internal/parser"
	"github.com/wesm/agentsview/internal/synccloud"

	sessionconflux "github.com/yanen-bohoon/session-conflux/pkg/sessionconflux"
	confluxsync "github.com/yanen-bohoon/session-conflux/pkg/sync"
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

	discoveredFiles := s.engine.ChangedFiles(time.Time{})
	var files []confluxsync.SyncFile
	for _, f := range discoveredFiles {
		info, err := os.Stat(f.Path)
		if err != nil {
			continue
		}
		files = append(files, confluxsync.FileFromDiscovered(f.Path, string(f.Agent), info.Size(), info.ModTime().UnixNano()))
	}

	stats, err := sessionconflux.Upload(scCfg, st, files)
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

	findAgentDir := func(agent string) string {
		dirs := s.cfg.AgentDirs[parser.AgentType(agent)]
		if len(dirs) > 0 {
			return dirs[0]
		}
		return ""
	}


	stats, err := sessionconflux.Download(scCfg, st, findAgentDir)
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
