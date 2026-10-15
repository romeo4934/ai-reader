package conj

import (
	"embed"
	"strings"
)

// Verb is one of a language's common verbs; Rank 1 is the most common.
type Verb struct {
	Infinitive string
	Rank       int
	meanings   map[string]string
}

// Meaning is the verb's meaning in an interface language.
func (v Verb) Meaning(native string) string { return v.meanings[native] }

//go:embed data/*.tsv
var dataFS embed.FS

// columns of the data files after the infinitive.
var columns = []string{"en", "fr", "es", "pt", "it", "de", "nl"}

var verbs = map[string][]Verb{}

func init() {
	for _, lang := range order {
		raw, err := dataFS.ReadFile("data/" + lang + ".tsv")
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(raw), "\n") {
			p := strings.Split(line, "\t")
			if len(p) != 1+len(columns) || p[0] == "" {
				continue
			}
			v := Verb{Infinitive: p[0], Rank: len(verbs[lang]) + 1, meanings: map[string]string{}}
			for i, c := range columns {
				v.meanings[c] = p[1+i]
			}
			verbs[lang] = append(verbs[lang], v)
		}
	}
}

// Common lists lang's common verbs, most common first.
func Common(lang string) []Verb { return verbs[lang] }

// CommonVerb finds one of lang's common verbs.
func CommonVerb(lang, infinitive string) (Verb, bool) {
	for _, v := range verbs[lang] {
		if v.Infinitive == infinitive {
			return v, true
		}
	}
	return Verb{}, false
}
