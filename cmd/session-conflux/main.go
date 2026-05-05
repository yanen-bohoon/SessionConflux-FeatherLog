package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	avcli "github.com/wesm/agentsview/cmd/agentsview"
	"github.com/yanen-bohoon/session-conflux/pkg/config"
	"github.com/yanen-bohoon/session-conflux/pkg/scanner"
	"github.com/yanen-bohoon/session-conflux/pkg/scheduler"
	"github.com/yanen-bohoon/session-conflux/pkg/state"
	"github.com/yanen-bohoon/session-conflux/pkg/sync"
	"github.com/yanen-bohoon/session-conflux/pkg/transport"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "session-conflux",
	Short: "Sync AI agent sessions across machines via Feishu Drive or SSH",
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("session-conflux", version)
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status summary",
	Run:   runStatus,
}

var configCmd = &cobra.Command{
	Use:     "setup",
	Aliases: []string{"config"},
	Short:   "Guided setup wizard",
	Run:     runSetup,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Daemon mode, auto sync daily at scheduled time",
	Run:   runSync,
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload changed sessions",
	Run:   runUpload,
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download sessions",
	Run:   runDownload,
}

var downloadAll bool
var downloadSession string

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all local AI agent sessions",
	Run:   runList,
}

func init() {
	// Root-level groups (mirroring agentsview groups)
	rootCmd.AddGroup(
		&cobra.Group{ID: "cloud", Title: "Cloud Sync Commands:"},
		&cobra.Group{ID: "core", Title: "Core Commands:"},
		&cobra.Group{ID: "data", Title: "Data Commands:"},
		&cobra.Group{ID: "usage", Title: "Usage Commands:"},
		&cobra.Group{ID: "meta", Title: "Other Commands:"},
	)

	// Cloud Sync Commands (original session-conflux)
	syncCmd.GroupID = "cloud"
	uploadCmd.GroupID = "cloud"
	downloadCmd.GroupID = "cloud"
	statusCmd.GroupID = "cloud"
	configCmd.GroupID = "cloud"
	listCmd.GroupID = "cloud"

	downloadCmd.Flags().BoolVar(&downloadAll, "all", false, "Download all remote sessions")
	downloadCmd.Flags().StringVar(&downloadSession, "session", "", "Download specific session by key")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(listCmd)

	// AgentsView Commands (Step 1 merge)
	rootCmd.AddCommand(avcli.NewServeCommand())

	// Rename agentsview 'sync' to 'file-sync' to avoid conflict with cloud sync
	fileSyncCmd := avcli.NewFileSyncCommand()
	fileSyncCmd.Use = "file-sync"
	fileSyncCmd.Short = "Local file sync (index sessions into SQLite)"
	rootCmd.AddCommand(fileSyncCmd)

	rootCmd.AddCommand(avcli.NewPruneCommand())
	rootCmd.AddCommand(avcli.NewUpdateCommand())
	rootCmd.AddCommand(avcli.NewTokenUseCommand())
	rootCmd.AddCommand(avcli.NewImportCommand())
	rootCmd.AddCommand(avcli.NewProjectsCommand())
	rootCmd.AddCommand(avcli.NewHealthCommand())
	rootCmd.AddCommand(avcli.NewUsageCommand())
	rootCmd.AddCommand(avcli.NewPGCommand())
	rootCmd.AddCommand(avcli.NewSessionCommand())
	rootCmd.AddCommand(avcli.NewStatsCommand())
	rootCmd.AddCommand(avcli.NewClassifierCommand())
}

func runSync(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Transport.Backend == "" {
		fmt.Fprintln(os.Stderr, "No transport configured. Run 'session-conflux setup' first.")
		os.Exit(1)
	}

	if err := scheduler.Daily(cfg.Sync.Schedule, func() error {
		t, err := transport.New(cfg)
		if err != nil {
			return fmt.Errorf("transport: %w", err)
		}

		st, err := state.Load()
		if err != nil {
			return fmt.Errorf("load state: %w", err)
		}

		dir := cfg.Sync.Direction
		if dir == "" {
			dir = "both"
		}
		doUpload := dir == "upload" || dir == "both"
		doDownload := dir == "download" || dir == "both"

		if doUpload {
			fmt.Println("--- Upload ---")
			stats, err := sync.UploadChanged(t, cfg, st)
			if err != nil {
				return fmt.Errorf("upload: %w", err)
			}
			fmt.Printf("Upload: %d total, %d synced, %d skipped, %d failed.\n",
				stats.Total, stats.Synced, stats.Skipped, stats.Failed)
		}
		if doDownload {
			if doUpload {
				fmt.Println()
			}
			fmt.Println("--- Download ---")
			n, err := sync.DownloadAllSessions(t)
			if err != nil {
				return fmt.Errorf("download: %w", err)
			}
			fmt.Printf("Download: %d sessions downloaded.\n", n)
		}
		return nil
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Scheduler error: %v\n", err)
		os.Exit(1)
	}
}

func runStatus(cmd *cobra.Command, args []string) {
	st, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		os.Exit(1)
	}

	entries := st.All()
	if len(entries) == 0 {
		fmt.Println("No sync state yet. Run 'session-conflux upload' or 'session-conflux download' first.")
		return
	}

	var lastUpload, lastDownload time.Time
	uploadCount, downloadCount := 0, 0
	for _, e := range entries {
		if e.LastUploaded.After(lastUpload) {
			lastUpload = e.LastUploaded
		}
		if e.FileToken != "" {
			uploadCount++
		}
		if e.LastDownloaded.After(lastDownload) {
			lastDownload = e.LastDownloaded
		}
		if e.DownloadedToken != "" {
			downloadCount++
		}
	}

	fmt.Println("Sync Status")
	fmt.Println("===========")
	fmt.Printf("Sessions tracked: %d\n", len(entries))
	fmt.Printf("Sessions uploaded: %d\n", uploadCount)
	if !lastUpload.IsZero() {
		fmt.Printf("Last upload: %s\n", lastUpload.Format(time.RFC3339))
	}
	fmt.Printf("Sessions downloaded: %d\n", downloadCount)
	if !lastDownload.IsZero() {
		fmt.Printf("Last download: %s\n", lastDownload.Format(time.RFC3339))
	}
}

