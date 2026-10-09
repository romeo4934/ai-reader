package web

import "testing"

func TestHighlightPhrase(t *testing.T) {
	cases := []struct{ context, phrase, want string }{
		// whole word only, and just its sentence
		{"His mother’s letter. He read the letter twice. Then he left.", "the",
			"He read <mark>the</mark> letter twice."},
		// "Mr." doesn't end the sentence; closing quotes are kept
		{"“Never such a marriage while I am alive and Mr. Luzhin be damned!” he said. Fine.", "Luzhin",
			"“Never such a marriage while I am alive and Mr. <mark>Luzhin</mark> be damned!”"},
		// case-insensitive fallback, start of text
		{"The cat sat. It slept.", "the", "<mark>The</mark> cat sat."},
		// accents are part of words
		{"Él está aquí. Esta casa es grande.", "esta", "<mark>Esta</mark> casa es grande."},
		// no whole word: any occurrence
		{"Weirwood trees grow.", "wood", "Weir<mark>wood</mark> trees grow."},
		// not found: the whole context
		{"Nothing here.", "zzz", "Nothing here."},
	}
	for _, c := range cases {
		if got := string(highlightPhrase(c.context, c.phrase)); got != c.want {
			t.Errorf("highlightPhrase(%q, %q) = %q, want %q", c.context, c.phrase, got, c.want)
		}
	}
}
