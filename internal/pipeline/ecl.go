package pipeline

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/surtr85/agent-memory/internal/embedding"
)

var (
	// Matches [[Target]] or [[Target|Alias]]
	wikilinkRegex = regexp.MustCompile(`\[\[([^\]\|]+)(?:\|[^\]]+)?\]\]`)
)

type markdownSection struct {
	Header  string
	Content string
}

// DreamCycleResult contains metrics and artifacts produced by the Nightly Dream consolidation.
type DreamCycleResult struct {
	DreamDate         string `json:"dream_date"`
	ObservationsMerged int    `json:"observations_merged"`
	FactsCreated      int    `json:"facts_created"`
	ProseContent      string `json:"prose_content"`
	SynthesisMarkdown string `json:"synthesis_markdown"`
}

// IngestMarkdown parses markdown by headers (#, ##, ###),
// chunks them, embeds each chunk, and populates `chunks` and `vector_embeddings`.
// It assigns appropriate memory bank category (world, experience, opinions, reflections, people, groups).
func IngestMarkdown(db *sql.DB, embClient embedding.Client, namespace, title, markdownContent string) error {
	return IngestMarkdownWithBank(db, embClient, namespace, title, markdownContent, "general", "", 0)
}

// IngestMarkdownWithBank ingests markdown with explicit bank, source URI, and line offset.
func IngestMarkdownWithBank(db *sql.DB, embClient embedding.Client, namespace, title, markdownContent, bank, sourceURI string, startLine int) error {
	if namespace == "" {
		namespace = "default"
	}
	if title == "" {
		title = "Untitled"
	}
	if bank == "" {
		bank = "general"
	}

	sections := splitMarkdownSections(title, markdownContent)
	if len(sections) == 0 {
		return nil
	}

	ctx := context.Background()

	for idx, sec := range sections {
		trimmedContent := strings.TrimSpace(sec.Content)
		if trimmedContent == "" {
			continue
		}

		// Generate a deterministic chunk ID based on namespace, title, and header
		hasher := sha256.New()
		hasher.Write([]byte(fmt.Sprintf("%s:%s:%s:%s", namespace, title, sec.Header, trimmedContent)))
		chunkID := hex.EncodeToString(hasher.Sum(nil))[:16]

		// Insert or replace chunk with bank and citation metadata
		chunkTitle := fmt.Sprintf("%s > %s", title, sec.Header)
		if sec.Header == title || sec.Header == "" {
			chunkTitle = title
		}

		lineNo := startLine + (idx * 10)

		_, err := db.Exec(`
			INSERT INTO chunks (id, namespace, title, content, source_uri, line_number, bank) 
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET 
				title = excluded.title,
				content = excluded.content,
				source_uri = excluded.source_uri,
				line_number = excluded.line_number,
				bank = excluded.bank
		`, chunkID, namespace, chunkTitle, trimmedContent, sourceURI, lineNo, bank)
		if err != nil {
			return fmt.Errorf("failed inserting chunk: %w", err)
		}

		// Generate embedding
		emb, err := embClient.GetEmbedding(ctx, trimmedContent)
		if err != nil {
			return fmt.Errorf("failed generating embedding: %w", err)
		}

		embBytes := embedding.SerializeEmbedding(emb)
		_, err = db.Exec(`
			INSERT INTO vector_embeddings (chunk_id, dimensions, embedding)
			VALUES (?, ?, ?)
			ON CONFLICT(chunk_id) DO UPDATE SET 
				dimensions = excluded.dimensions,
				embedding = excluded.embedding
		`, chunkID, len(emb), embBytes)
		if err != nil {
			return fmt.Errorf("failed inserting embedding: %w", err)
		}

		// Extract Wikilinks [[Entity]]
		links := extractWikilinks(trimmedContent)
		for _, link := range links {
			// Ensure entity exists
			_, err = db.Exec(`
				INSERT INTO entities (name, entity_type, namespace)
				VALUES (?, 'concept', ?)
				ON CONFLICT(name) DO NOTHING
			`, link, namespace)
			if err != nil {
				return fmt.Errorf("failed inserting entity %s: %w", link, err)
			}

			// Add relation between section header entity and link target
			sourceEnt := strings.TrimSpace(sec.Header)
			if sourceEnt == "" {
				sourceEnt = title
			}

			// Ensure source entity exists
			_, err = db.Exec(`
				INSERT INTO entities (name, entity_type, namespace)
				VALUES (?, 'section', ?)
				ON CONFLICT(name) DO NOTHING
			`, sourceEnt, namespace)
			if err != nil {
				return fmt.Errorf("failed inserting source entity %s: %w", sourceEnt, err)
			}

			relHasher := sha256.New()
			relHasher.Write([]byte(fmt.Sprintf("%s:%s:mentions", sourceEnt, link)))
			relID := hex.EncodeToString(relHasher.Sum(nil))[:16]

			_, err = db.Exec(`
				INSERT INTO entity_relations (id, source_entity, target_entity, relation_type, weight)
				VALUES (?, ?, ?, 'mentions', 1.0)
				ON CONFLICT(id) DO NOTHING
			`, relID, sourceEnt, link)
			if err != nil {
				return fmt.Errorf("failed inserting relation between %s and %s: %w", sourceEnt, link, err)
			}
		}
	}

	return nil
}

func splitMarkdownSections(defaultTitle, markdownContent string) []markdownSection {
	var sections []markdownSection
	scanner := bufio.NewScanner(strings.NewReader(markdownContent))

	currentHeader := defaultTitle
	var currentLines []string

	headerRegex := regexp.MustCompile(`^(#{1,6})\s+(.*)$`)

	for scanner.Scan() {
		line := scanner.Text()
		match := headerRegex.FindStringSubmatch(line)
		if len(match) > 0 {
			// Save previous section if non-empty
			if len(currentLines) > 0 {
				sections = append(sections, markdownSection{
					Header:  currentHeader,
					Content: strings.Join(currentLines, "\n"),
				})
				currentLines = nil
			}
			currentHeader = strings.TrimSpace(match[2])
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if len(currentLines) > 0 {
		sections = append(sections, markdownSection{
			Header:  currentHeader,
			Content: strings.Join(currentLines, "\n"),
		})
	}

	return sections
}

func extractWikilinks(content string) []string {
	matches := wikilinkRegex.FindAllStringSubmatch(content, -1)
	var links []string
	seen := make(map[string]bool)

	for _, m := range matches {
		if len(m) > 1 {
			target := strings.TrimSpace(m[1])
			if target != "" && !seen[target] {
				seen[target] = true
				links = append(links, target)
			}
		}
	}

	return links
}
