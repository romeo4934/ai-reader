// Package i18n translates the app's own interface (nav, buttons, empty
// states) — separate from internal/ai, which translates book content.
// Only the authenticated area is covered: at login/signup there's no user
// yet to read a language preference from, so those two pages stay French.
package i18n

import "strings"

type Dict map[string]string

// For resolves a user's free-text native_lang setting (one of the dropdown
// values on /settings, or anything typed into "Autre") to the closest
// supported UI language, defaulting to French — the original language of
// this app and its first user.
func For(nativeLang string) Dict {
	lang := strings.ToLower(strings.TrimSpace(nativeLang))
	switch {
	case strings.Contains(lang, "english"), strings.Contains(lang, "anglais"):
		return en
	case strings.Contains(lang, "español"), strings.Contains(lang, "espanol"), strings.Contains(lang, "spanish"), strings.Contains(lang, "espagnol"):
		return es
	case strings.Contains(lang, "português"), strings.Contains(lang, "portugues"), strings.Contains(lang, "portuguese"), strings.Contains(lang, "portugais"):
		return pt
	case strings.Contains(lang, "italiano"), strings.Contains(lang, "italian"), strings.Contains(lang, "italien"):
		return it
	case strings.Contains(lang, "deutsch"), strings.Contains(lang, "german"), strings.Contains(lang, "allemand"):
		return de
	case strings.Contains(lang, "nederlands"), strings.Contains(lang, "dutch"), strings.Contains(lang, "néerlandais"), strings.Contains(lang, "neerlandais"):
		return nl
	default:
		return fr
	}
}

var fr = Dict{
	"LangCode":   "fr",
	"NavMyWords": "mes mots", "NavReview": "révision", "NavHistory": "historique",
	"NavSettings": "réglages", "NavLogin": "connexion",

	"LibTitle": "Bibliothèque", "LibUpload": "Ajouter un livre",
	"LibEmpty":    "Aucun livre pour l'instant — ajoute un fichier .epub ci-dessus.",
	"LibChapters": "chapitres",

	"ReaderBack": "← bibliothèque", "ReaderPrev": "← précédent",
	"ReaderNext": "suivant →", "ReaderTranslating": "traduction…", "ReaderClose": "Fermer",
	"ReaderPrevChapter": "chapitre précédent", "ReaderNextChapter": "chapitre suivant",

	"ReviewTitle":      "Révision",
	"ReviewEmpty":      "Rien à réviser pour l'instant — reviens plus tard, ou lis un peu plus 📖",
	"ReviewRevealWord": "Voir le mot", "ReviewRevealTranslation": "Voir la traduction",
	"ReviewOriginalLabel": "phrase d'origine :",
	"ReviewAgain":         "Je ne savais pas", "ReviewGood": "Je savais",

	"WordsTitle":         "Mes mots",
	"WordsEmpty":         "Aucun mot enregistré pour l'instant — clique sur un mot en lisant pour commencer.",
	"WordsKnowIt":        "✓ je le connais, retire-le",
	"WordsDelete":        "🗑 supprimer",
	"WordsDeleteConfirm": "Supprimer ce mot pour de bon ? Il ne reviendra plus en révision.",

	"ReviewedTitle": "Dernières révisions",
	"ReviewedEmpty": "Aucune révision pour l'instant — ça se remplit dès que tu réponds à une carte sur /review.",
	"ReviewedBox":   "boîte",

	"SettingsTitle": "Réglages", "SettingsNativeLangLabel": "Ma langue maternelle",
	"SettingsNativeLangHint": "Utilisée pour toutes les traductions, quel que soit le livre.",
	"SettingsSave":           "Enregistrer", "SettingsLoggedInAs": "Connecté en tant que",
	"SettingsLogout": "se déconnecter", "SettingsOther": "Autre…",
}

