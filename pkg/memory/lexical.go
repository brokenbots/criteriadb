package memory

import (
	"strings"
	"unicode"

	pb "github.com/brokenbots/criteriadb/pkg/pb/criteriadb/v1"
)

// LexicalIndex provides pure-Go in-memory keyword searching without external dependencies.
type LexicalIndex struct{}

func NewLexicalIndex() *LexicalIndex {
	return &LexicalIndex{}
}

// Tokenize converts text into lowercased token set.
func Tokenize(text string) map[string]struct{} {
	tokens := make(map[string]struct{})
	f := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	words := strings.FieldsFunc(text, f)
	for _, w := range words {
		wLower := strings.ToLower(w)
		if len(wLower) > 1 {
			tokens[wLower] = struct{}{}
		}
	}
	return tokens
}

// EvaluateLexical calculates token overlap score (Jaccard similarity) between queryText and MemoryNode label/summary.
func (idx *LexicalIndex) EvaluateLexical(node *pb.MemoryNode, queryText string) float64 {
	if strings.TrimSpace(queryText) == "" {
		return 1.0
	}

	qTokens := Tokenize(queryText)
	if len(qTokens) == 0 {
		return 1.0
	}

	nodeText := node.GetLabel() + " " + node.GetSummary()
	nTokens := Tokenize(nodeText)

	if len(nTokens) == 0 {
		return 0.0
	}

	intersection := 0
	for t := range qTokens {
		if _, ok := nTokens[t]; ok {
			intersection++
		}
	}

	if intersection == 0 {
		return 0.0
	}

	// Jaccard similarity coefficient
	union := len(qTokens) + len(nTokens) - intersection
	return float64(intersection) / float64(union)
}
