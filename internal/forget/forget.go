package forget

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/surtr85/agent-memory/internal/decision"
	"github.com/surtr85/agent-memory/internal/temporal"
)

// StagedItem represents an identified memory element marked for retraction.
type StagedItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "fact", "chunk", "entity", "observation"
	Namespace string `json:"namespace"`
	Summary   string `json:"summary"`
}

// StageResult encapsulates the staging proposal before physical retraction.
type StageResult struct {
	StageID        string       `json:"stage_id"`
	TargetPattern  string       `json:"target_pattern"`
	ItemCount      int          `json:"item_count"`
	Items          []StagedItem `json:"items"`
	SafetyApproved bool         `json:"safety_approved"`
	SafetyReason   string       `json:"safety_reason"`
}

// RetractionReceipt contains the audit report of executed forget actions.
type RetractionReceipt struct {
	StageID           string    `json:"stage_id"`
	RetractedFacts    int       `json:"retracted_facts"`
	RetractedChunks   int       `json:"retracted_chunks"`
	RetractedEntities int       `json:"retracted_entities"`
	TombstonesCreated int       `json:"tombstones_created"`
	Timestamp         time.Time `json:"timestamp"`
}

// Pipeline orchestrates the 4-stage safe forgetting workflow.
type Pipeline struct {
	db       *sql.DB
	decision *decision.Engine
	registry *temporal.Registry
}

// NewPipeline creates a new Safe Forgetting pipeline.
func NewPipeline(db *sql.DB, dec *decision.Engine) *Pipeline {
	return &Pipeline{
		db:       db,
		decision: dec,
		registry: temporal.NewRegistry(db).WithDecision(dec),
	}
}

// Stage 1 & 2: Search target pattern, stage items, run Laya System-1 safety gate
func (p *Pipeline) StageForget(ctx context.Context, targetPattern, namespace string) (*StageResult, error) {
	targetPattern = strings.TrimSpace(targetPattern)
	if targetPattern == "" {
		return nil, fmt.Errorf("target pattern cannot be empty")
	}

	var staged []StagedItem
	patternQuery := "%" + strings.ToLower(targetPattern) + "%"

	// 1. Search matching active facts
	factQuery := `SELECT id, namespace, subject, predicate, object FROM facts 
	              WHERE valid_until IS NULL AND (LOWER(subject) LIKE ? OR LOWER(predicate) LIKE ? OR LOWER(object) LIKE ?)`
	var factArgs []interface{}
	factArgs = append(factArgs, patternQuery, patternQuery, patternQuery)
	if namespace != "" {
		factQuery += " AND namespace = ?"
		factArgs = append(factArgs, namespace)
	}

	fRows, err := p.db.QueryContext(ctx, factQuery, factArgs...)
	if err == nil {
		defer fRows.Close()
		for fRows.Next() {
			var id, ns, subj, pred, obj string
			if err := fRows.Scan(&id, &ns, &subj, &pred, &obj); err == nil {
				staged = append(staged, StagedItem{
					ID:        id,
					Type:      "fact",
					Namespace: ns,
					Summary:   fmt.Sprintf("(%s) -[%s]-> (%s)", subj, pred, obj),
				})
			}
		}
	}

	// 2. Search matching chunks
	chunkQuery := `SELECT id, namespace, title FROM chunks WHERE (LOWER(title) LIKE ? OR LOWER(content) LIKE ?)`
	var chunkArgs []interface{}
	chunkArgs = append(chunkArgs, patternQuery, patternQuery)
	if namespace != "" {
		chunkQuery += " AND namespace = ?"
		chunkArgs = append(chunkArgs, namespace)
	}
	cRows, err := p.db.QueryContext(ctx, chunkQuery, chunkArgs...)
	if err == nil {
		defer cRows.Close()
		for cRows.Next() {
			var id, ns, title string
			if err := cRows.Scan(&id, &ns, &title); err == nil {
				staged = append(staged, StagedItem{
					ID:        id,
					Type:      "chunk",
					Namespace: ns,
					Summary:   fmt.Sprintf("Chunk [%s]: %s", ns, title),
				})
			}
		}
	}

	// 3. Search matching entities
	entQuery := `SELECT name, namespace, entity_type FROM entities WHERE LOWER(name) LIKE ?`
	var entArgs []interface{}
	entArgs = append(entArgs, patternQuery)
	if namespace != "" {
		entQuery += " AND namespace = ?"
		entArgs = append(entArgs, namespace)
	}
	eRows, err := p.db.QueryContext(ctx, entQuery, entArgs...)
	if err == nil {
		defer eRows.Close()
		for eRows.Next() {
			var name, ns, eType string
			if err := eRows.Scan(&name, &ns, &eType); err == nil {
				staged = append(staged, StagedItem{
					ID:        name,
					Type:      "entity",
					Namespace: ns,
					Summary:   fmt.Sprintf("Entity (%s) [%s]", name, eType),
				})
			}
		}
	}

	// Stage 2: Laya System-1 Safety Check
	safetyApproved := true
	safetyReason := "Approved for retraction"

	// Guard against accidental root deletion of vital entities
	low := strings.ToLower(targetPattern)
	if low == "user" || low == "persona" || low == "amadeus" || low == "sajjad" || low == "system" {
		safetyApproved = false
		safetyReason = "Rejected by safety gate: protected core identity or system entity"
	}

	stageID := uuid.New().String()
	itemsJSON, _ := json.Marshal(staged)

	status := "pending"
	if !safetyApproved {
		status = "rejected"
	}

	_, err = p.db.ExecContext(ctx, `
		INSERT INTO forget_staging (id, target_pattern, item_type, status, staged_items_json)
		VALUES (?, ?, 'mixed', ?, ?)
	`, stageID, targetPattern, status, string(itemsJSON))
	if err != nil {
		return nil, fmt.Errorf("failed saving forget staging: %w", err)
	}

	return &StageResult{
		StageID:        stageID,
		TargetPattern:  targetPattern,
		ItemCount:      len(staged),
		Items:          staged,
		SafetyApproved: safetyApproved,
		SafetyReason:   safetyReason,
	}, nil
}

