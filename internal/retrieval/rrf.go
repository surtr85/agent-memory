package retrieval

import (
	"sort"
)

// ScoredItem represents an item scored by Reciprocal Rank Fusion.
type ScoredItem struct {
	ID    string
	Score float64
}

// ReciprocalRankFusion combines multiple ranked lists into a single ranked list using RRF.
// Formula for each list m and rank r (1-indexed): score(d) += 1.0 / (k + rank)
// Default standard constant k is 60.
func ReciprocalRankFusion(rankedLists [][]string, k int) []ScoredItem {
	if k <= 0 {
		k = 60
	}

	scores := make(map[string]float64)

	for _, list := range rankedLists {
		for rank, id := range list {
			if id == "" {
				continue
			}
			// rank is 0-indexed, so 1-indexed rank is rank + 1
			scores[id] += 1.0 / float64(k+rank+1)
		}
	}

	items := make([]ScoredItem, 0, len(scores))
	for id, score := range scores {
		items = append(items, ScoredItem{
			ID:    id,
			Score: score,
		})
	}

	// Sort descending by score; ties broken by ID for determinism
	sort.Slice(items, func(i, j int) bool {
		if items[i].Score == items[j].Score {
			return items[i].ID < items[j].ID
		}
		return items[i].Score > items[j].Score
	})

	return items
}
