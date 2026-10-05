package forget

import (
	"context"
	"database/sql"
	"testing"

	"github.com/surtr85/agent-memory/internal/config"
	"github.com/surtr85/agent-memory/internal/db"
	"github.com/surtr85/agent-memory/internal/decision"
	"github.com/surtr85/agent-memory/internal/temporal"
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

func TestSafeForgettingPipeline(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	cfg := config.LoadConfig()
	dec := decision.NewEngine(cfg)
	reg := temporal.NewRegistry(database)
	pipeline := NewPipeline(database, dec)

	// Seed test fact and entity
	f, err := reg.AddFact("forex", "EURUSD", "broker_lot_limit", "50", "manual_test")
	if err != nil {
		t.Fatalf("AddFact failed: %v", err)
	}

	_, _ = database.Exec(`INSERT INTO entities (name, entity_type, namespace) VALUES ('EURUSD', 'symbol', 'forex')`)
	_, _ = database.Exec(`INSERT INTO entity_relations (id, source_entity, target_entity, relation_type) VALUES ('rel1', 'EURUSD', 'EURUSD', 'self')`)

	// 1. Stage Forget
	ctx := context.Background()
	stageRes, err := pipeline.StageForget(ctx, "broker_lot_limit", "forex")
	if err != nil {
		t.Fatalf("StageForget failed: %v", err)
	}
	if stageRes.ItemCount == 0 || !stageRes.SafetyApproved {
		t.Fatalf("expected items to be staged and safety approved, got %+v", stageRes)
	}

	// 2. Execute Forget
	receipt, err := pipeline.ExecuteForget(ctx, stageRes.StageID)
	if err != nil {
		t.Fatalf("ExecuteForget failed: %v", err)
	}
	if receipt.RetractedFacts != 1 || receipt.TombstonesCreated == 0 {
		t.Errorf("unexpected receipt metrics: %+v", receipt)
	}

	// 3. Verify Fact is now invalidated
	active, err := reg.GetActiveFacts("forex")
	if err != nil {
		t.Fatalf("GetActiveFacts failed: %v", err)
	}
	for _, act := range active {
		if act.ID == f.ID {
			t.Errorf("fact %s was expected to be retracted, but still active", f.ID)
		}
	}

	// 4. Test Safety Gate Protection
	protectedRes, err := pipeline.StageForget(ctx, "Amadeus", "")
	if err != nil {
		t.Fatalf("StageForget protected failed: %v", err)
	}
	if protectedRes.SafetyApproved {
		t.Errorf("expected protected core entity 'Amadeus' to be rejected by safety gate")
	}
}
