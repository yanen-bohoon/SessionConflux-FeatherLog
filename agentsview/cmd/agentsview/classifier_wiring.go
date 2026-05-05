// ABOUTME: ApplyClassifierConfig installs user-defined
// ABOUTME: classifier prefixes into the db package singleton.
package avcli

import (
	"github.com/wesm/agentsview/internal/config"
	"github.com/wesm/agentsview/internal/db"
)

// ApplyClassifierConfig installs user-defined classifier
// prefixes into the db package singleton. Every command that
// loads config and may open SQLite or PostgreSQL must call
// this BEFORE db.Open / postgres.Open / postgres.NewStore /
// postgres.New / postgres.EnsureSchema. Silent by design so
// it's safe to call from quiet CLI paths (statusline, JSON
// output, etc.); see db.SetUserAutomationPrefixes for
// rationale. The static guardrail test in
// classifier_wiring_test.go (Task 7) enforces this rule.
func ApplyClassifierConfig(cfg config.Config) {
	db.SetUserAutomationPrefixes(cfg.Automated.Prefixes)
}
