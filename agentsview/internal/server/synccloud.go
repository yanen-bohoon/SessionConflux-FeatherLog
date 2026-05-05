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

// remoteCache caches listCloudMachines results with a TTL.
type remoteCache struct {
	mu       sync.Mutex
	machines []cloudMachineInfo
	expiry   time.Time
}

var remoteMachineCache remoteCache

const remoteCacheTTL = 30 * time.Second

func (c *remoteCache) get() ([]cloudMachineInfo, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.expiry) {
		return c.machines, true
	}
	return nil, false
}

func (c *remoteCache) set(machines []cloudMachineInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.machines = machines
	c.expiry = time.Now().Add(remoteCacheTTL)
}

// InvalidateRemoteCache clears the cached remote machine list so the
// next /remote request re-lists from the transport. Call after upload,
// download, or delete operations that change remote state.
func InvalidateRemoteCache() {
	remoteMachineCache.mu.Lock()
	defer remoteMachineCache.mu.Unlock()
	remoteMachineCache.expiry = time.Time{}
}

// Types for listing remote machine data.
type cloudIncrementalInfo struct {
	Agent string `json:"agent"`
	Count int    `json:"count"`
}
type cloudBaselineInfo struct {
	Files int   `json:"files"`
	Size  int64 `json:"size"`
}
type cloudMachineInfo struct {
	Name        string                 `json:"name"`
	Baseline    *cloudBaselineInfo     `json:"baseline,omitempty"`
	Incremental []cloudIncrementalInfo `json:"incremental,omitempty"`
}

// listCloudMachines enumerates remote machines and their baseline/incremental data.
func listCloudMachines(tr confluxtransport.Transport) []cloudMachineInfo {
	var machines []cloudMachineInfo

	hosts, err := tr.ListFiles("")
	if err != nil {
		return machines
	}
	for _, host := range hosts {
		if !host.IsDir {
			continue
		}
		m := cloudMachineInfo{Name: host.Name}

		l3Files, err := tr.ListFiles(host.Name)
		if err != nil {
			machines = append(machines, m)
			continue
		}
		for _, l3 := range l3Files {
			if !l3.IsDir {
				continue
			}
			switch l3.Name {
			case "baseline":
				parts, err := tr.ListFiles(host.Name + "/baseline")
				if err == nil {
					var totalSize int64
					for _, p := range parts {
						totalSize += p.Size
					}
					if len(parts) > 0 {
						m.Baseline = &cloudBaselineInfo{Files: len(parts), Size: totalSize}
					}
				}
			case "incremental":
				agents, err := tr.ListFiles(host.Name + "/incremental")
				if err == nil {
					for _, a := range agents {
						if !a.IsDir {
							continue
						}
						sessions, err := tr.ListFiles(host.Name + "/incremental/" + a.Name)
						count := 0
						if err == nil {
							count = len(sessions)
						}
						m.Incremental = append(m.Incremental, cloudIncrementalInfo{Agent: a.Name, Count: count})
					}
				}
			}
		}
		machines = append(machines, m)
	}
	return machines
}

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

	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		log.Printf("cloud sync upload: transport: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}
	stats, err := confluxsync.UploadChanged(tr, scCfg, st, files, func(phase string, current, total int, detail string) {
		stream.SendJSON("progress", map[string]any{
			"phase":   phase,
			"current": current,
			"total":   total,
			"detail":  detail,
		})
	})
	if err != nil {
		log.Printf("cloud sync upload: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	if err := st.Save(); err != nil {
		log.Printf("cloud sync save state: %v", err)
	}

	InvalidateRemoteCache()
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


	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		log.Printf("cloud sync download: transport: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	progressFn := func(phase string, current, total int, detail string) {
		stream.SendJSON("progress", map[string]any{
			"phase":   phase,
			"current": current,
			"total":   total,
			"detail":  detail,
		})
	}

	hostname := r.URL.Query().Get("hostname")
	var n int
	if hostname != "" {
		n, err = confluxsync.DownloadSessionsForHost(tr, hostname, findAgentDir, progressFn)
	} else {
		n, err = confluxsync.DownloadAllSessions(tr, findAgentDir, progressFn)
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

	InvalidateRemoteCache()
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
	if s.db.ReadOnly() {
		writeError(w, http.StatusNotImplemented, "cloud sync unavailable in read-only mode")
		return
	}

	hostname := r.PathValue("hostname")
	if hostname == "" {
		writeError(w, http.StatusBadRequest, "missing hostname")
		return
	}

	stream, err := NewSSEStream(w)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		log.Printf("cloud sync delete: transport: %v", err)
		stream.SendJSON("error", map[string]string{"message": err.Error()})
		return
	}

	stream.SendJSON("started", map[string]string{"operation": "delete", "hostname": hostname})

	// Collect all remote file paths under this hostname.
	stream.SendJSON("progress", map[string]any{
		"phase":   "listing",
		"current": 0,
		"total":   0,
		"detail":  hostname,
	})

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
	stream.SendJSON("progress", map[string]any{
		"phase":   "deleting",
		"current": 0,
		"total":   total,
		"detail":  hostname,
	})

	for i, p := range paths {
		if err := tr.DeleteFile(p); err != nil {
			log.Printf("cloud sync delete %s: %v", p, err)
		}
		stream.SendJSON("progress", map[string]any{
			"phase":   "deleting",
			"current": i + 1,
			"total":   total,
			"detail":  p,
		})
	}

	// update local state for this machine
	st, err := synccloud.LoadState(s.cfg.DataDir)
	if err == nil {
		st.RemoveAll(hostname)
		if saveErr := st.Save(); saveErr != nil {
			log.Printf("cloud sync delete save state: %v", saveErr)
		}
	}

	InvalidateRemoteCache()
	stream.SendJSON("done", map[string]any{
		"hostname": hostname,
		"deleted":  total,
	})
}

// handleSyncCloudRemote lists remote machines and their data without
// verifying the connection first (assumes config is already saved/tested).
// Results are cached for remoteCacheTTL to avoid repeated slow Feishu API calls.
func (s *Server) handleSyncCloudRemote(w http.ResponseWriter, r *http.Request) {
	if cached, ok := remoteMachineCache.get(); ok {
		writeJSON(w, http.StatusOK, map[string]any{
			"machines": cached,
		})
		return
	}

	scCfg := synccloud.ToSessionConfluxConfig(&s.cfg.Sync)
	tr, err := confluxtransport.New(scCfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "transport: "+err.Error())
		return
	}

	machines := listCloudMachines(tr)
	remoteMachineCache.set(machines)
	writeJSON(w, http.StatusOK, map[string]any{
		"machines": machines,
	})
}