// Stage 3 & 4: Execute Retraction & Record Tombstones
func (p *Pipeline) ExecuteForget(ctx context.Context, stageID string) (*RetractionReceipt, error) {
	var targetPattern, status, itemsJSON string
	row := p.db.QueryRowContext(ctx, `SELECT target_pattern, status, staged_items_json FROM forget_staging WHERE id = ?`, stageID)
	if err := row.Scan(&targetPattern, &status, &itemsJSON); err != nil {
		return nil, fmt.Errorf("staging entry %s not found: %w", stageID, err)
	}

	if status == "rejected" {
		return nil, fmt.Errorf("cannot execute stage %s: rejected by safety guard", stageID)
	}
	if status == "executed" {
		return nil, fmt.Errorf("stage %s has already been executed", stageID)
	}

	var items []StagedItem
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return nil, fmt.Errorf("corrupted staging data: %w", err)
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	receipt := &RetractionReceipt{
		StageID:   stageID,
		Timestamp: now,
	}

	for _, it := range items {
		// Create tombstone signature to permanently forbid zombie re-ingestion
		h := sha256.New()
		h.Write([]byte(fmt.Sprintf("%s:%s:%s", it.Type, it.Namespace, it.Summary)))
		sig := hex.EncodeToString(h.Sum(nil))

		_, _ = tx.ExecContext(ctx, `
			INSERT INTO tombstones (hash_signature, original_id, item_type, reason, retracted_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT(hash_signature) DO NOTHING
		`, sig, it.ID, it.Type, "Forced retraction via stage "+stageID, nowStr)
		receipt.TombstonesCreated++

		switch it.Type {
		case "fact":
			// Invalidate fact bi-temporally
			res, err := tx.ExecContext(ctx, `
				UPDATE facts 
				SET valid_until = ?, invalidated_at = ? 
				WHERE id = ? AND valid_until IS NULL
			`, nowStr, nowStr, it.ID)
			if err == nil {
				aff, _ := res.RowsAffected()
				receipt.RetractedFacts += int(aff)
			}

		case "chunk":
			// Delete chunk and cascading vector embedding
			_, _ = tx.ExecContext(ctx, `DELETE FROM vector_embeddings WHERE chunk_id = ?`, it.ID)
			res, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE id = ?`, it.ID)
			if err == nil {
				aff, _ := res.RowsAffected()
				receipt.RetractedChunks += int(aff)
			}

		case "entity":
			// Sever entity relations
			_, _ = tx.ExecContext(ctx, `DELETE FROM entity_relations WHERE source_entity = ? OR target_entity = ?`, it.ID, it.ID)
			res, err := tx.ExecContext(ctx, `DELETE FROM entities WHERE name = ?`, it.ID)
			if err == nil {
				aff, _ := res.RowsAffected()
				receipt.RetractedEntities += int(aff)
			}
		}
	}

	// Update stage status
	_, err = tx.ExecContext(ctx, `UPDATE forget_staging SET status = 'executed' WHERE id = ?`, stageID)
	if err != nil {
		return nil, fmt.Errorf("failed updating staging status: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("retraction commit failed: %w", err)
	}

	return receipt, nil
}

// IsTombstoned checks if an incoming item matches a retracted tombstone.
func (p *Pipeline) IsTombstoned(itemType, namespace, content string) bool {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%s:%s:%s", itemType, namespace, content)))
	sig := hex.EncodeToString(h.Sum(nil))

	var count int
	_ = p.db.QueryRow(`SELECT COUNT(*) FROM tombstones WHERE hash_signature = ?`, sig).Scan(&count)
	return count > 0
}
