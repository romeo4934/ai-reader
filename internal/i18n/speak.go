package i18n

// The 🔊 button that pronounces a word, and the reader's text size buttons.
var speakStrings = map[string]Dict{
	"en": {"ReaderSmaller": "Smaller text", "ReaderBigger": "Larger text", "SayWord": "Listen to the pronunciation"},
	"fr": {"ReaderSmaller": "Texte plus petit", "ReaderBigger": "Texte plus grand", "SayWord": "Écouter la prononciation"},
	"es": {"ReaderSmaller": "Texto más pequeño", "ReaderBigger": "Texto más grande", "SayWord": "Escuchar la pronunciación"},
	"pt": {"ReaderSmaller": "Texto menor", "ReaderBigger": "Texto maior", "SayWord": "Ouvir a pronúncia"},
	"it": {"ReaderSmaller": "Testo più piccolo", "ReaderBigger": "Testo più grande", "SayWord": "Ascolta la pronuncia"},
	"de": {"ReaderSmaller": "Kleinere Schrift", "ReaderBigger": "Größere Schrift", "SayWord": "Aussprache anhören"},
	"nl": {"ReaderSmaller": "Kleinere tekst", "ReaderBigger": "Grotere tekst", "SayWord": "Uitspraak beluisteren"},
}

func init() {
	for code, d := range speakStrings {
		for k, v := range d {
			byCode[code][k] = v
		}
	}
}
