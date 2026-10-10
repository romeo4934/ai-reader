// Package frequency ranks words by how common they are, from static
// corpus-derived lists, so the review deck and the word game can rely on
// genuinely frequent vocabulary instead of a model's own estimate.
//
// data/<lang>.txt are the 50k lists of FrequencyWords by Hermit Dave
// (https://github.com/hermitdave/FrequencyWords, 2018 OpenSubtitles corpus,
// content under CC BY-SA 4.0, credited on /credits): one word per line,
// most common first, counts stripped.
package frequency

import (
	"bufio"
	"bytes"
	"embed"
	"strings"
)

//go:embed data/*.txt
var dataFS embed.FS

// NotInList is the rank given to a word that doesn't appear in the top of
// the corpus — rarer than anything the list covers.
const NotInList = 100000

// ranks maps a two-letter language code to its word ranks.
var ranks = map[string]map[string]int{}

func init() {
	entries, err := dataFS.ReadDir("data")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		raw, err := dataFS.ReadFile("data/" + e.Name())
		if err != nil {
			panic(err)
		}
		m := make(map[string]int, 50000)
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		rank := 1
		for scanner.Scan() {
			word := strings.ToLower(strings.TrimSpace(scanner.Text()))
			if word == "" {
				continue
			}
			if _, exists := m[word]; !exists {
				m[word] = rank
			}
			rank++
		}
		ranks[strings.TrimSuffix(e.Name(), ".txt")] = m
	}
}

// listFor resolves an epub dc:language value ("en", "en-US", "eng", "fra"…)
// to its list, if Lydi has one.
func listFor(lang string) map[string]int {
	lang = strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(lang, "-_"); i >= 0 {
		lang = lang[:i]
	}
	if m, ok := ranks[lang]; ok {
		return m
	}
	if two, ok := iso639_2[lang]; ok {
		return ranks[two]
	}
	return nil
}

var iso639_2 = map[string]string{
	"eng": "en", "spa": "es", "fra": "fr", "fre": "fr", "por": "pt",
	"ita": "it", "deu": "de", "ger": "de", "nld": "nl", "dut": "nl",
}

// Covered: the language has a frequency list, so a single word missing from
// it is genuinely rare.
func Covered(lang string) bool { return listFor(lang) != nil }

// Rank looks up a word's frequency rank (1 = most common) for the given
// book language. Languages without a list, and words missing from it,
// return (NotInList, false).
func Rank(lang, word string) (rank int, found bool) {
	m := listFor(lang)
	word = strings.ToLower(strings.TrimSpace(word))
	if m == nil || word == "" {
		return NotInList, false
	}
	r, ok := m[word]
	if !ok {
		return NotInList, false
	}
	return r, true
}

// Of ranks a looked-up word: its lemma, else the form found in the book, on
// the corpus list; failing that, a single word in a covered language is
// rarer than the whole list, and anything else (phrases, uncovered
// languages) falls back on the model's 1-5 estimate.
func Of(lang, lemma, phrase string, estimate int) int {
	for _, w := range []string{lemma, phrase} {
		if r, ok := Rank(lang, w); ok {
			return r
		}
	}
	if Covered(lang) && len(strings.Fields(phrase)) == 1 {
		return NotInList
	}
	return FallbackFromEstimate(estimate)
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
