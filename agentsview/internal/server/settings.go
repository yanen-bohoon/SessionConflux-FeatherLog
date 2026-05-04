package server

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/wesm/agentsview/internal/parser"
)

// syncCloudResponse is the JSON shape for the sync-cloud config block.
type syncCloudResponse struct {
	Enabled          bool                  `json:"enabled"`
	Schedule         string                `json:"schedule"`
	Direction        string                `json:"direction"`
	CompressionLevel int                   `json:"compression_level"`
	ExcludeAgents    []string              `json:"exclude_agents,omitempty"`
	Transport        syncTransportResponse `json:"transport"`
}

type syncTransportResponse struct {
	Backend string               `json:"backend"`
	Feishu  syncFeishuResponse   `json:"feishu,omitempty"`
	SSH     syncSSHResponse      `json:"ssh,omitempty"`
}

type syncFeishuResponse struct {
	AppID       string `json:"app_id"`
	AppSecret   string `json:"app_secret"` // masked: "••••••••" or ""
	FolderToken string `json:"folder_token,omitempty"`
}

type syncSSHResponse struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	KeyFile    string `json:"key_file"`
	RemotePath string `json:"remote_path"`
}

// settingsResponse is the JSON shape returned by GET /api/v1/settings.
type settingsResponse struct {
	AgentDirs        map[string][]string `json:"agent_dirs"`
	Terminal         terminalResponse    `json:"terminal"`
	GithubConfigured bool                `json:"github_configured"`
	Host             string              `json:"host"`
	Port             int                 `json:"port"`
	AuthToken        string              `json:"auth_token,omitempty"`
	RequireAuth      bool                `json:"require_auth"`
	SyncCloud        *syncCloudResponse  `json:"sync_cloud,omitempty"`
}

// terminalResponse mirrors config.TerminalConfig for JSON output.
type terminalResponse struct {
	Mode       string `json:"mode"`
	CustomBin  string `json:"custom_bin,omitempty"`
	CustomArgs string `json:"custom_args,omitempty"`
}

func (s *Server) handleGetSettings(
	w http.ResponseWriter, r *http.Request,
) {
	// Hold the read lock for the entire duration of building the
	// response to prevent a data race with concurrent writes.
	s.mu.RLock()
	dirs := make(map[string][]string)
	for _, def := range parser.Registry {
		if !def.FileBased && def.EnvVar == "" {
			continue
		}
		d := s.cfg.AgentDirs[def.Type]
		if d == nil {
			d = []string{}
		}
		dirs[string(def.Type)] = d
	}

	tc := s.cfg.Terminal
	if tc.Mode == "" {
		tc.Mode = "auto"
	}

	resp := settingsResponse{
		AgentDirs: dirs,
		Terminal: terminalResponse{
			Mode:       tc.Mode,
			CustomBin:  tc.CustomBin,
			CustomArgs: tc.CustomArgs,
		},
		GithubConfigured: s.cfg.GithubToken != "",
		Host:             s.cfg.Host,
		Port:             s.cfg.Port,
		RequireAuth:      s.cfg.RequireAuth,
	}

	// Only expose auth_token to localhost requests, never to remote clients.
	if isLocalhostRequest(r) {
		resp.AuthToken = s.cfg.AuthToken
	}

	// Sync cloud config (mask secrets).
	sc := s.cfg.Sync
	syncResp := &syncCloudResponse{
		Enabled:          sc.Enabled,
		Schedule:         sc.Schedule,
		Direction:        sc.Direction,
		CompressionLevel: sc.CompressionLevel,
		ExcludeAgents:    sc.ExcludeAgents,
		Transport: syncTransportResponse{
			Backend: sc.Transport.Backend,
			Feishu: syncFeishuResponse{
				AppID:       sc.Transport.Feishu.AppID,
				AppSecret:   maskSecret(sc.Transport.Feishu.AppSecret),
				FolderToken: sc.Transport.Feishu.FolderToken,
			},
			SSH: syncSSHResponse{
				Host:       sc.Transport.SSH.Host,
				Port:       sc.Transport.SSH.Port,
				User:       sc.Transport.SSH.User,
				KeyFile:    sc.Transport.SSH.KeyFile,
				RemotePath: sc.Transport.SSH.RemotePath,
			},
		},
	}
	resp.SyncCloud = syncResp
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, resp)
}

