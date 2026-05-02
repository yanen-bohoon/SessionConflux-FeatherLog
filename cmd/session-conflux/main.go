package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/yanen-bohoon/session-conflux/internal/config"
	"github.com/yanen-bohoon/session-conflux/internal/feishu"
	"github.com/yanen-bohoon/session-conflux/internal/scanner"
	"github.com/yanen-bohoon/session-conflux/internal/scheduler"
	"github.com/yanen-bohoon/session-conflux/internal/state"
	"github.com/yanen-bohoon/session-conflux/internal/sync"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "session-conflux",
	Short: "Sync AI agent sessions across machines via Feishu Drive",
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
	Short:   "Guided setup wizard for Feishu credentials",
	Run:     runSetup,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Daemon mode, auto sync daily at scheduled time",
	Run:   runSync,
}

var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload changed sessions to Feishu Drive",
	Run:   runUpload,
}

var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download sessions from Feishu Drive",
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
	downloadCmd.Flags().BoolVar(&downloadAll, "all", false, "Download all remote sessions")
	downloadCmd.Flags().StringVar(&downloadSession, "session", "", "Download specific session by key")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(uploadCmd)
	rootCmd.AddCommand(downloadCmd)
	rootCmd.AddCommand(listCmd)
}

func runSync(cmd *cobra.Command, args []string) {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Feishu.AppID == "" || cfg.Feishu.AppSecret == "" {
		fmt.Fprintln(os.Stderr, "Feishu not configured. Run 'session-conflux config' first.")
		os.Exit(1)
	}

	// Run scheduler
	if err := scheduler.Daily(cfg.Sync.Schedule, func() error {
		return sync.RunFullSync(cfg)
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

	var lastUpload, lastDownload string
	uploadCount, downloadCount := 0, 0
	for _, e := range entries {
		if e.LastUploaded > lastUpload {
			lastUpload = e.LastUploaded
		}
		if e.FileToken != "" {
			uploadCount++
		}
		if e.LastDownloaded > lastDownload {
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
	if lastUpload != "" {
		fmt.Printf("Last upload: %s\n", lastUpload)
	}
	fmt.Printf("Sessions downloaded: %d\n", downloadCount)
	if lastDownload != "" {
		fmt.Printf("Last download: %s\n", lastDownload)
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

	// Step 1: App ID (required, loop until non-empty)
	for cfg.Feishu.AppID == "" {
		fmt.Print("1. Feishu App ID: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			fmt.Println("   App ID is required. Find it at Feishu Open Platform → App Settings.")
			continue
		}
		cfg.Feishu.AppID = input
	}

	// Step 2: App Secret (required, min 8 chars)
	for cfg.Feishu.AppSecret == "" {
		fmt.Print("2. Feishu App Secret: ")
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
		cfg.Feishu.AppSecret = input
	}

	// Step 3: Verify credentials (loop until success)
	fmt.Println()
	for {
		fmt.Print("3. Verifying credentials... ")
		token, err := feishu.GetTenantToken(cfg.Feishu.AppID, cfg.Feishu.AppSecret)
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			fmt.Print("   Re-enter App ID? (Enter to keep, or type new value): ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Feishu.AppID = input
			}
			fmt.Print("   Re-enter App Secret? (Enter to keep, or type new value): ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input != "" {
				cfg.Feishu.AppSecret = input
			}
			continue
		}
		fmt.Println("OK")

		// Step 4: Find or create root folder
		fmt.Print("4. Setting up SessionConflux folder... ")
		folderToken, err := feishu.FindOrCreateFolder(token, "SessionConflux")
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
		} else {
			cfg.Feishu.FolderToken = folderToken
			fmt.Println("OK")
		}
		break
	}

	// Step 5: Sync schedule (optional)
	fmt.Println()
	fmt.Printf("5. Sync schedule (HH:MM, 24h format) [%s]: ", cfg.Sync.Schedule)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input != "" {
		cfg.Sync.Schedule = input
	}

	if err := config.Save(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("Configuration saved to ~/.session-conflux/config.toml")
	fmt.Println("Run 'session-conflux upload' to start syncing.")
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

	// Print table
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

	if cfg.Feishu.AppID == "" || cfg.Feishu.AppSecret == "" {
		fmt.Fprintln(os.Stderr, "Feishu not configured. Run 'session-conflux config' first.")
		os.Exit(1)
	}

	st, err := state.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load state: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Scanning for changed sessions...")
	stats, err := sync.UploadChanged(cfg, st)
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

	if cfg.Feishu.AppID == "" || cfg.Feishu.AppSecret == "" {
		fmt.Fprintln(os.Stderr, "Feishu not configured. Run 'session-conflux config' first.")
		os.Exit(1)
	}

	// --all flag
	if downloadAll {
		fmt.Println("Downloading all remote sessions...")
		n, err := sync.DownloadAllSessions(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Download failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Downloaded %d sessions.\n", n)
		return
	}

	// --session flag
	if downloadSession != "" {
		sessions, err := sync.ListRemoteSessions(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list remote sessions: %v\n", err)
			os.Exit(1)
		}
		for _, s := range sessions {
			if s.Key == downloadSession {
				if err := sync.DownloadSession(cfg, s); err != nil {
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
	sessions, err := sync.ListRemoteSessions(cfg)
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