var en = Dict{
	"LangCode":   "en",
	"NavMyWords": "my words", "NavReview": "review", "NavHistory": "history",
	"NavSettings": "settings", "NavLogin": "log in",

	"LibTitle": "Library", "LibUpload": "Add a book",
	"LibEmpty":    "No books yet — add an .epub file above.",
	"LibChapters": "chapters",

	"ReaderBack": "← library", "ReaderPrev": "← previous",
	"ReaderNext": "next →", "ReaderTranslating": "translating…", "ReaderClose": "Close",
	"ReaderPrevChapter": "previous chapter", "ReaderNextChapter": "next chapter",

	"ReviewTitle":      "Review",
	"ReviewEmpty":      "Nothing to review right now — come back later, or keep reading 📖",
	"ReviewRevealWord": "Show the word", "ReviewRevealTranslation": "Show the translation",
	"ReviewOriginalLabel": "original sentence:",
	"ReviewAgain":         "I didn't know it", "ReviewGood": "I knew it",

	"WordsTitle":         "My words",
	"WordsEmpty":         "No words saved yet — tap a word while reading to get started.",
	"WordsKnowIt":        "✓ I know this, remove it",
	"WordsDelete":        "🗑 delete",
	"WordsDeleteConfirm": "Delete this word for good? It won't come back in review.",

	"ReviewedTitle": "Recent reviews",
	"ReviewedEmpty": "No reviews yet — this fills up once you answer a card on /review.",
	"ReviewedBox":   "box",

	"SettingsTitle": "Settings", "SettingsNativeLangLabel": "My native language",
	"SettingsNativeLangHint": "Used for every translation, regardless of the book.",
	"SettingsSave":           "Save", "SettingsLoggedInAs": "Logged in as",
	"SettingsLogout": "log out", "SettingsOther": "Other…",
}

var es = Dict{
	"LangCode":   "es",
	"NavMyWords": "mis palabras", "NavReview": "repaso", "NavHistory": "historial",
	"NavSettings": "ajustes", "NavLogin": "iniciar sesión",

	"LibTitle": "Biblioteca", "LibUpload": "Añadir un libro",
	"LibEmpty":    "Aún no hay libros — añade un archivo .epub arriba.",
	"LibChapters": "capítulos",

	"ReaderBack": "← biblioteca", "ReaderPrev": "← anterior",
	"ReaderNext": "siguiente →", "ReaderTranslating": "traduciendo…", "ReaderClose": "Cerrar",
	"ReaderPrevChapter": "capítulo anterior", "ReaderNextChapter": "capítulo siguiente",

	"ReviewTitle":      "Repaso",
	"ReviewEmpty":      "Nada que repasar por ahora — vuelve más tarde, o sigue leyendo 📖",
	"ReviewRevealWord": "Ver la palabra", "ReviewRevealTranslation": "Ver la traducción",
	"ReviewOriginalLabel": "frase original:",
	"ReviewAgain":         "No lo sabía", "ReviewGood": "Lo sabía",

	"WordsTitle":         "Mis palabras",
	"WordsEmpty":         "Aún no hay palabras guardadas — toca una palabra mientras lees para empezar.",
	"WordsKnowIt":        "✓ ya la sé, quítala",
	"WordsDelete":        "🗑 eliminar",
	"WordsDeleteConfirm": "¿Eliminar esta palabra definitivamente? No volverá a aparecer en el repaso.",

	"ReviewedTitle": "Últimos repasos",
	"ReviewedEmpty": "Aún no hay repasos — esto se llena en cuanto respondas una carta en /review.",
	"ReviewedBox":   "caja",

	"SettingsTitle": "Ajustes", "SettingsNativeLangLabel": "Mi lengua materna",
	"SettingsNativeLangHint": "Se usa para todas las traducciones, sea cual sea el libro.",
	"SettingsSave":           "Guardar", "SettingsLoggedInAs": "Conectado como",
	"SettingsLogout": "cerrar sesión", "SettingsOther": "Otra…",
}

