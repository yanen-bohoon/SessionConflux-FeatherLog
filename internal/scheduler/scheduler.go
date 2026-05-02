package scheduler

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Task is a function that the scheduler runs.
type Task func() error

// Daily schedules a task to run daily at the given time string (HH:MM).
// Returns when the context is cancelled or SIGINT/SIGTERM received.
func Daily(schedule string, task Task) error {
	if schedule == "" {
		schedule = "02:00"
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("Scheduler started. Will sync daily at %s.\n", schedule)
	fmt.Println("Press Ctrl+C to stop.")

	var mu sync.Mutex
	running := false

	runTask := func() {
		mu.Lock()
		if running {
			mu.Unlock()
			fmt.Println("Previous sync still running, skipping this cycle.")
			return
		}
		running = true
		mu.Unlock()

		fmt.Printf("\n=== Sync started at %s ===\n", time.Now().Format(time.RFC3339))
		start := time.Now()
		if err := task(); err != nil {
			fmt.Fprintf(os.Stderr, "Sync error: %v\n", err)
		}
		fmt.Printf("=== Sync completed in %s ===\n", time.Since(start).Round(time.Second))

		mu.Lock()
		running = false
		mu.Unlock()
	}

	for {
		next := nextScheduleTime(schedule)
		waitDuration := time.Until(next)
		fmt.Printf("Next sync at: %s (in %s)\n", next.Format(time.RFC3339), waitDuration.Round(time.Second))

		select {
		case <-sigCh:
			fmt.Println("\nShutting down...")
			return nil
		case <-time.After(waitDuration):
			runTask()
		}
	}
}

// nextScheduleTime returns the next occurrence of HH:MM.
func nextScheduleTime(schedule string) time.Time {
	now := time.Now()
	t, err := time.Parse("15:04", schedule)
	if err != nil {
		// Default to 02:00
		t, _ = time.Parse("15:04", "02:00")
	}
	target := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	if target.Before(now) || target.Equal(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}
