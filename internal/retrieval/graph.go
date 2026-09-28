package retrieval

import (
	"database/sql"
	"fmt"
)

// TraverseGraph performs BFS entity graph traversal starting from startEntities up to maxHops.
// Returns all discovered entity names, avoiding cycles.
func TraverseGraph(db *sql.DB, startEntities []string, maxHops int) ([]string, error) {
	if len(startEntities) == 0 {
		return []string{}, nil
	}
	if maxHops <= 0 {
		maxHops = 1
	}

	visited := make(map[string]bool)
	var queue []string

	for _, e := range startEntities {
		if e != "" && !visited[e] {
			visited[e] = true
			queue = append(queue, e)
		}
	}

	for hop := 0; hop < maxHops; hop++ {
		levelSize := len(queue)
		if levelSize == 0 {
			break
		}

		currentLevel := make([]string, levelSize)
		copy(currentLevel, queue)
		queue = queue[:0]

		for _, entity := range currentLevel {
			// Find neighbors in both directions (source or target)
			rows, err := db.Query(`
				SELECT target_entity FROM entity_relations WHERE source_entity = ?
				UNION
				SELECT source_entity FROM entity_relations WHERE target_entity = ?
			`, entity, entity)
			if err != nil {
				return nil, fmt.Errorf("failed querying relations for %s: %w", entity, err)
			}

			var neighbors []string
			for rows.Next() {
				var neighbor string
				if err := rows.Scan(&neighbor); err != nil {
					rows.Close()
					return nil, err
				}
				neighbors = append(neighbors, neighbor)
			}
			rows.Close()

			for _, n := range neighbors {
				if !visited[n] {
					visited[n] = true
					queue = append(queue, n)
				}
			}
		}
	}

	result := make([]string, 0, len(visited))
	for e := range visited {
		result = append(result, e)
	}

	return result, nil
}