// settingsUpdateRequest is the JSON body for PUT /api/v1/settings.
// All fields are optional; only non-nil fields are applied.
type settingsUpdateRequest struct {
	Terminal    *terminalResponse  `json:"terminal,omitempty"`
	AuthToken   *string            `json:"auth_token,omitempty"`
	RequireAuth *bool              `json:"require_auth,omitempty"`
	SyncCloud   *syncCloudResponse `json:"sync_cloud,omitempty"`
}

func (s *Server) handleUpdateSettings(
	w http.ResponseWriter, r *http.Request,
) {
	if s.db.ReadOnly() {
		writeError(w, http.StatusNotImplemented,
			"settings cannot be modified in read-only mode")
		return
	}

	var req settingsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	patch := make(map[string]any)

	// Terminal config is deliberately excluded from the generic
	// settings endpoint. It must be updated through the dedicated
	// POST /api/v1/config/terminal endpoint which validates
	// custom_bin and custom_args to prevent command injection.
	if req.Terminal != nil {
		writeError(w, http.StatusBadRequest,
			"terminal config must be updated via POST /api/v1/config/terminal")
		return
	}

	if req.AuthToken != nil {
		patch["auth_token"] = *req.AuthToken
	}

	if req.RequireAuth != nil {
		patch["require_auth"] = *req.RequireAuth
	}

	if req.SyncCloud != nil {
		syncPatch := s.cfg.Sync // start from current config
		syncPatch.Enabled = req.SyncCloud.Enabled
		if req.SyncCloud.Schedule != "" {
			syncPatch.Schedule = req.SyncCloud.Schedule
		}
		if req.SyncCloud.Direction != "" {
			syncPatch.Direction = req.SyncCloud.Direction
		}
		if req.SyncCloud.CompressionLevel != 0 {
			syncPatch.CompressionLevel = req.SyncCloud.CompressionLevel
		}
		if req.SyncCloud.ExcludeAgents != nil {
			syncPatch.ExcludeAgents = req.SyncCloud.ExcludeAgents
		}
		if req.SyncCloud.Transport.Backend != "" {
			syncPatch.Transport.Backend = req.SyncCloud.Transport.Backend
		}
		if req.SyncCloud.Transport.Feishu.AppID != "" {
			syncPatch.Transport.Feishu.AppID = req.SyncCloud.Transport.Feishu.AppID
		}
		// Preserve existing secret when the masked value is sent back.
		if req.SyncCloud.Transport.Feishu.AppSecret != "" &&
			!isMasked(req.SyncCloud.Transport.Feishu.AppSecret) {
			syncPatch.Transport.Feishu.AppSecret = req.SyncCloud.Transport.Feishu.AppSecret
		}
		if req.SyncCloud.Transport.Feishu.FolderToken != "" {
			syncPatch.Transport.Feishu.FolderToken = req.SyncCloud.Transport.Feishu.FolderToken
		}
		if req.SyncCloud.Transport.SSH.Host != "" {
			syncPatch.Transport.SSH.Host = req.SyncCloud.Transport.SSH.Host
		}
		if req.SyncCloud.Transport.SSH.Port != 0 {
			syncPatch.Transport.SSH.Port = req.SyncCloud.Transport.SSH.Port
		}
		if req.SyncCloud.Transport.SSH.User != "" {
			syncPatch.Transport.SSH.User = req.SyncCloud.Transport.SSH.User
		}
		if req.SyncCloud.Transport.SSH.KeyFile != "" {
			syncPatch.Transport.SSH.KeyFile = req.SyncCloud.Transport.SSH.KeyFile
		}
		if req.SyncCloud.Transport.SSH.RemotePath != "" {
			syncPatch.Transport.SSH.RemotePath = req.SyncCloud.Transport.SSH.RemotePath
		}
		patch["sync"] = syncPatch
	}

	if len(patch) == 0 {
		// Nothing to update; return current settings.
		s.handleGetSettings(w, r)
		return
	}

	s.mu.Lock()
	err := s.cfg.SaveSettings(patch)
	// Auto-generate auth token when require_auth is enabled.
	if err == nil && s.cfg.RequireAuth {
		err = s.cfg.EnsureAuthToken()
	}
	s.mu.Unlock()
	if err != nil {
		log.Printf("save settings: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Return the full updated settings.
	s.handleGetSettings(w, r)
}

// maskSecret returns "••••••••" if s is non-empty, or "" otherwise.
func maskSecret(s string) string {
	if s == "" {
		return ""
	}
	return strings.Repeat("•", 8)
}

// isMasked reports whether s is the masked secret placeholder.
func isMasked(s string) bool {
	return strings.TrimSpace(s) == strings.Repeat("•", 8)
}
