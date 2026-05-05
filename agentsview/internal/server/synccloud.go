package server

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/wesm/agentsview/internal/parser"
	"github.com/wesm/agentsview/internal/synccloud"

	confluxsync "github.com/yanen-bohoon/session-conflux/pkg/sync"
	confluxtransport "github.com/yanen-bohoon/session-conflux/pkg/transport"
)

// beginCloudSyncStream handles the shared SSE preamble for cloud sync
// handlers: ReadOnly guard, SSE stream creation, and transport setup.
// On transport error, logs and sends an SSE error event.
func (s *Server) beginCloudSyncStream(w http.ResponseWriter) (*SSEStream, confluxtransport.Transport) {
	if s.db.ReadOnly() {
		writeError(w, http.StatusNotImplemented, "cloud sync unavailable in read-only mode")
		return nil, nil
	}
	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return nil, nil
	}
	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		log.Printf("cloud sync: transport: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return nil, nil
	}
	return stream, tr
}

// sendCloudSyncProgress sends an SSE progress event in the standard
// cloud sync shape.
func sendCloudSyncProgress(stream *SSEStream, phase string, current, total int, detail string) {
	stream.SendJSON("progress", map[string]any{
		"phase":   phase,
		"current": current,
		"total":   total,
		"detail":  detail,
	})
}

// cloudMachineInfo holds summary info for one remote machine.
type cloudMachineInfo struct {
	Name           string `json:"name"`
	HasBaseline    bool   `json:"has_baseline"`
	HasIncremental bool   `json:"has_incremental"`
}

// listCloudMachines enumerates remote machines concurrently, checking only
// whether baseline/incremental directories exist (2 levels deep instead of 4).
func listCloudMachines(tr confluxtransport.Transport) []cloudMachineInfo {
	hosts, err := tr.ListFiles("")
	if err != nil {
		return nil
	}

	type result struct {
		name           string
		hasBaseline    bool
		hasIncremental bool
	}

	var wg sync.WaitGroup
	ch := make(chan result, len(hosts))

	for _, host := range hosts {
		if !host.IsDir {
			continue
		}
		wg.Add(1)
		go func(h confluxtransport.FileInfo) {
			defer wg.Done()
			r := result{name: h.Name}
			entries, err := tr.ListFiles(h.Name)
			if err != nil {
				ch <- r
				return
			}
			for _, e := range entries {
				if !e.IsDir {
					continue
				}
				switch e.Name {
				case "baseline":
					r.hasBaseline = true
				case "incremental":
					r.hasIncremental = true
				}
			}
			ch <- r
		}(host)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var machines []cloudMachineInfo
	for r := range ch {
		machines = append(machines, cloudMachineInfo{
			Name:           r.name,
			HasBaseline:    r.hasBaseline,
			HasIncremental: r.hasIncremental,
		})
	}
	return machines
}

// handleSyncCloudUpload streams an upload via SSE.
func (s *Server) handleSyncCloudUpload(w http.ResponseWriter, r *http.Request) {
	stream, tr := s.beginCloudSyncStream(w)
	if stream == nil {
		return
	}

	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err != nil {
		stream.SendJSON("error", map[string]string{"message": "loading state: " + err.Error()})
		return
	}

	stream.SendJSON("started", map[string]string{"operation": "upload"})

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)

	discoveredFiles := s.engine.ChangedFiles(time.Time{})
	var files []confluxsync.SyncFile
	for _, f := range discoveredFiles {
		info, err := os.Stat(f.Path)
		if err != nil {
			continue
		}
		files = append(files, confluxsync.FileFromDiscovered(f.Path, string(f.Agent), info.Size(), info.ModTime().UnixNano()))
	}

	stats, err := confluxsync.UploadChanged(tr, scCfg, st, files, os.DirFS("/"), func(phase string, current, total int, detail string) {
		sendCloudSyncProgress(stream, phase, current, total, detail)
	})
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
	stream, tr := s.beginCloudSyncStream(w)
	if stream == nil {
		return
	}

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

	hostname := r.URL.Query().Get("hostname")
	var n int
	if hostname != "" {
		n, err = confluxsync.DownloadSessionsForHost(tr, hostname, findAgentDir, func(phase string, current, total int, detail string) {
			sendCloudSyncProgress(stream, phase, current, total, detail)
		})
	} else {
		n, err = confluxsync.DownloadAllSessions(tr, findAgentDir, func(phase string, current, total int, detail string) {
			sendCloudSyncProgress(stream, phase, current, total, detail)
		})
	}
	if err != nil {
		log.Printf("cloud sync download: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}
	stats := &confluxsync.UploadStats{Synced: n}

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
	info := synccloud.Status(st)
	writeJSON(w, http.StatusOK, info)
}

// handleSyncCloudTest verifies the transport connection and returns a
// summary of remote machines/folders so the user can see what already exists.
func (s *Server) handleSyncCloudTest(w http.ResponseWriter, r *http.Request) {
	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}
	if err := tr.Verify(); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      false,
			"message": err.Error(),
		})
		return
	}

	machines := listCloudMachines(tr)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"message":  "connection successful",
		"machines": machines,
	})
}

