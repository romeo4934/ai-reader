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
