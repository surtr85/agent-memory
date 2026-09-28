package pipeline

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/surtr85/agent-memory/internal/blocks"
	"github.com/surtr85/agent-memory/internal/temporal"
)

// Observation represents a raw contextual signal captured during agent runtime.
type Observation struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"`
	Content   string    `json:"content"`
	Namespace string    `json:"namespace"`
	Status    string    `json:"status"` // 'unconsolidated', 'consolidated'
	CreatedAt time.Time `json:"created_at"`
}

// RecordObservation stores a raw runtime observation for later consolidation.
func RecordObservation(db *sql.DB, category, content, namespace string) error {
	if namespace == "" {
		namespace = "default"
	}
	if category == "" {
		category = "general"
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("observation content cannot be empty")
	}

	hasher := sha256.New()
	hasher.Write([]byte(fmt.Sprintf("%s:%s:%s:%d", category, content, namespace, time.Now().UnixNano())))
	id := hex.EncodeToString(hasher.Sum(nil))[:16]

	_, err := db.Exec(`
		INSERT INTO observations (id, category, content, namespace, status)
		VALUES (?, ?, ?, ?, 'unconsolidated')
	`, id, category, content, namespace)

	return err
}

// ListObservations returns observations for a namespace (or all if namespace is empty).
func ListObservations(db *sql.DB, namespace string) ([]Observation, error) {
	query := `SELECT id, category, content, namespace, status, created_at FROM observations`
	var args []interface{}
	if namespace != "" {
		query += ` WHERE namespace = ?`
		args = append(args, namespace)
	}
	query += ` ORDER BY created_at ASC`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var observations []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.Category, &o.Content, &o.Namespace, &o.Status, &o.CreatedAt); err != nil {
			return nil, err
		}
		observations = append(observations, o)
	}

	return observations, nil
}

var (
	// Matches triples like "User prefers Neovim" or "Niri is a Wayland compositor"
	// format: [Subject] [predicate] [Object]
	factPattern = regexp.MustCompile(`^([\w\.\-]+)\s+([\w\.\-]+)\s+(.+)$`)
)

// ConsolidateObservations processes all 'unconsolidated' observations:
// - Parses actionable signals into active facts (bi-temporal registry) or core blocks
// - Marks processed observations as 'consolidated'
func ConsolidateObservations(db *sql.DB) error {
	rows, err := db.Query(`SELECT id, category, content, namespace FROM observations WHERE status = 'unconsolidated'`)
	if err != nil {
		return fmt.Errorf("failed fetching unconsolidated observations: %w", err)
	}
	defer rows.Close()

	type obsRecord struct {
		id        string
		category  string
		content   string
		namespace string
	}
	var pending []obsRecord
	for rows.Next() {
		var o obsRecord
		if err := rows.Scan(&o.id, &o.category, &o.content, &o.namespace); err != nil {
			return err
		}
		pending = append(pending, o)
	}
	rows.Close()

	if len(pending) == 0 {
		return nil
	}

	reg := temporal.NewRegistry(db)
	blockMgr := blocks.NewManager(db)

	for _, p := range pending {
		switch strings.ToLower(p.category) {
		case "human", "persona", "environment", "scratchpad":
			// Update core block
			_ = blockMgr.AppendBlock(p.category, "\n"+p.content)

		case "fact", "assertion":
			// Parse triple
			matches := factPattern.FindStringSubmatch(p.content)
			if len(matches) == 4 {
				subj := strings.TrimSpace(matches[1])
				pred := strings.TrimSpace(matches[2])
				obj := strings.TrimSpace(matches[3])
				_, _ = reg.AddFact(p.namespace, subj, pred, obj, "observation:"+p.id)
			} else {
				// Fallback triple
				_, _ = reg.AddFact(p.namespace, "System", "observed", p.content, "observation:"+p.id)
			}

		default:
			// General category: try parsing triple, if not store as observed fact
			matches := factPattern.FindStringSubmatch(p.content)
			if len(matches) == 4 {
				subj := strings.TrimSpace(matches[1])
				pred := strings.TrimSpace(matches[2])
				obj := strings.TrimSpace(matches[3])
				_, _ = reg.AddFact(p.namespace, subj, pred, obj, "observation:"+p.id)
			} else {
				_, _ = reg.AddFact(p.namespace, "System", "noted", p.content, "observation:"+p.id)
			}
		}

		// Mark observation as consolidated
		_, err := db.Exec(`UPDATE observations SET status = 'consolidated' WHERE id = ?`, p.id)
		if err != nil {
			return fmt.Errorf("failed marking observation %s consolidated: %w", p.id, err)
		}
	}

	return nil
}
