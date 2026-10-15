package conj

import (
	"strings"
	"testing"
)

func TestData(t *testing.T) {
	for _, lang := range Langs() {
		if len(Persons(lang)) == 0 || len(Tenses(lang)) < 3 {
			t.Errorf("%s : personnes ou temps manquants", lang)
		}
		vs := Common(lang)
		if len(vs) < 100 {
			t.Errorf("%s : %d verbes", lang, len(vs))
		}
		for _, v := range vs {
			for _, c := range columns {
				if c != lang && v.Meaning(c) == "" {
					t.Errorf("%s %s : pas de sens en %s", lang, v.Infinitive, c)
				}
			}
		}
	}
}

func TestFull(t *testing.T) {
	sub, _ := TenseOf("fr", Subjunctive)
	pres, _ := TenseOf("fr", Present)
	part, _ := TenseOf("en", Participle)
	for _, c := range []struct{ got, want string }{
		{Full("fr", pres, 0, "ai"), "j'ai"},
		{Full("fr", pres, 0, "mange"), "je mange"},
		{Full("fr", sub, 0, "aille"), "que j'aille"},
		{Full("fr", sub, 2, "aille"), "qu'il/elle aille"},
		{Full("en", part, 3, "ridden"), "ridden"},
	} {
		if c.got != c.want {
			t.Errorf("%q, voulu %q", c.got, c.want)
		}
	}
}

func TestAnswers(t *testing.T) {
	for _, c := range []struct{ lang, form, want string }{
		{"es", "hablo", "hablo"},
		{"it", "devo / debbo", "devo|debbo"},
		{"en", "have gotten / got", "have gotten|have got"},
		{"it", "sono andato/a", "sono andato|sono andata"},
		{"it", "siamo andati/e", "siamo andati|siamo andate"},
		{"it", "sono stato / stata", "sono stato|sono stata"},
		{"fr", "es allé(e)", "es allé|es allée"},
		{"fr", "sommes allé(e)s", "sommes allés|sommes allées"},
		{"fr", "suis allé/e", "suis allé|suis allée"},
		{"fr", "sommes allés/ées", "sommes allés|sommes allées"},
		{"fr", "sont venus/es", "sont venus|sont venues"},
		{"fr", "êtes descendu/e(s)", "êtes descendu|êtes descendue|êtes descendues"},
		{"fr", "êtes venu/e/s/es", "êtes venu|êtes venue|êtes venus|êtes venues"},
	} {
		if got := strings.Join(Answers(c.lang, c.form), "|"); got != c.want {
			t.Errorf("%q : %q, voulu %q", c.form, got, c.want)
		}
	}
}