var pt = Dict{
	"LangCode":   "pt",
	"NavMyWords": "minhas palavras", "NavReview": "revisão", "NavHistory": "histórico",
	"NavSettings": "ajustes", "NavLogin": "entrar",

	"LibTitle": "Biblioteca", "LibUpload": "Adicionar um livro",
	"LibEmpty":    "Nenhum livro ainda — adicione um arquivo .epub acima.",
	"LibChapters": "capítulos",

	"ReaderBack": "← biblioteca", "ReaderPrev": "← anterior",
	"ReaderNext": "próximo →", "ReaderTranslating": "traduzindo…", "ReaderClose": "Fechar",
	"ReaderPrevChapter": "capítulo anterior", "ReaderNextChapter": "próximo capítulo",

	"ReviewTitle":      "Revisão",
	"ReviewEmpty":      "Nada para revisar agora — volte mais tarde, ou continue lendo 📖",
	"ReviewRevealWord": "Ver a palavra", "ReviewRevealTranslation": "Ver a tradução",
	"ReviewOriginalLabel": "frase original:",
	"ReviewAgain":         "Eu não sabia", "ReviewGood": "Eu sabia",

	"WordsTitle":         "Minhas palavras",
	"WordsEmpty":         "Nenhuma palavra salva ainda — toque em uma palavra durante a leitura para começar.",
	"WordsKnowIt":        "✓ eu já sei, remover",
	"WordsDelete":        "🗑 excluir",
	"WordsDeleteConfirm": "Excluir esta palavra de vez? Ela não voltará na revisão.",

	"ReviewedTitle": "Últimas revisões",
	"ReviewedEmpty": "Nenhuma revisão ainda — isso se preenche assim que você responder um cartão em /review.",
	"ReviewedBox":   "caixa",

	"SettingsTitle": "Ajustes", "SettingsNativeLangLabel": "Minha língua materna",
	"SettingsNativeLangHint": "Usada em todas as traduções, não importa o livro.",
	"SettingsSave":           "Salvar", "SettingsLoggedInAs": "Conectado como",
	"SettingsLogout": "sair", "SettingsOther": "Outra…",
}

var it = Dict{
	"LangCode":   "it",
	"NavMyWords": "le mie parole", "NavReview": "ripasso", "NavHistory": "cronologia",
	"NavSettings": "impostazioni", "NavLogin": "accedi",

	"LibTitle": "Libreria", "LibUpload": "Aggiungi un libro",
	"LibEmpty":    "Ancora nessun libro — aggiungi un file .epub qui sopra.",
	"LibChapters": "capitoli",

	"ReaderBack": "← libreria", "ReaderPrev": "← precedente",
	"ReaderNext": "successivo →", "ReaderTranslating": "traduzione…", "ReaderClose": "Chiudi",
	"ReaderPrevChapter": "capitolo precedente", "ReaderNextChapter": "capitolo successivo",

	"ReviewTitle":      "Ripasso",
	"ReviewEmpty":      "Niente da ripassare per ora — torna più tardi, o continua a leggere 📖",
	"ReviewRevealWord": "Mostra la parola", "ReviewRevealTranslation": "Mostra la traduzione",
	"ReviewOriginalLabel": "frase originale:",
	"ReviewAgain":         "Non la sapevo", "ReviewGood": "La sapevo",

	"WordsTitle":         "Le mie parole",
	"WordsEmpty":         "Ancora nessuna parola salvata — tocca una parola mentre leggi per iniziare.",
	"WordsKnowIt":        "✓ la conosco, rimuovila",
	"WordsDelete":        "🗑 elimina",
	"WordsDeleteConfirm": "Eliminare questa parola definitivamente? Non tornerà più nel ripasso.",

	"ReviewedTitle": "Ultimi ripassi",
	"ReviewedEmpty": "Ancora nessun ripasso — si riempie non appena rispondi a una carta su /review.",
	"ReviewedBox":   "scatola",

	"SettingsTitle": "Impostazioni", "SettingsNativeLangLabel": "La mia lingua madre",
	"SettingsNativeLangHint": "Usata per tutte le traduzioni, qualunque sia il libro.",
	"SettingsSave":           "Salva", "SettingsLoggedInAs": "Connesso come",
	"SettingsLogout": "esci", "SettingsOther": "Altra…",
}