func runSetup(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("SessionConflux Setup")
	fmt.Println("====================")
	fmt.Println()

	// Step 1: Choose backend.
	for cfg.Transport.Backend == "" {
		fmt.Print("1. Backend (feishu / ssh): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		switch input {
		case "feishu", "ssh":
			cfg.Transport.Backend = input
		default:
			fmt.Println("   Please enter 'feishu' or 'ssh'.")
		}
	}

	if cfg.Transport.Backend == "feishu" {
		setupFeishu(cfg, reader)
	} else {
		setupSSH(cfg, reader)
	}

	// Sync schedule (common).
	fmt.Println()
	fmt.Printf("Sync schedule (HH:MM, 24h format) [%s]: ", cfg.Sync.Schedule)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.Sync.Schedule = input
	}

	// agentsview port.
	fmt.Println()
	fmt.Printf("AgentsView web port [8080]: ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	port := 8080
	if input != "" {
		if p, err := strconv.Atoi(input); err != nil || p < 1 || p > 65535 {
			fmt.Printf("   Invalid port, using default 8080.\n")
		} else {
			port = p
		}
	}
	if err := writeAgentsviewConfig(port); err != nil {
		fmt.Printf("   WARN: could not write agentsview config: %v\n", err)
	} else {
		fmt.Printf("AgentsView port %d saved to ~/.agentsview/config.toml\n", port)
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("Configuration saved to ~/.session-conflux/config.toml")
	fmt.Println("Run 'session-conflux upload' to start syncing.")
	fmt.Println("Run 'session-conflux serve' to start the web viewer.")
}

func setupFeishu(cfg *config.Config, reader *bufio.Reader) {
	// App ID.
	for cfg.Transport.Feishu.AppID == "" {
		fmt.Print("2. Feishu App ID: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   App ID is required. Find it at Feishu Open Platform → App Settings.")
			continue
		}
		cfg.Transport.Feishu.AppID = input
	}

	// App Secret.
	for cfg.Transport.Feishu.AppSecret == "" {
		fmt.Print("3. Feishu App Secret: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   App Secret is required.")
			continue
		}
		if len(input) < 8 {
			fmt.Println("   App Secret appears too short (< 8 chars). Please re-enter.")
			continue
		}
		cfg.Transport.Feishu.AppSecret = input
	}

	// Verify credentials + create root folder.
	fmt.Println()
	for {
		ft := transport.NewFeishuTransport(cfg.Transport.Feishu.AppID, cfg.Transport.Feishu.AppSecret, cfg.Transport.Feishu.FolderToken)
		fmt.Print("4. Verifying credentials... ")
		err := ft.Verify()
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			fmt.Print("   Re-enter App ID? (Enter to keep, or type new value): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Transport.Feishu.AppID = input
			}
			fmt.Print("   Re-enter App Secret? (Enter to keep, or type new value): ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Transport.Feishu.AppSecret = input
			}
			continue
		}
		fmt.Println("OK")

		fmt.Print("5. Setting up SessionConflux folder... ")
		err = ft.CreateFolder("")
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			cfg.Transport.Feishu.FolderToken = ft.RootToken()
			fmt.Println("OK")
		}
		break
	}
}

