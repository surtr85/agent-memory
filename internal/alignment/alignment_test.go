package alignment

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/surtr85/agent-memory/internal/db"
)

func setupTestDB(t *testing.T) *sql.DB {
	database, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open memory db: %v", err)
	}
	if err := db.InitSchema(database); err != nil {
		t.Fatalf("failed to init schema: %v", err)
	}
	return database
}

func TestAlignmentAndRepairs(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	mgr := NewManager(database)

	// Add Guidance
	g, err := mgr.AddGuidance("boundary", "Never output corporate pleasantries or generic fluff", 1.5)
	if err != nil {
		t.Fatalf("AddGuidance failed: %v", err)
	}
	if g.Category != "boundary" || g.Salience != 1.5 {
		t.Errorf("unexpected guidance: %+v", g)
	}

	// Record Repair Thread
	th, err := mgr.RecordRepairThread("User noted redundant greetings", "Skip hello/greeting, jump straight to architecture or solution")
	if err != nil {
		t.Fatalf("RecordRepairThread failed: %v", err)
	}
	if th.Status != "open" {
		t.Errorf("expected status 'open', got %s", th.Status)
	}

	// Synthesis Prompt Generation
	synth, err := mgr.GenerateSynthesisPrompt()
	if err != nil {
		t.Fatalf("GenerateSynthesisPrompt failed: %v", err)
	}

	if !strings.Contains(synth, "<alignment_synthesis>") || !strings.Contains(synth, "Never output corporate pleasantries") {
		t.Errorf("synthesis missing boundary: %s", synth)
	}
	if !strings.Contains(synth, "Skip hello/greeting") {
		t.Errorf("synthesis missing repair instruction: %s", synth)
	}

	// Resolve Repair Thread
	if err := mgr.ResolveRepairThread(th.ID); err != nil {
		t.Fatalf("ResolveRepairThread failed: %v", err)
	}

	openThreads, err := mgr.GetOpenRepairThreads()
	if err != nil {
		t.Fatalf("GetOpenRepairThreads failed: %v", err)
	}
	if len(openThreads) != 0 {
		t.Errorf("expected 0 open threads, got %d", len(openThreads))
	}
}
