package i18n

import "strings"

// Languages lists the supported UI languages in the order the landing page's
// language switcher shows them, with each one's name in its own language.
var Languages = []struct{ Code, Name string }{
	{"en", "English"}, {"fr", "Français"}, {"es", "Español"}, {"pt", "Português"},
	{"it", "Italiano"}, {"de", "Deutsch"}, {"nl", "Nederlands"},
}

var byCode = map[string]Dict{"en": en, "fr": fr, "es": es, "pt": pt, "it": it, "de": de, "nl": nl}

// ByCode returns the dictionary for a two-letter language code; ok is false
// for an unsupported code.
func ByCode(code string) (Dict, bool) {
	d, ok := byCode[strings.ToLower(code)]
	return d, ok
}

// ForVisitor picks the UI language for someone without an account yet, from
// their browser's Accept-Language header — the first supported language in
// the list wins (q-values are ignored: browsers already send the list in
// preference order). English when nothing matches, since a visitor arriving
// from anywhere is likelier to read English than French.
func ForVisitor(acceptLanguage string) Dict {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		primary := strings.SplitN(tag, "-", 2)[0]
		if d, ok := ByCode(primary); ok {
			return d
		}
	}
	return en
}

// The landing page's text lives here rather than in the main dictionaries:
// it's long marketing copy, not interface labels, and merging it at init
// keeps i18n.go readable.
//
// The demo card shows a public-domain opening line with one word translated
// into the visitor's language — Don Quijote for English readers (an English
// book translated into English would demo nothing), Pride and Prejudice for
// everyone else.
var landing = map[string]Dict{
	"en": {
		"LandTitle":    "Lydi — learn a language by reading real books",
		"LandMetaDesc": "Read your epub books in the original language: tap a word and its translation appears in the context of the sentence, then it joins your reviews.",
		"LandH1":       "Learn a language by reading real books.",
		"LandLead":     "Open your epub and tap a word you don't know: its translation appears in the context of the sentence. The word goes straight into your reviews, so it sticks.",
		"LandCTA":      "Start for free", "LandLogin": "I already have an account",
		"LandDemoAria": "Example: a word tapped while reading, and its translation",
		"LandDemoBook": "Don Quijote", "LandDemoChapter": "chapter 1",
		"LandDemoBefore": "En un lugar de la Mancha, de cuyo nombre no quiero", "LandDemoWord": "acordarme",
		"LandDemoAfter":      ", no ha mucho tiempo que vivía un hidalgo…",
		"LandDemoWordTr":     "to remember",
		"LandDemoSentBefore": "“In a village of La Mancha, whose name I have no wish to", "LandDemoSentWord": "remember",
		"LandDemoSentAfter": ", there lived not long ago a gentleman…”",
		"LandDemoSaved":     "✓ added to your reviews",
		"LandStepsTitle":    "How it works",
		"LandStep1T":        "Add your book", "LandStep1": "Drop in any .epub file. Your library is private: only you can see it.",
		"LandStep2T": "Read and tap", "LandStep2": "A word or a phrase escapes you? Tap it. The translation takes the whole paragraph into account, not just the dictionary.",
		"LandStep3T": "Review what matters", "LandStep3": "Your words come back at the right time, in fresh sentences, most frequent first. Your time goes to what will actually help you read.",
		"LandPitch1T": "Made for tablets.", "LandPitch1": "Large text, simple scrolling, built for calm reading.",
		"LandPitch2T": "In your language.", "LandPitch2": "Interface and translations in English, French, Spanish, Portuguese, Italian, German or Dutch.",
		"LandPitch3T": "No hassle.", "LandPitch3": "No flashcards to make by hand: you read, the rest happens on its own.",
		"LandFinalH": "Your next book, in the original.", "LandFinalCTA": "Create my account",
		"LandLangLabel": "Language",
	},
	"fr": {
		"LandTitle":    "Lydi — apprendre une langue en lisant de vrais livres",
		"LandMetaDesc": "Lis tes livres epub en version originale : touche un mot, sa traduction apparaît dans le contexte de la phrase, et il part dans tes révisions.",
		"LandH1":       "Apprends une langue en lisant de vrais livres.",
		"LandLead":     "Ouvre ton epub, touche un mot que tu ne connais pas : sa traduction apparaît dans le contexte de la phrase. Le mot rejoint automatiquement tes révisions, pour que tu le retiennes.",
		"LandCTA":      "Commencer gratuitement", "LandLogin": "J'ai déjà un compte",
		"LandDemoAria": "Exemple : un mot touché pendant la lecture et sa traduction",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "chapitre 1",
		"LandDemoWordTr":     "reconnue",
		"LandDemoSentBefore": "« C'est une vérité universellement", "LandDemoSentWord": "reconnue",
		"LandDemoSentAfter": " qu'un célibataire pourvu d'une belle fortune doit être en quête d'une épouse. »",
		"LandDemoSaved":     "✓ ajouté à tes révisions",
		"LandStepsTitle":    "Comment ça marche",
		"LandStep1T":        "Ajoute ton livre", "LandStep1": "Dépose n'importe quel fichier .epub. Ta bibliothèque est privée, toi seul la vois.",
		"LandStep2T": "Lis et touche", "LandStep2": "Un mot ou un bout de phrase t'échappe ? Touche-le. La traduction tient compte du paragraphe, pas seulement du dictionnaire.",
		"LandStep3T": "Révise ce qui compte", "LandStep3": "Tes mots reviennent au bon moment, dans des phrases nouvelles, les plus fréquents d'abord. Tu passes ton temps sur ce qui t'aidera vraiment à lire.",
		"LandPitch1T": "Pensé pour la tablette.", "LandPitch1": "Grand texte, défilement simple, pensé pour lire au calme.",
		"LandPitch2T": "Dans ta langue.", "LandPitch2": "Interface et traductions en français, anglais, espagnol, portugais, italien, allemand ou néerlandais.",
		"LandPitch3T": "Sans prise de tête.", "LandPitch3": "Pas de cartes à créer à la main : tu lis, le reste se fait tout seul.",
		"LandFinalH": "Ton prochain livre, en version originale.", "LandFinalCTA": "Créer mon compte",
		"LandLangLabel": "Langue",
	},
	"es": {
		"LandTitle":    "Lydi — aprende un idioma leyendo libros de verdad",
		"LandMetaDesc": "Lee tus libros epub en versión original: toca una palabra y su traducción aparece en el contexto de la frase, y pasa a tus repasos.",
		"LandH1":       "Aprende un idioma leyendo libros de verdad.",
		"LandLead":     "Abre tu epub y toca una palabra que no conoces: su traducción aparece en el contexto de la frase. La palabra pasa automáticamente a tus repasos, para que no se te olvide.",
		"LandCTA":      "Empezar gratis", "LandLogin": "Ya tengo una cuenta",
		"LandDemoAria": "Ejemplo: una palabra tocada durante la lectura y su traducción",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "capítulo 1",
		"LandDemoWordTr":     "reconocida",
		"LandDemoSentBefore": "«Es una verdad universalmente", "LandDemoSentWord": "reconocida",
		"LandDemoSentAfter": " que un hombre soltero, poseedor de una gran fortuna, necesita una esposa.»",
		"LandDemoSaved":     "✓ añadida a tus repasos",
		"LandStepsTitle":    "Cómo funciona",
		"LandStep1T":        "Añade tu libro", "LandStep1": "Sube cualquier archivo .epub. Tu biblioteca es privada: solo tú la ves.",
		"LandStep2T": "Lee y toca", "LandStep2": "¿Se te escapa una palabra o una frase? Tócala. La traducción tiene en cuenta todo el párrafo, no solo el diccionario.",
		"LandStep3T": "Repasa lo que importa", "LandStep3": "Tus palabras vuelven en el momento justo, en frases nuevas, las más frecuentes primero. Dedicas tu tiempo a lo que de verdad te ayuda a leer.",
		"LandPitch1T": "Pensado para tablet.", "LandPitch1": "Texto grande, desplazamiento sencillo, para leer con calma.",
		"LandPitch2T": "En tu idioma.", "LandPitch2": "Interfaz y traducciones en español, inglés, francés, portugués, italiano, alemán o neerlandés.",
		"LandPitch3T": "Sin complicaciones.", "LandPitch3": "Nada de tarjetas hechas a mano: tú lees, lo demás se hace solo.",
		"LandFinalH": "Tu próximo libro, en versión original.", "LandFinalCTA": "Crear mi cuenta",
		"LandLangLabel": "Idioma",
	},
	"pt": {
		"LandTitle":    "Lydi — aprenda um idioma lendo livros de verdade",
		"LandMetaDesc": "Leia seus livros epub no idioma original: toque numa palavra e a tradução aparece no contexto da frase, depois ela vai para as suas revisões.",
		"LandH1":       "Aprenda um idioma lendo livros de verdade.",
		"LandLead":     "Abra seu epub e toque numa palavra que você não conhece: a tradução aparece no contexto da frase. A palavra vai automaticamente para as suas revisões, para você não esquecer.",
		"LandCTA":      "Começar grátis", "LandLogin": "Já tenho uma conta",
		"LandDemoAria": "Exemplo: uma palavra tocada durante a leitura e sua tradução",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "capítulo 1",
		"LandDemoWordTr":     "reconhecida",
		"LandDemoSentBefore": "“É uma verdade universalmente", "LandDemoSentWord": "reconhecida",
		"LandDemoSentAfter": " que um homem solteiro, possuidor de uma boa fortuna, deve estar à procura de uma esposa.”",
		"LandDemoSaved":     "✓ adicionada às suas revisões",
		"LandStepsTitle":    "Como funciona",
		"LandStep1T":        "Adicione seu livro", "LandStep1": "Envie qualquer arquivo .epub. Sua biblioteca é privada: só você a vê.",
		"LandStep2T": "Leia e toque", "LandStep2": "Uma palavra ou expressão escapou? Toque nela. A tradução leva em conta o parágrafo inteiro, não só o dicionário.",
		"LandStep3T": "Revise o que importa", "LandStep3": "Suas palavras voltam na hora certa, em frases novas, as mais frequentes primeiro. Seu tempo vai para o que realmente ajuda a ler.",
		"LandPitch1T": "Feito para tablet.", "LandPitch1": "Texto grande, rolagem simples, para ler com calma.",
		"LandPitch2T": "No seu idioma.", "LandPitch2": "Interface e traduções em português, inglês, francês, espanhol, italiano, alemão ou holandês.",
		"LandPitch3T": "Sem complicação.", "LandPitch3": "Nada de criar cartões à mão: você lê, o resto acontece sozinho.",
		"LandFinalH": "Seu próximo livro, no original.", "LandFinalCTA": "Criar minha conta",
		"LandLangLabel": "Idioma",
	},
	"it": {
		"LandTitle":    "Lydi — impara una lingua leggendo libri veri",
		"LandMetaDesc": "Leggi i tuoi libri epub in lingua originale: tocca una parola e la traduzione appare nel contesto della frase, poi finisce nei tuoi ripassi.",
		"LandH1":       "Impara una lingua leggendo libri veri.",
		"LandLead":     "Apri il tuo epub e tocca una parola che non conosci: la traduzione appare nel contesto della frase. La parola finisce automaticamente nei tuoi ripassi, così te la ricordi.",
		"LandCTA":      "Inizia gratis", "LandLogin": "Ho già un account",
		"LandDemoAria": "Esempio: una parola toccata durante la lettura e la sua traduzione",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "capitolo 1",
		"LandDemoWordTr":     "riconosciuta",
		"LandDemoSentBefore": "«È una verità universalmente", "LandDemoSentWord": "riconosciuta",
		"LandDemoSentAfter": " che uno scapolo in possesso di un cospicuo patrimonio debba essere in cerca di moglie.»",
		"LandDemoSaved":     "✓ aggiunta ai tuoi ripassi",
		"LandStepsTitle":    "Come funziona",
		"LandStep1T":        "Aggiungi il tuo libro", "LandStep1": "Carica un qualsiasi file .epub. La tua biblioteca è privata: la vedi solo tu.",
		"LandStep2T": "Leggi e tocca", "LandStep2": "Ti sfugge una parola o un'espressione? Toccala. La traduzione tiene conto di tutto il paragrafo, non solo del dizionario.",
		"LandStep3T": "Ripassa ciò che conta", "LandStep3": "Le tue parole tornano al momento giusto, in frasi nuove, le più frequenti per prime. Il tuo tempo va a ciò che ti aiuta davvero a leggere.",
		"LandPitch1T": "Pensato per il tablet.", "LandPitch1": "Testo grande, scorrimento semplice, per leggere con calma.",
		"LandPitch2T": "Nella tua lingua.", "LandPitch2": "Interfaccia e traduzioni in italiano, inglese, francese, spagnolo, portoghese, tedesco o olandese.",
		"LandPitch3T": "Senza complicazioni.", "LandPitch3": "Nessuna flashcard da creare a mano: tu leggi, il resto si fa da solo.",
		"LandFinalH": "Il tuo prossimo libro, in lingua originale.", "LandFinalCTA": "Crea il mio account",
		"LandLangLabel": "Lingua",
	},
	"de": {
		"LandTitle":    "Lydi — eine Sprache lernen mit echten Büchern",
		"LandMetaDesc": "Lies deine epub-Bücher im Original: Tippe auf ein Wort, die Übersetzung erscheint im Kontext des Satzes, und es landet in deinen Wiederholungen.",
		"LandH1":       "Lerne eine Sprache mit echten Büchern.",
		"LandLead":     "Öffne dein epub und tippe auf ein Wort, das du nicht kennst: Die Übersetzung erscheint im Kontext des Satzes. Das Wort landet automatisch in deinen Wiederholungen, damit es hängen bleibt.",
		"LandCTA":      "Kostenlos starten", "LandLogin": "Ich habe schon ein Konto",
		"LandDemoAria": "Beispiel: ein beim Lesen angetipptes Wort und seine Übersetzung",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "Kapitel 1",
		"LandDemoWordTr":     "anerkannt",
		"LandDemoSentBefore": "„Es ist eine allgemein", "LandDemoSentWord": "anerkannte",
		"LandDemoSentAfter": " Wahrheit, dass ein alleinstehender Mann mit einem schönen Vermögen nach einer Frau suchen muss.“",
		"LandDemoSaved":     "✓ zu deinen Wiederholungen hinzugefügt",
		"LandStepsTitle":    "So funktioniert's",
		"LandStep1T":        "Buch hinzufügen", "LandStep1": "Lade eine beliebige .epub-Datei hoch. Deine Bibliothek ist privat: Nur du siehst sie.",
		"LandStep2T": "Lesen und tippen", "LandStep2": "Ein Wort oder eine Wendung entgeht dir? Tippe darauf. Die Übersetzung berücksichtigt den ganzen Absatz, nicht nur das Wörterbuch.",
		"LandStep3T": "Wiederholen, was zählt", "LandStep3": "Deine Wörter kommen zur richtigen Zeit zurück, in neuen Sätzen, die häufigsten zuerst. So nutzt du deine Zeit für das, was dir beim Lesen wirklich hilft.",
		"LandPitch1T": "Fürs Tablet gemacht.", "LandPitch1": "Große Schrift, einfaches Scrollen, für ruhiges Lesen.",
		"LandPitch2T": "In deiner Sprache.", "LandPitch2": "Oberfläche und Übersetzungen auf Deutsch, Englisch, Französisch, Spanisch, Portugiesisch, Italienisch oder Niederländisch.",
		"LandPitch3T": "Ohne Aufwand.", "LandPitch3": "Keine Karteikarten von Hand: Du liest, der Rest passiert von selbst.",
		"LandFinalH": "Dein nächstes Buch, im Original.", "LandFinalCTA": "Mein Konto erstellen",
		"LandLangLabel": "Sprache",
	},
	"nl": {
		"LandTitle":    "Lydi — leer een taal door echte boeken te lezen",
		"LandMetaDesc": "Lees je epub-boeken in de originele taal: tik op een woord en de vertaling verschijnt in de context van de zin, daarna gaat het naar je herhalingen.",
		"LandH1":       "Leer een taal door echte boeken te lezen.",
		"LandLead":     "Open je epub en tik op een woord dat je niet kent: de vertaling verschijnt in de context van de zin. Het woord gaat automatisch naar je herhalingen, zodat je het onthoudt.",
		"LandCTA":      "Gratis beginnen", "LandLogin": "Ik heb al een account",
		"LandDemoAria": "Voorbeeld: een woord aangetikt tijdens het lezen en de vertaling",
		"LandDemoBook": "Pride and Prejudice", "LandDemoChapter": "hoofdstuk 1",
		"LandDemoWordTr":     "erkend",
		"LandDemoSentBefore": "“Het is een algemeen", "LandDemoSentWord": "erkende",
		"LandDemoSentAfter": " waarheid dat een vrijgezel met een aardig fortuin op zoek moet zijn naar een vrouw.”",
		"LandDemoSaved":     "✓ toegevoegd aan je herhalingen",
		"LandStepsTitle":    "Zo werkt het",
		"LandStep1T":        "Voeg je boek toe", "LandStep1": "Upload een willekeurig .epub-bestand. Je bibliotheek is privé: alleen jij ziet hem.",
		"LandStep2T": "Lees en tik", "LandStep2": "Ontgaat je een woord of uitdrukking? Tik erop. De vertaling houdt rekening met de hele alinea, niet alleen met het woordenboek.",
		"LandStep3T": "Herhaal wat telt", "LandStep3": "Je woorden komen op het juiste moment terug, in nieuwe zinnen, de meest voorkomende eerst. Je tijd gaat naar wat je echt helpt om te lezen.",
		"LandPitch1T": "Gemaakt voor de tablet.", "LandPitch1": "Grote letters, eenvoudig scrollen, om rustig te lezen.",
		"LandPitch2T": "In jouw taal.", "LandPitch2": "Interface en vertalingen in het Nederlands, Engels, Frans, Spaans, Portugees, Italiaans of Duits.",
		"LandPitch3T": "Zonder gedoe.", "LandPitch3": "Geen flashcards met de hand maken: jij leest, de rest gaat vanzelf.",
		"LandFinalH": "Je volgende boek, in het origineel.", "LandFinalCTA": "Mijn account aanmaken",
		"LandLangLabel": "Taal",
	},
}

// Every non-English demo uses the same Pride and Prejudice line.
var prideDemo = Dict{
	"LandDemoBefore": "It is a truth universally", "LandDemoWord": "acknowledged",
	"LandDemoAfter": ", that a single man in possession of a good fortune, must be in want of a wife.",
}

func init() {
	for code, extra := range landing {
		d := byCode[code]
		if code != "en" {
			for k, v := range prideDemo {
				d[k] = v
			}
		}
		for k, v := range extra {
			d[k] = v
		}
	}
}
