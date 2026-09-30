// Package frequency ranks English words by how common they are, from a
// static corpus-derived list, so the review deck can prioritize genuinely
// frequent vocabulary instead of relying on a model's own estimate.
//
// data/en.txt is the "Google 10000 English" list (the top ~10k words of
// Google's Trillion Word Corpus, one per line, most common first) —
// https://github.com/first20hours/google-10000-english. It's word-frequency
// data, not creative text, and is widely reused unmodified for exactly this
// purpose.
package frequency

import (
	"bufio"
	"bytes"
	_ "embed"
	"strings"
)

//go:embed data/en.txt
var enData []byte

// NotInList is the rank given to a word that doesn't appear in the top of
// the corpus — rarer than anything the list covers.
const NotInList = 100000

var enRank map[string]int

func init() {
	enRank = make(map[string]int, 10000)
	scanner := bufio.NewScanner(bytes.NewReader(enData))
	rank := 1
	for scanner.Scan() {
		word := strings.ToLower(strings.TrimSpace(scanner.Text()))
		if word == "" {
			continue
		}
		if _, exists := enRank[word]; !exists {
			enRank[word] = rank
		}
		rank++
	}
}

// Rank looks up a word's frequency rank (1 = most common) for the given
// book language. Only English is covered today; other languages and words
// missing from the list return (NotInList, false).
func Rank(lang, word string) (rank int, found bool) {
	lang = strings.ToLower(strings.TrimSpace(lang))
	// epub dc:language values vary: "en", "en-US", "en-GB", "eng"...
	if !strings.HasPrefix(lang, "en") {
		return NotInList, false
	}
	word = strings.ToLower(strings.TrimSpace(word))
	if word == "" {
		return NotInList, false
	}
	r, ok := enRank[word]
	if !ok {
		return NotInList, false
	}
	return r, true
}

// fallbackBands maps Claude's own 1-5 frequency estimate to a representative
// rank, for words the corpus doesn't cover (proper nouns, archaic or
// specialized vocabulary, non-English books) or entire unsupported
// languages. Keeps those words sorted sensibly relative to corpus-ranked
// ones instead of falling outside the scale.
var fallbackBands = map[int]int{1: 250, 2: 1000, 3: 3000, 4: 7000, 5: 15000}

// FallbackFromEstimate converts a model's 1-5 frequency guess into a rank on
// the same scale Rank returns, for use when Rank found nothing.
func FallbackFromEstimate(estimate int) int {
	if r, ok := fallbackBands[estimate]; ok {
		return r
	}
	return fallbackBands[3]
}

// Label gives a short human-readable bucket for a rank, for display.
func Label(rank int) string {
	switch {
	case rank <= 500:
		return "très courant"
	case rank <= 2000:
		return "courant"
	case rank <= 5000:
		return "peu courant"
	case rank < NotInList:
		return "rare"
	default:
		return "très rare"
	}
}
