package temporal

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Fact represents a bi-temporal atomic assertion.
type Fact struct {
	ID            string     `json:"id"`
	Namespace     string     `json:"namespace"`
	Subject       string     `json:"subject"`
	Predicate     string     `json:"predicate"`
	Object        string     `json:"object"`
	Confidence    float64    `json:"confidence"`
	Source        string     `json:"source"`
	ValidFrom     time.Time  `json:"valid_from"`
	ValidUntil    *time.Time `json:"valid_until,omitempty"`
	RecordedAt    time.Time  `json:"recorded_at"`
	InvalidatedAt *time.Time `json:"invalidated_at,omitempty"`
	SupersededBy  *string    `json:"superseded_by,omitempty"`
}

// Registry handles bi-temporal fact storage, contradiction resolution, and queries.
type Registry struct {
	db *sql.DB
}

// NewRegistry creates a new temporal fact Registry.
func NewRegistry(db *sql.DB) *Registry {
	return &Registry{db: db}
}

// AddFact inserts a new fact into the bi-temporal registry.
// Contradiction resolution:
// If an active fact (`valid_until IS NULL`) exists with identical `namespace`, `subject`, and `predicate`:
// - If the object is identical, we can either return the existing fact or ignore.
// - If the object differs, mark the old fact's `valid_until = now()`, `invalidated_at = now()`,
//   `superseded_by = newFact.ID`.
func (r *Registry) AddFact(namespace, subject, predicate, object, source string) (*Fact, error) {
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	newID := uuid.New().String()

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

	// Insert the new fact first so foreign key constraints on superseded_by are satisfied
	_, err = tx.Exec(
		`INSERT INTO facts (
			id, namespace, subject, predicate, object, 
			confidence, source, valid_from, valid_until, 
			recorded_at, invalidated_at, superseded_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, NULL, NULL)`,
		newID, namespace, subject, predicate, object,
		1.0, source, nowStr, nowStr,
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
		ID:         newID,
		Namespace:  namespace,
		Subject:    subject,
		Predicate:  predicate,
		Object:     object,
		Confidence: 1.0,
		Source:     source,
		ValidFrom:  now,
		ValidUntil: nil,
		RecordedAt: now,
	}, nil
}

// GetActiveFacts returns all active facts (where valid_until IS NULL) in a given namespace.
func (r *Registry) GetActiveFacts(namespace string) ([]Fact, error) {
	query := `
		SELECT id, namespace, subject, predicate, object, confidence, source,
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
		       valid_from, valid_until, recorded_at, invalidated_at, superseded_by
		FROM facts WHERE id = ?`, id)

	var f Fact
	var validFromStr, recordedAtStr string
	var validUntilStr, invalidatedAtStr, supersededBy sql.NullString

	err := row.Scan(
		&f.ID, &f.Namespace, &f.Subject, &f.Predicate, &f.Object, &f.Confidence, &f.Source,
		&validFromStr, &validUntilStr, &recordedAtStr, &invalidatedAtStr, &supersededBy,
	)
	if err != nil {
		return nil, fmt.Errorf("failed scanning fact %s: %w", id, err)
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
		var validUntilStr, invalidatedAtStr, supersededBy sql.NullString

		err := rows.Scan(
			&f.ID, &f.Namespace, &f.Subject, &f.Predicate, &f.Object, &f.Confidence, &f.Source,
			&validFromStr, &validUntilStr, &recordedAtStr, &invalidatedAtStr, &supersededBy,
		)
		if err != nil {
			return nil, fmt.Errorf("failed scanning fact row: %w", err)
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
