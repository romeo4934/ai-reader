package i18n

// The 🔊 button that pronounces a word.
var speakStrings = map[string]Dict{
	"en": {"SayWord": "Listen to the pronunciation"},
	"fr": {"SayWord": "Écouter la prononciation"},
	"es": {"SayWord": "Escuchar la pronunciación"},
	"pt": {"SayWord": "Ouvir a pronúncia"},
	"it": {"SayWord": "Ascolta la pronuncia"},
	"de": {"SayWord": "Aussprache anhören"},
	"nl": {"SayWord": "Uitspraak beluisteren"},
}

func init() {
	for code, d := range speakStrings {
		for k, v := range d {
			byCode[code][k] = v
		}
	}
}
