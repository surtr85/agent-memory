package temporal

import (
	"database/sql"
	"testing"
	"time"

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

func TestAddFact_Simple(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	reg := NewRegistry(database)

	fact, err := reg.AddFactWithCitation("system", "User", "prefers_editor", "Neovim", "user_prompt", "memory://live/human.md", "User prefers Neovim", 10, 0.9)
	if err != nil {
		t.Fatalf("AddFact failed: %v", err)
	}
	if fact.ID == "" {
		t.Errorf("expected fact to have non-empty ID")
	}
	if fact.Object != "Neovim" {
		t.Errorf("expected object 'Neovim', got %q", fact.Object)
	}
	if fact.SourceURI != "memory://live/human.md" || fact.SourceQuote != "User prefers Neovim" || fact.LineNumber != 10 {
		t.Errorf("citation fields mismatch: %+v", fact)
	}
	if fact.ValidUntil != nil {
		t.Errorf("expected ValidUntil to be nil for active fact")
	}

	active, err := reg.GetActiveFacts("system")
	if err != nil {
		t.Fatalf("GetActiveFacts failed: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected 1 active fact, got %d", len(active))
	}
	if active[0].ID != fact.ID {
		t.Errorf("expected fact ID %s, got %s", fact.ID, active[0].ID)
	}

	// Test ExplainFact
	evidence, err := reg.ExplainFact(fact.ID)
	if err != nil {
		t.Fatalf("ExplainFact failed: %v", err)
	}
	if evidence.SourceURI != "memory://live/human.md" || evidence.EvidenceQuote != "User prefers Neovim" || evidence.Status != "active" {
		t.Errorf("unexpected evidence data: %+v", evidence)
	}
}

func TestAddFact_ContradictionResolution(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	reg := NewRegistry(database)

	f1, err := reg.AddFact("system", "User", "uses_compositor", "Hyprland", "test")
	if err != nil {
		t.Fatalf("AddFact 1 failed: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	// User switches from Hyprland to Niri
	f2, err := reg.AddFact("system", "User", "uses_compositor", "Niri", "test")
	if err != nil {
		t.Fatalf("AddFact 2 failed: %v", err)
	}

	if f1.ID == f2.ID {
		t.Fatalf("f1 and f2 should have different IDs")
	}

	// Active facts check: only Niri should be active
	active, err := reg.GetActiveFacts("system")
	if err != nil {
		t.Fatalf("GetActiveFacts failed: %v", err)
	}
	if len(active) != 1 {
		t.Fatalf("expected exactly 1 active fact, got %d", len(active))
	}
	if active[0].Object != "Niri" {
		t.Errorf("expected active fact object 'Niri', got %q", active[0].Object)
	}

	// Query all facts for subject/predicate
	all, err := reg.QueryFacts("system", "User", "uses_compositor")
	if err != nil {
		t.Fatalf("QueryFacts failed: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 total facts (1 active, 1 superseded), got %d", len(all))
	}

	// Find the old Hyprland fact and verify superseded_by and valid_until
	var oldFact, newFact *Fact
	for i := range all {
		if all[i].Object == "Hyprland" {
			oldFact = &all[i]
		} else if all[i].Object == "Niri" {
			newFact = &all[i]
		}
	}

	if oldFact == nil || newFact == nil {
		t.Fatalf("could not locate both old and new facts")
	}
	if oldFact.ValidUntil == nil {
		t.Errorf("expected old fact ValidUntil to be set")
	}
	if oldFact.InvalidatedAt == nil {
		t.Errorf("expected old fact InvalidatedAt to be set")
	}
	if oldFact.SupersededBy == nil || *oldFact.SupersededBy != newFact.ID {
		t.Errorf("expected old fact SupersededBy = %s, got %v", newFact.ID, oldFact.SupersededBy)
	}
}

func TestGetFactsAt_TimeTravel(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	reg := NewRegistry(database)

	beforeT := time.Now().Add(-1 * time.Minute)

	f1, err := reg.AddFact("forex", "EURUSD", "trend", "bullish", "algo")
	if err != nil {
		t.Fatalf("AddFact 1 failed: %v", err)
	}

	// Midpoint time when f1 was active
	midpoint := time.Now()
	time.Sleep(20 * time.Millisecond)

	_, err = reg.AddFact("forex", "EURUSD", "trend", "bearish", "algo")
	if err != nil {
		t.Fatalf("AddFact 2 failed: %v", err)
	}

	// Before f1 was inserted
	factsBefore, err := reg.GetFactsAt("forex", beforeT)
	if err != nil {
		t.Fatalf("GetFactsAt before failed: %v", err)
	}
	if len(factsBefore) != 0 {
		t.Errorf("expected 0 facts before insertion, got %d", len(factsBefore))
	}

	// At midpoint, f1 should be active
	factsMid, err := reg.GetFactsAt("forex", midpoint)
	if err != nil {
		t.Fatalf("GetFactsAt midpoint failed: %v", err)
	}
	if len(factsMid) != 1 {
		t.Fatalf("expected 1 fact at midpoint, got %d", len(factsMid))
	}
	if factsMid[0].ID != f1.ID || factsMid[0].Object != "bullish" {
		t.Errorf("expected bullish fact at midpoint, got %s (%s)", factsMid[0].ID, factsMid[0].Object)
	}

	// Now (after f2)
	factsNow, err := reg.GetFactsAt("forex", time.Now())
	if err != nil {
		t.Fatalf("GetFactsAt now failed: %v", err)
	}
	if len(factsNow) != 1 {
		t.Fatalf("expected 1 fact now, got %d", len(factsNow))
	}
	if factsNow[0].Object != "bearish" {
		t.Errorf("expected bearish fact now, got %s", factsNow[0].Object)
	}
}

func TestInvalidateFact(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	reg := NewRegistry(database)

	f, err := reg.AddFact("ecommerce", "Order123", "status", "pending", "checkout")
	if err != nil {
		t.Fatalf("AddFact failed: %v", err)
	}

	if err := reg.InvalidateFact(f.ID); err != nil {
		t.Fatalf("InvalidateFact failed: %v", err)
	}

	active, err := reg.GetActiveFacts("ecommerce")
	if err != nil {
		t.Fatalf("GetActiveFacts failed: %v", err)
	}
	if len(active) != 0 {
		t.Errorf("expected 0 active facts after invalidation, got %d", len(active))
	}

	// Second invalidation should return error
	if err := reg.InvalidateFact(f.ID); err == nil {
		t.Error("expected error when invalidating already invalidated fact, got nil")
	}
}

func TestQueryFacts_Filters(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	reg := NewRegistry(database)

	_, _ = reg.AddFact("lit", "BookA", "genre", "SciFi", "catalog")
	_, _ = reg.AddFact("lit", "BookA", "author", "Asimov", "catalog")
	_, _ = reg.AddFact("lit", "BookB", "genre", "SciFi", "catalog")

	// Match namespace and subject
	bySubj, err := reg.QueryFacts("lit", "BookA", "")
	if err != nil {
		t.Fatalf("QueryFacts by subject failed: %v", err)
	}
	if len(bySubj) != 2 {
		t.Errorf("expected 2 facts for BookA, got %d", len(bySubj))
	}

	// Match namespace and predicate
	byPred, err := reg.QueryFacts("lit", "", "genre")
	if err != nil {
		t.Fatalf("QueryFacts by predicate failed: %v", err)
	}
	if len(byPred) != 2 {
		t.Errorf("expected 2 facts for predicate 'genre', got %d", len(byPred))
	}

	// Match namespace, subject, and predicate
	exact, err := reg.QueryFacts("lit", "BookA", "author")
	if err != nil {
		t.Fatalf("QueryFacts exact failed: %v", err)
	}
	if len(exact) != 1 || exact[0].Object != "Asimov" {
		t.Errorf("expected exact match for Asimov, got %v", exact)
	}
}
