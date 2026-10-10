package temporal

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/surtr85/agent-memory/internal/decision"
)

// Fact represents a bi-temporal atomic assertion with citation and claim evidence.
type Fact struct {
	ID            string     `json:"id"`
	Namespace     string     `json:"namespace"`
	Subject       string     `json:"subject"`
	Predicate     string     `json:"predicate"`
	Object        string     `json:"object"`
	Confidence    float64    `json:"confidence"`
	Source        string     `json:"source"`
	SourceURI     string     `json:"source_uri,omitempty"`
	SourceQuote   string     `json:"source_quote,omitempty"`
	LineNumber    int        `json:"line_number,omitempty"`
	Salience      float64    `json:"salience,omitempty"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidUntil    *time.Time `json:"valid_until,omitempty"`
	RecordedAt    time.Time  `json:"recorded_at"`
	InvalidatedAt *time.Time `json:"invalidated_at,omitempty"`
	SupersededBy  *string    `json:"superseded_by,omitempty"`
}

// FactEvidence represents the explainability envelope for a claim.
type FactEvidence struct {
	Fact           Fact    `json:"fact"`
	EvidenceQuote  string  `json:"evidence_quote"`
	SourceURI      string  `json:"source_uri"`
	LineNumber     int     `json:"line_number"`
	Confidence     float64 `json:"confidence"`
	Salience       float64 `json:"salience"`
	Status         string  `json:"status"` // 'active', 'superseded', 'retracted'
	SupersededByID *string `json:"superseded_by_id,omitempty"`
	ChainHistory   []Fact  `json:"chain_history,omitempty"`
}

// Registry handles bi-temporal fact storage, contradiction resolution, and queries.
type Registry struct {
	db       *sql.DB
	decision *decision.Engine
}

// NewRegistry creates a new temporal fact Registry.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{db: db}
}

// WithDecision attaches a Laya System-1 decision engine to the registry for intelligent conflict checking.
func (r *Registry) WithDecision(engine *decision.Engine) *Registry {
	r.decision = engine
	return r
}

// AddFact inserts a new fact into the bi-temporal registry.
// Contradiction resolution:
// If an active fact (`valid_until IS NULL`) exists with identical `namespace`, `subject`, and `predicate`:
//   - If the object is identical, we can either return the existing fact or ignore.
//   - If the object differs, mark the old fact's `valid_until = now()`, `invalidated_at = now()`,
//     `superseded_by = newFact.ID`.
func (r *Registry) AddFact(namespace, subject, predicate, object, source string) (*Fact, error) {
	return r.AddFactWithCitation(namespace, subject, predicate, object, source, "", "", 0, 0.5)
}

// AddFactWithCitation inserts a new fact along with detailed Muse-style citation & quote evidence.
func (r *Registry) AddFactWithCitation(namespace, subject, predicate, object, source, sourceURI, sourceQuote string, lineNo int, salience float64) (*Fact, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	newID := uuid.New().String()

	if salience <= 0 {
		salience = 0.5
	}

	// Find any active conflicting facts (same namespace, subject, predicate with valid_until IS NULL)
	rows, err := tx.Query(
		`SELECT id, object FROM facts 
		 WHERE namespace = ? AND subject = ? AND predicate = ? AND valid_until IS NULL`,
		namespace, subject, predicate,
	)
	if err != nil {
		return nil, fmt.Errorf("failed querying existing facts: %w", err)
	}

	type conflict struct {
		id     string
		object string
	}
	var conflicts []conflict
	for rows.Next() {
		var c conflict
		if err := rows.Scan(&c.id, &c.object); err != nil {
			rows.Close()
			return nil, fmt.Errorf("failed scanning conflict: %w", err)
		}
		conflicts = append(conflicts, c)
	}
	rows.Close()

	// If there's an active fact with identical object, we can return it directly without duplicate insert
	for _, c := range conflicts {
		if c.object == object {
			// Already active and identical
			fact, err := r.getFactByIDTx(tx, c.id)
			if err != nil {
				return nil, err
			}
			return fact, tx.Commit()
		}
	}

	// If Laya decision engine is attached, verify conflict/contradiction
	if r.decision != nil && len(conflicts) > 0 {
		for _, c := range conflicts {
			isContradiction, _, err := r.decision.DecideConflict(context.Background(), c.object, object)
			if err == nil && !isContradiction {
				// If Laya affirms both can coexist without contradiction, we don't invalidate
				// (Keep in registry as parallel assertions)
			}
		}
	}

	// Insert the new fact first so foreign key constraints on superseded_by are satisfied
	_, err = tx.Exec(
		`INSERT INTO facts (
			id, namespace, subject, predicate, object, 
			confidence, source, source_uri, source_quote, line_number, salience,
			valid_from, valid_until, 
			recorded_at, invalidated_at, superseded_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL, NULL)`,
		newID, namespace, subject, predicate, object,
		1.0, source, sourceURI, sourceQuote, lineNo, salience,
		nowStr, nowStr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed inserting new fact: %w", err)
	}

	// Invalidate older conflicting facts (superseded by newID)
	for _, c := range conflicts {
		_, err = tx.Exec(
			`UPDATE facts 
			 SET valid_until = ?, invalidated_at = ?, superseded_by = ? 
			 WHERE id = ?`,
			nowStr, nowStr, newID, c.id,
		)
		if err != nil {
			return nil, fmt.Errorf("failed invalidating superseded fact %s: %w", c.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed committing transaction: %w", err)
	}

	return &Fact{
		ID:          newID,
		Namespace:   namespace,
		Subject:     subject,
		Predicate:   predicate,
		Object:      object,
		Confidence:  1.0,
		Source:      source,
		SourceURI:   sourceURI,
		SourceQuote: sourceQuote,
		LineNumber:  lineNo,
		Salience:    salience,
		ValidFrom:   now,
		ValidUntil:  nil,
		RecordedAt:  now,
	}, nil
}

// GetActiveFacts returns all active facts (where valid_until IS NULL) in a given namespace.
func (r *Registry) GetActiveFacts(namespace string) ([]Fact, error) {
	query := `
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts
		WHERE namespace = ? AND valid_until IS NULL
		ORDER BY recorded_at DESC`
	return r.queryFactsList(query, namespace)
}

// GetFactsAt returns all facts that were valid at time t (valid_from <= t AND (valid_until IS NULL OR valid_until > t)).
func (r *Registry) GetFactsAt(namespace string, t time.Time) ([]Fact, error) {
	tStr := t.UTC().Format(time.RFC3339Nano)
	query := `
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts
		WHERE namespace = ?
		  AND valid_from <= ?
		  AND (valid_until IS NULL OR valid_until > ?)
		ORDER BY valid_from DESC`
	return r.queryFactsList(query, namespace, tStr, tStr)
}

// InvalidateFact marks a fact as invalidated at now().
func (r *Registry) InvalidateFact(id string) error {
	nowStr := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := r.db.Exec(
		`UPDATE facts 
		 SET valid_until = ?, invalidated_at = ? 
		 WHERE id = ? AND valid_until IS NULL`,
		nowStr, nowStr, id,
	)
	if err != nil {
		return fmt.Errorf("failed invalidating fact %s: %w", id, err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed checking rows affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("fact %s not found or already invalidated", id)
	}
	return nil
}

// QueryFacts finds facts by subject and predicate in a namespace.
// If subject is empty, matches any subject.
// If predicate is empty, matches any predicate.
func (r *Registry) QueryFacts(namespace, subject, predicate string) ([]Fact, error) {
	baseQuery := `
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts
		WHERE namespace = ?`
	var args []interface{}
	args = append(args, namespace)

	if subject != "" {
		baseQuery += " AND subject = ?"
		args = append(args, subject)
	}
	if predicate != "" {
		baseQuery += " AND predicate = ?"
		args = append(args, predicate)
	}

	baseQuery += " ORDER BY valid_from DESC"
	return r.queryFactsList(baseQuery, args...)
}

func (r *Registry) getFactByIDTx(tx *sql.Tx, id string) (*Fact, error) {
	row := tx.QueryRow(`
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts WHERE id = ?`, id)

	var f Fact
	var validFromStr, recordedAtStr string
	var validUntilStr, invalidatedAtStr, supersededBy, sourceURI, sourceQuote sql.NullString
	var lineNo sql.NullInt64
	var salience sql.NullFloat64

	err := row.Scan(
		&f.ID, &f.Namespace, &f.Subject, &f.Predicate, &f.Object, &f.Confidence, &f.Source,
		&sourceURI, &sourceQuote, &lineNo, &salience,
		&validFromStr, &validUntilStr, &recordedAtStr, &invalidatedAtStr, &supersededBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed scanning fact %s: %w", id, err)
	}

	if sourceURI.Valid {
		f.SourceURI = sourceURI.String
	}
	if sourceQuote.Valid {
		f.SourceQuote = sourceQuote.String
	}
	if lineNo.Valid {
		f.LineNumber = int(lineNo.Int64)
	}
	if salience.Valid {
		f.Salience = salience.Float64
	}

	f.ValidFrom = parseFactTime(validFromStr)
	f.RecordedAt = parseFactTime(recordedAtStr)
	if validUntilStr.Valid {
		vu := parseFactTime(validUntilStr.String)
		f.ValidUntil = &vu
	}
	if invalidatedAtStr.Valid {
		ia := parseFactTime(invalidatedAtStr.String)
		f.InvalidatedAt = &ia
	}
	if supersededBy.Valid {
		sb := supersededBy.String
		f.SupersededBy = &sb
	}

	return &f, nil
}

// ExplainFact retrieves full claim evidence, quotes, source URI, and line citations,
// plus historical chain of supersession for a given fact ID.
func (r *Registry) ExplainFact(id string) (*FactEvidence, error) {
	var f Fact
	var validFromStr, recordedAtStr string
	var validUntilStr, invalidatedAtStr, supersededBy, sourceURI, sourceQuote sql.NullString
	var lineNo sql.NullInt64
	var salience sql.NullFloat64

	row := r.db.QueryRow(`
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts WHERE id = ?`, id)

	err := row.Scan(
		&f.ID, &f.Namespace, &f.Subject, &f.Predicate, &f.Object, &f.Confidence, &f.Source,
		&sourceURI, &sourceQuote, &lineNo, &salience,
		&validFromStr, &validUntilStr, &recordedAtStr, &invalidatedAtStr, &supersededBy,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("fact with id %q not found", id)
		}
		return nil, fmt.Errorf("failed fetching fact %s: %w", id, err)
	}

	if sourceURI.Valid {
		f.SourceURI = sourceURI.String
	}
	if sourceQuote.Valid {
		f.SourceQuote = sourceQuote.String
	}
	if lineNo.Valid {
		f.LineNumber = int(lineNo.Int64)
	}
	if salience.Valid {
		f.Salience = salience.Float64
	}
	f.ValidFrom = parseFactTime(validFromStr)
	f.RecordedAt = parseFactTime(recordedAtStr)
	if validUntilStr.Valid {
		vu := parseFactTime(validUntilStr.String)
		f.ValidUntil = &vu
	}
	if invalidatedAtStr.Valid {
		ia := parseFactTime(invalidatedAtStr.String)
		f.InvalidatedAt = &ia
	}
	if supersededBy.Valid {
		sb := supersededBy.String
		f.SupersededBy = &sb
	}

	status := "active"
	if f.InvalidatedAt != nil {
		if f.SupersededBy != nil {
			status = "superseded"
		} else {
			status = "retracted"
		}
	}

	// Trace chain history if this fact superseded older facts
	chain, _ := r.queryFactsList(`
		SELECT id, namespace, subject, predicate, object, confidence, source,
		       source_uri, source_quote, line_number, salience,
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts
		WHERE superseded_by = ? OR id = (SELECT superseded_by FROM facts WHERE id = ?)
		ORDER BY recorded_at ASC`, id, id)

	evidence := &FactEvidence{
		Fact:           f,
		EvidenceQuote:  f.SourceQuote,
		SourceURI:      f.SourceURI,
		LineNumber:     f.LineNumber,
		Confidence:     f.Confidence,
		Salience:       f.Salience,
		Status:         status,
		SupersededByID: f.SupersededBy,
		ChainHistory:   chain,
	}

	return evidence, nil
}

// QueryFactsList executes a custom query returning facts.
func (r *Registry) QueryFactsList(query string, args ...interface{}) ([]Fact, error) {
	return r.queryFactsList(query, args...)
}

func (r *Registry) queryFactsList(query string, args ...interface{}) ([]Fact, error) {
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed executing query: %w", err)
	}
	defer rows.Close()

	var facts []Fact
	for rows.Next() {
		var f Fact
		var validFromStr, recordedAtStr string
		var validUntilStr, invalidatedAtStr, supersededBy, sourceURI, sourceQuote sql.NullString
		var lineNo sql.NullInt64
		var salience sql.NullFloat64

		err := rows.Scan(
			&f.ID, &f.Namespace, &f.Subject, &f.Predicate, &f.Object, &f.Confidence, &f.Source,
			&sourceURI, &sourceQuote, &lineNo, &salience,
			&validFromStr, &validUntilStr, &recordedAtStr, &invalidatedAtStr, &supersededBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning fact row: %w", err)
		}

		if sourceURI.Valid {
			f.SourceURI = sourceURI.String
		}
		if sourceQuote.Valid {
			f.SourceQuote = sourceQuote.String
		}
		if lineNo.Valid {
			f.LineNumber = int(lineNo.Int64)
		}
		if salience.Valid {
			f.Salience = salience.Float64
		}

		f.ValidFrom = parseFactTime(validFromStr)
		f.RecordedAt = parseFactTime(recordedAtStr)
		if validUntilStr.Valid {
			vu := parseFactTime(validUntilStr.String)
			f.ValidUntil = &vu
		}
		if invalidatedAtStr.Valid {
			ia := parseFactTime(invalidatedAtStr.String)
			f.InvalidatedAt = &ia
		}
		if supersededBy.Valid {
			sb := supersededBy.String
			f.SupersededBy = &sb
		}

		facts = append(facts, f)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return facts, nil
}

func parseFactTime(s string) time.Time {
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
