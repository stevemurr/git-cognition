package retrieval

import (
	"math"
	"strings"
	"unicode"
)

type ScoredExcerpt struct {
	Text  string
	Score float64
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
)

// RankExcerpts splits the document into sentences and scores each against
// the query terms using BM25. Returns excerpts sorted by score descending.
func RankExcerpts(query string, document string) []ScoredExcerpt {
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 || document == "" {
		return nil
	}

	sentences := splitSentences(document)
	if len(sentences) == 0 {
		return nil
	}

	// Compute document frequencies
	df := make(map[string]int)
	for _, s := range sentences {
		seen := make(map[string]bool)
		for _, t := range tokenize(s) {
			if !seen[t] {
				df[t]++
				seen[t] = true
			}
		}
	}

	// Average document length
	totalLen := 0
	for _, s := range sentences {
		totalLen += len(tokenize(s))
	}
	avgDL := float64(totalLen) / float64(len(sentences))

	n := float64(len(sentences))
	var results []ScoredExcerpt

	for _, s := range sentences {
		tokens := tokenize(s)
		dl := float64(len(tokens))

		// Term frequencies
		tf := make(map[string]int)
		for _, t := range tokens {
			tf[t]++
		}

		score := 0.0
		for _, qt := range queryTerms {
			if df[qt] == 0 {
				continue
			}
			idf := math.Log((n-float64(df[qt])+0.5)/(float64(df[qt])+0.5) + 1)
			tfNorm := (float64(tf[qt]) * (bm25K1 + 1)) /
				(float64(tf[qt]) + bm25K1*(1-bm25B+bm25B*dl/avgDL))
			score += idf * tfNorm
		}

		if score > 0 {
			results = append(results, ScoredExcerpt{Text: s, Score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// QueryFromFilePath generates query terms from a file path.
func QueryFromFilePath(filePath string) string {
	// Split on path separators and dots
	parts := strings.FieldsFunc(filePath, func(r rune) bool {
		return r == '/' || r == '\\' || r == '.'
	})
	return strings.Join(parts, " ")
}

func splitSentences(text string) []string {
	// Split on paragraph breaks first, then sentence boundaries
	paragraphs := strings.Split(text, "\n\n")
	var sentences []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// If it's a list item or short paragraph, keep as-is
		if strings.HasPrefix(p, "- ") || strings.HasPrefix(p, "* ") || len(p) < 100 {
			sentences = append(sentences, p)
			continue
		}
		// Split on sentence-ending punctuation
		var current strings.Builder
		for i, r := range p {
			current.WriteRune(r)
			if (r == '.' || r == '!' || r == '?') && i+1 < len(p) && p[i+1] == ' ' {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
			}
		}
		if s := strings.TrimSpace(current.String()); s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

func tokenize(text string) []string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	return words
}
