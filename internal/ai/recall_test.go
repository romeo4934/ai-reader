package ai

import "testing"

func TestTrimStray(t *testing.T) {
	cases := map[string]string{
		"Dejé las llaves sobre la mesa antes de salir.A": "Dejé las llaves sobre la mesa antes de salir.",
		"¿Dónde está?Ab ":                                "¿Dónde está?",
		"Il est parti.":                                  "Il est parti.",
		"Il a dit « oui ».":                              "Il a dit « oui ».",
		"Une phrase sans point":                          "Une phrase sans point",
	}
	for in, want := range cases {
		if got := trimStray(in); got != want {
			t.Errorf("trimStray(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLeaksSentence(t *testing.T) {
	bad := RecallCard{
		SentenceBlank:       "Se quedó _____ al ver que la tienda estaba cerrada un lunes, algo que nunca había pasado.",
		SentenceTranslation: "Se quedó extrañado al ver que la tienda estaba cerrada un lunes, algo que nunca había pasado.— Se quedó sorprendido al ver que la tienda estaba cerrada un lunes (en français: Il fut étonné de voir que le magasin était fermé un lundi).",
	}
	if !leaksSentence(bad) {
		t.Error("the Spanish sentence in the translation should be caught")
	}
	good := bad
	good.SentenceTranslation = "Il fut étonné de voir que le magasin était fermé un lundi, chose qui n'était jamais arrivée."
	if leaksSentence(good) {
		t.Error("a real translation is fine")
	}
	short := RecallCard{SentenceBlank: "Él _____ la puerta.", SentenceTranslation: "Il ferma la porte."}
	if leaksSentence(short) {
		t.Error("short parts aren't telling")
	}
}