func setupSSH(cfg *config.Config, reader *bufio.Reader) {
	// Host.
	for cfg.Transport.SSH.Host == "" {
		fmt.Print("2. SSH host: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   Host is required.")
			continue
		}
		cfg.Transport.SSH.Host = input
	}

	// Port (default 22).
	if cfg.Transport.SSH.Port == 0 {
		cfg.Transport.SSH.Port = 22
	}
	fmt.Printf("3. SSH port [%d]: ", cfg.Transport.SSH.Port)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		fmt.Sscanf(input, "%d", &cfg.Transport.SSH.Port)
	}

	// User.
	for cfg.Transport.SSH.User == "" {
		fmt.Print("4. SSH user: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   User is required.")
			continue
		}
		cfg.Transport.SSH.User = input
	}

	// Key file (default ~/.ssh/id_ed25519).
	if cfg.Transport.SSH.KeyFile == "" {
		cfg.Transport.SSH.KeyFile = "~/.ssh/id_ed25519"
	}
	fmt.Printf("5. SSH private key file [%s]: ", cfg.Transport.SSH.KeyFile)
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.Transport.SSH.KeyFile = input
	}

	// Remote path.
	for cfg.Transport.SSH.RemotePath == "" {
		fmt.Print("6. Remote path (e.g. /data/session-conflux): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   Remote path is required.")
			continue
		}
		cfg.Transport.SSH.RemotePath = input
	}

	// Verify connection.
	fmt.Println()
	for {
		fmt.Print("7. Verifying SSH connection... ")
		t, err := transport.NewSSHTransport(cfg.Transport.SSH)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			fmt.Print("   Re-enter host? (Enter to keep, or type new value): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Transport.SSH.Host = input
			}
			fmt.Print("   Re-enter port? (Enter to keep, or type new value): ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				fmt.Sscanf(input, "%d", &cfg.Transport.SSH.Port)
			}
			fmt.Print("   Re-enter user? (Enter to keep, or type new value): ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Transport.SSH.User = input
			}
			fmt.Print("   Re-enter key file? (Enter to keep, or type new value): ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Transport.SSH.KeyFile = input
			}

			continue
		}
		t.Close()
		fmt.Println("OK")
		break
	}
}

func writeAgentsviewConfig(port int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".agentsview")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	content := fmt.Sprintf("port = %d\n", port)
	return os.WriteFile(filepath.Join(dir, "config.toml"), []byte(content), 0644)
}

func runList(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	exclude := make(map[string]bool)
	for _, a := range cfg.Agents.Exclude {
		exclude[a] = true
	}

	results, err := scanner.Scan(exclude)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Scan failed: %v\n", err)
		os.Exit(1)
	}

	if len(results) == 0 {
		fmt.Println("No sessions found.")
		return
	}

	fmt.Printf("%-12s %-20s %8s\n", "AGENT", "SESSION ID", "SIZE")
	fmt.Println(strings.Repeat("-", 50))
	for _, r := range results {
		sizeKB := r.Size / 1024
		fmt.Printf("%-12s %-20s %6dK\n", r.Agent, r.SessionID, sizeKB)
	}
	fmt.Printf("\n%d sessions total.\n", len(results))
}

func runUpload(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Transport.Backend == "" {
		fmt.Fprintln(os.Stderr, "No transport configured. Run 'session-conflux setup' first.")
		os.Exit(1)
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		os.Exit(1)
	}

	t, err := transport.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Transport init failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Scanning for changed sessions...")
	stats, err := sync.UploadChanged(t, cfg, st)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Upload failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nDone. %d total, %d synced, %d skipped, %d failed.\n",
		stats.Total, stats.Synced, stats.Skipped, stats.Failed)
}

func runDownload(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Transport.Backend == "" {
		fmt.Fprintln(os.Stderr, "No transport configured. Run 'session-conflux setup' first.")
		os.Exit(1)
	}

	t, err := transport.New(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Transport init failed: %v\n", err)
		os.Exit(1)
	}

	// --all flag
	if downloadAll {
		fmt.Println("Downloading all remote sessions...")
		n, err := sync.DownloadAllSessions(t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Downloaded %d sessions.\n", n)
		return
	}

	// --session flag
	if downloadSession != "" {
		sessions, err := sync.ListRemoteSessions(t)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list remote sessions: %v\n", err)
			os.Exit(1)
		}
		for _, s := range sessions {
			if s.Key == downloadSession {
				if err := sync.DownloadSession(t, s); err != nil {
					fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
					os.Exit(1)
				}
				fmt.Println("Done.")
				return
			}
		}
		fmt.Fprintf(os.Stderr, "Session not found: %s\n", downloadSession)
		os.Exit(1)
	}

	// Interactive mode (default): list remote sessions
	fmt.Println("Fetching remote sessions...")
	sessions, err := sync.ListRemoteSessions(t)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to list remote sessions: %v\n", err)
		os.Exit(1)
	}
	if len(sessions) == 0 {
		fmt.Println("No remote sessions found.")
		return
	}

	fmt.Printf("\nRemote sessions (%d):\n\n", len(sessions))
	fmt.Printf("%-50s %-10s\n", "KEY", "AGENT")
	fmt.Println(strings.Repeat("-", 65))
	for _, s := range sessions {
		fmt.Printf("%-50s %-10s\n", s.Key, s.Agent)
	}
	fmt.Println("\nUse --all to download all, or --session <key> for a specific one.")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
