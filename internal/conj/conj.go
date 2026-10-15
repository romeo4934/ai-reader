// Package conj holds what the conjugation exercise needs besides the
// tables themselves (worked out by the AI, kept in the database): the
// tenses and persons drilled per language, how a question's pronoun reads,
// and the most common verbs of each language with a short meaning in every
// interface language, so someone can drill from the first minute.
package conj

import (
	"slices"
	"strings"
)

// Tense is one tense drilled in a language. ID is shared across languages
// (its gloss in the reader's language comes from the UI dictionary); Name
// is what the learned language itself calls it.
type Tense struct {
	ID, Name string
	// Impersonal: a single form, no person (the English past participle).
	Impersonal bool
}

const (
	Present     = "present"
	Preterite   = "preterite"
	Imperfect   = "imperfect"
	Perfect     = "perfect"
	Future      = "future"
	Conditional = "conditional"
	Subjunctive = "subjunctive"
	Participle  = "participle"
)

// tenses per language, the most useful first: the first three are drilled
// until the reader picks others.
var tenses = map[string][]Tense{
	"en": {{ID: Preterite, Name: "simple past"}, {ID: Participle, Name: "past participle", Impersonal: true}, {ID: Perfect, Name: "present perfect"}, {ID: Present, Name: "present simple"}},
	"fr": {{ID: Present, Name: "présent"}, {ID: Imperfect, Name: "imparfait"}, {ID: Perfect, Name: "passé composé"}, {ID: Future, Name: "futur simple"}, {ID: Conditional, Name: "conditionnel présent"}, {ID: Subjunctive, Name: "subjonctif présent"}, {ID: Preterite, Name: "passé simple"}},
	"es": {{ID: Present, Name: "presente"}, {ID: Preterite, Name: "pretérito indefinido"}, {ID: Imperfect, Name: "pretérito imperfecto"}, {ID: Perfect, Name: "pretérito perfecto"}, {ID: Future, Name: "futuro simple"}, {ID: Conditional, Name: "condicional"}, {ID: Subjunctive, Name: "presente de subjuntivo"}},
	"pt": {{ID: Present, Name: "presente"}, {ID: Preterite, Name: "pretérito perfeito"}, {ID: Imperfect, Name: "pretérito imperfeito"}, {ID: Future, Name: "futuro do presente"}, {ID: Conditional, Name: "futuro do pretérito"}, {ID: Subjunctive, Name: "presente do subjuntivo"}},
	"it": {{ID: Present, Name: "presente"}, {ID: Perfect, Name: "passato prossimo"}, {ID: Imperfect, Name: "imperfetto"}, {ID: Future, Name: "futuro semplice"}, {ID: Conditional, Name: "condizionale presente"}, {ID: Subjunctive, Name: "congiuntivo presente"}, {ID: Preterite, Name: "passato remoto"}},
	"de": {{ID: Present, Name: "Präsens"}, {ID: Preterite, Name: "Präteritum"}, {ID: Perfect, Name: "Perfekt"}, {ID: Future, Name: "Futur I"}, {ID: Conditional, Name: "Konjunktiv II"}},
	"nl": {{ID: Present, Name: "tegenwoordige tijd"}, {ID: Preterite, Name: "verleden tijd"}, {ID: Perfect, Name: "voltooid tegenwoordige tijd"}, {ID: Future, Name: "toekomende tijd"}},
}

// persons per language, as shown before the blank. Portuguese leaves out
// vós; "ele/ela" is also the você form.
var persons = map[string][]string{
	"en": {"I", "you", "he/she", "we", "they"},
	"fr": {"je", "tu", "il/elle", "nous", "vous", "ils/elles"},
	"es": {"yo", "tú", "él/ella", "nosotros", "vosotros", "ellos/ellas"},
	"pt": {"eu", "tu", "ele/ela", "nós", "eles/elas"},
	"it": {"io", "tu", "lui/lei", "noi", "voi", "loro"},
	"de": {"ich", "du", "er/sie", "wir", "ihr", "sie"},
	"nl": {"ik", "jij", "hij/zij", "wij", "jullie", "zij"},
}

