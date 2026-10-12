// Package starter holds the game's starter words: for each language a
// player may learn, a few hundred common words picked from the frequency
// lists and translated into each interface language, so someone without a
// list of their own yet can play from the first minute.
package starter

import (
	"embed"
	"math/rand/v2"
	"slices"
	"strconv"
	"strings"
)

// Word is one starter word; Rank is its frequency rank.
type Word struct {
	Phrase, Translation string
	Rank                int
}

//go:embed data/*.tsv
var dataFS embed.FS

// lists by "lang-native", sorted by rank.
var lists = map[string][]Word{}

// order: the languages offered first to someone without a list.
var order = []string{"en", "es", "fr", "de", "it", "pt", "nl"}

func init() {
	files, _ := dataFS.ReadDir("data")
	for _, f := range files {
		raw, err := dataFS.ReadFile("data/" + f.Name())
		if err != nil {
			continue
		}
		var words []Word
		for _, line := range strings.Split(string(raw), "\n") {
			p := strings.Split(line, "\t")
			if len(p) != 3 {
				continue
			}
			rank, err := strconv.Atoi(p[2])
			if err != nil || p[0] == "" || p[1] == "" {
				continue
			}
			words = append(words, Word{Phrase: p[0], Translation: p[1], Rank: rank})
		}
		slices.SortFunc(words, func(a, b Word) int { return a.Rank - b.Rank })
		lists[strings.TrimSuffix(f.Name(), ".tsv")] = words
	}
}

func list(lang, native string) []Word { return lists[lang+"-"+native] }

// Has: there are starter words for learning lang with answers in native.
func Has(lang, native string) bool { return len(list(lang, native)) > 0 }

// Langs lists the languages with starter words for a native language.
func Langs(native string) []string {
	var out []string
	for _, l := range order {
		if Has(l, native) {
			out = append(out, l)
		}
	}
	return out
}

// Count counts the starter words ranked between lo and hi.
func Count(lang, native string, lo, hi int) int { return len(between(lang, native, lo, hi)) }

func between(lang, native string, lo, hi int) []Word {
	l := list(lang, native)
	i, _ := slices.BinarySearchFunc(l, lo, func(w Word, r int) int { return w.Rank - r })
	j, _ := slices.BinarySearchFunc(l, hi+1, func(w Word, r int) int { return w.Rank - r })
	return l[i:j]
}

// Pick returns up to n random starter words ranked between lo and hi.
func Pick(lang, native string, lo, hi, n int) []Word {
	l := slices.Clone(between(lang, native, lo, hi))
	rand.Shuffle(len(l), func(a, b int) { l[a], l[b] = l[b], l[a] })
	return l[:min(n, len(l))]
}

// Decoys returns n wrong answers for a word of the given rank: starter
// words of a similar frequency, their translation (or, reverse, the word
// itself), never one equal to the answer.
func Decoys(lang, native string, rank int, answer string, reverse bool, n int) []string {
	l := slices.Clone(list(lang, native))
	slices.SortStableFunc(l, func(a, b Word) int { return abs(a.Rank-rank) - abs(b.Rank-rank) })
	l = l[:min(len(l), n*5)]
	rand.Shuffle(len(l), func(a, b int) { l[a], l[b] = l[b], l[a] })
	var out []string
	seen := map[string]bool{strings.ToLower(strings.TrimSpace(answer)): true}
	for _, w := range l {
		d := w.Translation
		if reverse {
			d = w.Phrase
		}
		key := strings.ToLower(strings.TrimSpace(d))
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, d)
		if len(out) == n {
			break
		}
	}
	return out
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