// handleSyncCloudDeleteRemote streams deletion of a remote machine's data via SSE.
func (s *Server) handleSyncCloudDeleteRemote(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		writeError(w, http.StatusBadRequest, "missing hostname")
		return
	}

	stream, tr := s.beginCloudSyncStream(w)
	if stream == nil {
		return
	}

	stream.SendJSON("started", map[string]string{"operation": "delete", "hostname": hostname})

	// Collect all remote file paths under this hostname.
	sendCloudSyncProgress(stream, "listing", 0, 0, hostname)

	var paths []string
	addFiles := func(dir string) {
		files, err := tr.ListFiles(dir)
		if err != nil {
			return
		}
		for _, f := range files {
			if f.IsDir {
				continue
			}
			paths = append(paths, dir+"/"+f.Name)
		}
	}

	// baseline files
	addFiles(hostname + "/baseline")

	// incremental files per agent
	agents, err := tr.ListFiles(hostname + "/incremental")
	if err == nil {
		for _, a := range agents {
			if a.IsDir {
				addFiles(hostname + "/incremental/" + a.Name)
			}
		}
	}

	total := len(paths)
	sendCloudSyncProgress(stream, "deleting", 0, total, hostname)

	for i, p := range paths {
		if err := tr.DeleteFile(p); err != nil {
			log.Printf("cloud sync delete %s: %v", p, err)
		}
		sendCloudSyncProgress(stream, "deleting", i+1, total, p)
	}

	// update local state for this machine
	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err == nil {
		st.RemoveAll(hostname)
		if saveErr := st.Save(); saveErr != nil {
			log.Printf("cloud sync delete save state: %v", saveErr)
		}
	}

	stream.SendJSON("done", map[string]any{
		"hostname": hostname,
		"deleted":  total,
	})
}

// handleSyncCloudRemote streams remote machines via SSE as they are discovered.
// Machines are scanned concurrently — results stream as each goroutine finishes.
func (s *Server) handleSyncCloudRemote(w http.ResponseWriter, r *http.Request) {
	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	hosts, err := tr.ListFiles("")
	if err != nil {
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	stream.SendJSON("phase", map[string]string{"phase": "listing"})

	// Filter directories and scan concurrently.
	type result struct {
		name           string
		hasBaseline    bool
		hasIncremental bool
	}

	var wg sync.WaitGroup
	ch := make(chan result)

	for _, host := range hosts {
		if !host.IsDir {
			continue
		}
		wg.Add(1)
		go func(h confluxtransport.FileInfo) {
			defer wg.Done()
			r := result{name: h.Name}
			entries, err := tr.ListFiles(h.Name)
			if err != nil {
				ch <- r
				return
			}
			for _, e := range entries {
				if !e.IsDir {
					continue
				}
				switch e.Name {
				case "baseline":
					r.hasBaseline = true
				case "incremental":
					r.hasIncremental = true
				}
			}
			ch <- r
		}(host)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var machines []cloudMachineInfo
	for r := range ch {
		m := cloudMachineInfo{
			Name:           r.name,
			HasBaseline:    r.hasBaseline,
			HasIncremental: r.hasIncremental,
		}
		machines = append(machines, m)
		stream.SendJSON("machine", m)
	}

	stream.SendJSON("done", map[string]any{"machines": machines})
}