var de = Dict{
	"LangCode":   "de",
	"NavMyWords": "meine Wörter", "NavReview": "Wiederholung", "NavHistory": "Verlauf",
	"NavSettings": "Einstellungen", "NavLogin": "Anmelden",

	"LibTitle": "Bibliothek", "LibUpload": "Buch hinzufügen",
	"LibEmpty":    "Noch keine Bücher — füge oben eine .epub-Datei hinzu.",
	"LibChapters": "Kapitel",

	"ReaderBack": "← Bibliothek", "ReaderPrev": "← zurück",
	"ReaderNext": "weiter →", "ReaderTranslating": "Übersetzung läuft…", "ReaderClose": "Schließen",
	"ReaderPrevChapter": "vorheriges Kapitel", "ReaderNextChapter": "nächstes Kapitel",

	"ReviewTitle":      "Wiederholung",
	"ReviewEmpty":      "Gerade nichts zu wiederholen — komm später wieder oder lies weiter 📖",
	"ReviewRevealWord": "Wort anzeigen", "ReviewRevealTranslation": "Übersetzung anzeigen",
	"ReviewOriginalLabel": "Originalsatz:",
	"ReviewAgain":         "Wusste ich nicht", "ReviewGood": "Wusste ich",

	"WordsTitle":         "Meine Wörter",
	"WordsEmpty":         "Noch keine Wörter gespeichert — tippe beim Lesen auf ein Wort, um loszulegen.",
	"WordsKnowIt":        "✓ kenne ich, entfernen",
	"WordsDelete":        "🗑 löschen",
	"WordsDeleteConfirm": "Dieses Wort endgültig löschen? Es kommt nicht mehr in der Wiederholung vor.",

	"ReviewedTitle": "Letzte Wiederholungen",
	"ReviewedEmpty": "Noch keine Wiederholungen — füllt sich, sobald du eine Karte auf /review beantwortest.",
	"ReviewedBox":   "Box",

	"SettingsTitle": "Einstellungen", "SettingsNativeLangLabel": "Meine Muttersprache",
	"SettingsNativeLangHint": "Wird für alle Übersetzungen verwendet, unabhängig vom Buch.",
	"SettingsSave":           "Speichern", "SettingsLoggedInAs": "Angemeldet als",
	"SettingsLogout": "Abmelden", "SettingsOther": "Andere…",
}

var nl = Dict{
	"LangCode":   "nl",
	"NavMyWords": "mijn woorden", "NavReview": "herhaling", "NavHistory": "geschiedenis",
	"NavSettings": "instellingen", "NavLogin": "inloggen",

	"LibTitle": "Bibliotheek", "LibUpload": "Boek toevoegen",
	"LibEmpty":    "Nog geen boeken — voeg hierboven een .epub-bestand toe.",
	"LibChapters": "hoofdstukken",

	"ReaderBack": "← bibliotheek", "ReaderPrev": "← vorige",
	"ReaderNext": "volgende →", "ReaderTranslating": "vertalen…", "ReaderClose": "Sluiten",
	"ReaderPrevChapter": "vorig hoofdstuk", "ReaderNextChapter": "volgend hoofdstuk",

	"ReviewTitle":      "Herhaling",
	"ReviewEmpty":      "Niets te herhalen op dit moment — kom later terug, of lees verder 📖",
	"ReviewRevealWord": "Toon het woord", "ReviewRevealTranslation": "Toon de vertaling",
	"ReviewOriginalLabel": "oorspronkelijke zin:",
	"ReviewAgain":         "Wist ik niet", "ReviewGood": "Wist ik",

	"WordsTitle":         "Mijn woorden",
	"WordsEmpty":         "Nog geen woorden opgeslagen — tik tijdens het lezen op een woord om te beginnen.",
	"WordsKnowIt":        "✓ ken ik al, verwijderen",
	"WordsDelete":        "🗑 wissen",
	"WordsDeleteConfirm": "Dit woord definitief wissen? Het komt niet meer terug bij het herhalen.",

	"ReviewedTitle": "Laatste herhalingen",
	"ReviewedEmpty": "Nog geen herhalingen — dit vult zich zodra je een kaart op /review beantwoordt.",
	"ReviewedBox":   "box",

	"SettingsTitle": "Instellingen", "SettingsNativeLangLabel": "Mijn moedertaal",
	"SettingsNativeLangHint": "Wordt gebruikt voor alle vertalingen, ongeacht het boek.",
	"SettingsSave":           "Opslaan", "SettingsLoggedInAs": "Ingelogd als",
	"SettingsLogout": "uitloggen", "SettingsOther": "Andere…",
}
