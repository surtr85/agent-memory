package alignment

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// GuidanceItem represents a standing preference, boundary, or principle.
type GuidanceItem struct {
	ID        string    `json:"id"`
	Category  string    `json:"category"` // 'boundary', 'preference', 'friction', 'principle'
	Guidance  string    `json:"guidance"`
	Salience  float64   `json:"salience"`
	Status    string    `json:"status"` // 'active', 'repaired', 'archived'
	UpdatedAt time.Time `json:"updated_at"`
}

// RepairThread tracks active or resolved friction points between agent and user.
type RepairThread struct {
	ID              string     `json:"id"`
	TriggerSummary  string     `json:"trigger_summary"`
	AgentAdjustment string     `json:"agent_adjustment"`
	Status          string     `json:"status"` // 'open', 'in_progress', 'resolved'
	CreatedAt       time.Time  `json:"created_at"`
	ResolvedAt      *time.Time `json:"resolved_at,omitempty"`
}

// Manager orchestrates standing guidance and repair threads.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new Alignment Manager.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// AddGuidance registers or updates an alignment guidance rule.
func (m *Manager) AddGuidance(category, guidance string, salience float64) (*GuidanceItem, error) {
	if category == "" {
		category = "preference"
	}
	guidance = strings.TrimSpace(guidance)
	if guidance == "" {
		return nil, fmt.Errorf("guidance text cannot be empty")
	}
	if salience <= 0 {
		salience = 1.0
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	_, err := m.db.Exec(`
		INSERT INTO alignment_state (id, category, guidance, salience, status, updated_at)
		VALUES (?, ?, ?, ?, 'active', ?)
	`, id, category, guidance, salience, nowStr)
	if err != nil {
		return nil, fmt.Errorf("failed adding guidance: %w", err)
	}

	return &GuidanceItem{
		ID:        id,
		Category:  category,
		Guidance:  guidance,
		Salience:  salience,
		Status:    "active",
		UpdatedAt: now,
	}, nil
}

// RecordRepairThread opens a new repair thread when friction occurs.
func (m *Manager) RecordRepairThread(triggerSummary, agentAdjustment string) (*RepairThread, error) {
	triggerSummary = strings.TrimSpace(triggerSummary)
	agentAdjustment = strings.TrimSpace(agentAdjustment)
	if triggerSummary == "" || agentAdjustment == "" {
		return nil, fmt.Errorf("trigger_summary and agent_adjustment cannot be empty")
	}

	id := uuid.New().String()
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	_, err := m.db.Exec(`
		INSERT INTO repair_threads (id, trigger_summary, agent_adjustment, status, created_at)
		VALUES (?, ?, ?, 'open', ?)
	`, id, triggerSummary, agentAdjustment, nowStr)
	if err != nil {
		return nil, fmt.Errorf("failed creating repair thread: %w", err)
	}

	// Also register high-salience friction guidance in alignment_state
	_, _ = m.AddGuidance("friction", fmt.Sprintf("Avoid: %s | Adjustment: %s", triggerSummary, agentAdjustment), 1.2)

	return &RepairThread{
		ID:              id,
		TriggerSummary:  triggerSummary,
		AgentAdjustment: agentAdjustment,
		Status:          "open",
		CreatedAt:       now,
	}, nil
}

// ResolveRepairThread marks a repair thread as resolved.
func (m *Manager) ResolveRepairThread(id string) error {
	nowStr := time.Now().UTC().Format(time.RFC3339)
	res, err := m.db.Exec(`
		UPDATE repair_threads 
		SET status = 'resolved', resolved_at = ? 
		WHERE id = ? AND status != 'resolved'
	`, nowStr, id)
	if err != nil {
		return fmt.Errorf("failed resolving repair thread: %w", err)
	}
	aff, err := res.RowsAffected()
	if err != nil || aff == 0 {
		return fmt.Errorf("repair thread %s not found or already resolved", id)
	}
	return nil
}

// GetActiveGuidance returns all active guidance items.
func (m *Manager) GetActiveGuidance() ([]GuidanceItem, error) {
	rows, err := m.db.Query(`
		SELECT id, category, guidance, salience, status, updated_at 
		FROM alignment_state 
		WHERE status = 'active'
		ORDER BY salience DESC, updated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []GuidanceItem
	for rows.Next() {
		var item GuidanceItem
		var updatedStr string
		if err := rows.Scan(&item.ID, &item.Category, &item.Guidance, &item.Salience, &item.Status, &updatedStr); err != nil {
			return nil, err
		}
		item.UpdatedAt = parseTime(updatedStr)
		items = append(items, item)
	}
	return items, nil
}

// GetOpenRepairThreads returns all unresolved repair threads.
func (m *Manager) GetOpenRepairThreads() ([]RepairThread, error) {
	rows, err := m.db.Query(`
		SELECT id, trigger_summary, agent_adjustment, status, created_at, resolved_at 
		FROM repair_threads 
		WHERE status != 'resolved'
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []RepairThread
	for rows.Next() {
		var rt RepairThread
		var createdStr string
		var resStr sql.NullString
		if err := rows.Scan(&rt.ID, &rt.TriggerSummary, &rt.AgentAdjustment, &rt.Status, &createdStr, &resStr); err != nil {
			return nil, err
		}
		rt.CreatedAt = parseTime(createdStr)
		if resStr.Valid {
			t := parseTime(resStr.String)
			rt.ResolvedAt = &t
		}
		threads = append(threads, rt)
	}
	return threads, nil
}

// GenerateSynthesisPrompt generates compact Markdown guidance (< 60 tokens) for prompt injection.
// Following Muse architecture: prompt_hoisted: true for synthesis, avoiding heavy dream prose.
func (m *Manager) GenerateSynthesisPrompt() (string, error) {
	items, err := m.GetActiveGuidance()
	if err != nil {
		return "", err
	}
	threads, err := m.GetOpenRepairThreads()
	if err != nil {
		return "", err
	}

	if len(items) == 0 && len(threads) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("<alignment_synthesis>\n")

	// 1. Standing Boundaries & Priorities
	var activeRules []string
	for _, it := range items {
		activeRules = append(activeRules, fmt.Sprintf("- [%s] %s", it.Category, it.Guidance))
		if len(activeRules) >= 4 { // Keep strictly within token budget
			break
		}
	}
	if len(activeRules) > 0 {
		sb.WriteString(strings.Join(activeRules, "\n") + "\n")
	}

	// 2. Active Repair / Friction Threads
	if len(threads) > 0 {
		sb.WriteString("Active Adjustments:\n")
		for _, t := range threads {
			sb.WriteString(fmt.Sprintf("- Fix: %s -> %s\n", t.TriggerSummary, t.AgentAdjustment))
			if len(threads) >= 2 {
				break
			}
		}
	}

	sb.WriteString("</alignment_synthesis>")
	return strings.TrimSpace(sb.String()), nil
}

func parseTime(s string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