// subjunctive: what comes before the pronoun in the subjunctive.
var subjunctive = map[string]string{"fr": "que", "es": "que", "pt": "que", "it": "che"}

// order: the languages offered.
var order = []string{"en", "es", "fr", "de", "it", "pt", "nl"}

// Langs lists the languages there are exercises for.
func Langs() []string { return order }

// Has: lang can be drilled.
func Has(lang string) bool { _, ok := tenses[lang]; return ok }

func Tenses(lang string) []Tense { return tenses[lang] }

// TenseOf finds one of lang's tenses by ID.
func TenseOf(lang, id string) (Tense, bool) {
	for _, t := range tenses[lang] {
		if t.ID == id {
			return t, true
		}
	}
	return Tense{}, false
}

// DefaultTenses: what's drilled until the reader picks.
func DefaultTenses(lang string) []string {
	var out []string
	for i, t := range tenses[lang] {
		if i < 3 {
			out = append(out, t.ID)
		}
	}
	return out
}

func Persons(lang string) []string { return persons[lang] }

// Count is how many forms a tense has in lang's tables.
func Count(lang string, t Tense) int {
	if t.Impersonal {
		return 1
	}
	return len(persons[lang])
}

// Pronoun is what's shown before the blank for a person ("je", "que tu",
// "qu'il/elle"); empty for an impersonal tense.
func Pronoun(lang string, t Tense, person int) string {
	if t.Impersonal || person < 0 || person >= len(persons[lang]) {
		return ""
	}
	p := persons[lang][person]
	if t.ID == Subjunctive {
		if q, ok := subjunctive[lang]; ok {
			if lang == "fr" && (strings.HasPrefix(p, "il")) {
				return "qu'" + p
			}
			return q + " " + p
		}
	}
	return p
}

// Full is the answer as read in full: pronoun and form, "je" elided before
// a vowel ("j'ai mangé", "que j'aille").
func Full(lang string, t Tense, person int, form string) string {
	p := Pronoun(lang, t, person)
	if p == "" {
		return form
	}
	if lang == "fr" && strings.HasSuffix(p, "je") && startsWithVowel(form) {
		return strings.TrimSuffix(p, "e") + "'" + form
	}
	return p + " " + form
}

func startsWithVowel(s string) bool {
	for _, r := range strings.ToLower(s) {
		return strings.ContainsRune("aeiouyéèêàâîïôûh", r)
	}
	return false
}

// Answers lists the forms accepted for a table entry: "devo / debbo" is
// either one, "have gotten / got" is "have gotten" or "have got", and a
// gender ending given with a slash or in brackets ("sono andato/a",
// "es allé(e)") is either gender.
func Answers(form string) []string {
	var out []string
	add := func(a string) {
		if a = strings.Join(strings.Fields(a), " "); a != "" && !slices.Contains(out, a) {
			out = append(out, a)
		}
	}
	parts := strings.Split(form, " / ")
	first := strings.Fields(parts[0])
	for i, p := range parts {
		words := strings.Fields(p)
		if i > 0 && len(words) < len(first) {
			words = append(slices.Clone(first[:len(first)-len(words)]), words...)
		}
		masc, fem := make([]string, len(words)), make([]string, len(words))
		for j, w := range words {
			masc[j], fem[j] = w, w
			if base, end, ok := strings.Cut(w, "/"); ok && len(end) <= 2 && len(base) > len(end) {
				masc[j] = base
				fem[j] = base[:len(base)-len(end)] + end
			} else if before, rest, ok := strings.Cut(w, "("); ok {
				if inside, after, ok := strings.Cut(rest, ")"); ok { // "allé(e)s"
					masc[j] = before + after
					fem[j] = before + inside + after
				}
			}
		}
		add(strings.Join(masc, " "))
		add(strings.Join(fem, " "))
	}
	return out
}
