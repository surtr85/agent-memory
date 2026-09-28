package blocks

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// CoreBlock represents an in-context working memory block.
type CoreBlock struct {
	ID        string    `json:"id"`
	Label     string    `json:"label"`
	Content   string    `json:"content"`
	MaxTokens int       `json:"max_tokens"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Manager handles CRUD and prompt generation for core memory blocks.
type Manager struct {
	db *sql.DB
}

// NewManager creates a new Manager instance.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// GetBlock retrieves a core memory block by its label.
func (m *Manager) GetBlock(label string) (*CoreBlock, error) {
	row := m.db.QueryRow(
		"SELECT id, label, content, max_tokens, updated_at FROM core_blocks WHERE label = ?",
		label,
	)

	var block CoreBlock
	var updatedAtStr string
	if err := row.Scan(&block.ID, &block.Label, &block.Content, &block.MaxTokens, &updatedAtStr); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("core block %q not found", label)
		}
		return nil, fmt.Errorf("failed to get core block %q: %w", label, err)
	}

	block.UpdatedAt = parseTime(updatedAtStr)
	return &block, nil
}

// SetBlock creates or updates a core block with full replacement of content.
func (m *Manager) SetBlock(label, content string) error {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339)

	var existingID string
	err := m.db.QueryRow("SELECT id FROM core_blocks WHERE label = ?", label).Scan(&existingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed checking block existence: %w", err)
	}

	if err == sql.ErrNoRows {
		newID := uuid.New().String()
		_, err = m.db.Exec(
			"INSERT INTO core_blocks (id, label, content, max_tokens, updated_at) VALUES (?, ?, ?, ?, ?)",
			newID, label, content, 500, nowStr,
		)
		if err != nil {
			return fmt.Errorf("failed inserting core block %q: %w", label, err)
		}
		return nil
	}

	_, err = m.db.Exec(
		"UPDATE core_blocks SET content = ?, updated_at = ? WHERE label = ?",
		content, nowStr, label,
	)
	if err != nil {
		return fmt.Errorf("failed updating core block %q: %w", label, err)
	}
	return nil
}

// ReplaceBlock replaces oldContent with newContent in the block identified by label.
func (m *Manager) ReplaceBlock(label, oldContent, newContent string) error {
	block, err := m.GetBlock(label)
	if err != nil {
		return err
	}

	if !strings.Contains(block.Content, oldContent) {
		return fmt.Errorf("target string not found in block %q", label)
	}

	updated := strings.Replace(block.Content, oldContent, newContent, 1)
	return m.SetBlock(label, updated)
}

// AppendBlock appends content to the specified block (with newline separator if non-empty).
func (m *Manager) AppendBlock(label, content string) error {
	block, err := m.GetBlock(label)
	if err != nil {
		// If block does not exist, initialize it
		return m.SetBlock(label, content)
	}

	var updated string
	if strings.TrimSpace(block.Content) == "" {
		updated = content
	} else {
		updated = block.Content + "\n" + content
	}
	return m.SetBlock(label, updated)
}

// ListBlocks returns all core memory blocks ordered by label.
func (m *Manager) ListBlocks() ([]CoreBlock, error) {
	rows, err := m.db.Query("SELECT id, label, content, max_tokens, updated_at FROM core_blocks ORDER BY label ASC")
	if err != nil {
		return nil, fmt.Errorf("failed listing core blocks: %w", err)
	}
	defer rows.Close()

	var blocks []CoreBlock
	for rows.Next() {
		var b CoreBlock
		var updatedAtStr string
		if err := rows.Scan(&b.ID, &b.Label, &b.Content, &b.MaxTokens, &updatedAtStr); err != nil {
			return nil, fmt.Errorf("failed scanning core block: %w", err)
		}
		b.UpdatedAt = parseTime(updatedAtStr)
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}
	return blocks, nil
}

// GetBootstrapPrompt generates compact markdown for agent system prompt injection (< 400 tokens).
func (m *Manager) GetBootstrapPrompt() (string, error) {
	blocks, err := m.ListBlocks()
	if err != nil {
		return "", err
	}

	if len(blocks) == 0 {
		return "", nil
	}

	var sb strings.Builder
	sb.WriteString("### Core Memory Context\n")
	for _, b := range blocks {
		sb.WriteString(fmt.Sprintf("<%s>\n%s\n</%s>\n", b.Label, strings.TrimSpace(b.Content), b.Label))
	}
	return strings.TrimSpace(sb.String()), nil
}

// SeedDefaults seeds default 'human', 'persona', and 'environment' blocks if table is empty.
func (m *Manager) SeedDefaults() error {
	var count int
	err := m.db.QueryRow("SELECT COUNT(*) FROM core_blocks").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed checking core_blocks count: %w", err)
	}

	if count > 0 {
		return nil
	}

	defaults := []struct {
		label   string
		content string
	}{
		{
			label: "persona",
			content: "I am Amadeus / OmniMem, a highly capable, autonomous, and precise AI systems architect and programming partner. " +
				"I speak with conciseness, technical rigor, and zero unnecessary fluff. I act as an elite peer.",
		},
		{
			label: "human",
			content: "User is Sajjad: Full-stack engineer, systems architect, and algorithmic trader. " +
				"Values high-density answers, clean idiomatic code, robust testing, and deep architectural clarity.",
		},
		{
			label: "environment",
			content: "NixOS Linux (x86_64), Niri Wayland scrollable compositor, Alacritty terminal, Neovim / Claude Code. " +
				"Pure-Go zero-CGO binaries, local SQLite storage.",
		},
	}

	for _, d := range defaults {
		if err := m.SetBlock(d.label, d.content); err != nil {
			return fmt.Errorf("failed seeding default block %q: %w", d.label, err)
		}
	}

	return nil
}

func parseTime(s string) time.Time {
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
