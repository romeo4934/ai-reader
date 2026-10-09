package i18n

// The Agora as seen by visitors (not logged in).
var visitorStrings = map[string]Dict{
	"en": {
		"LandAgoraH":        "🏛️ The Lydi Agora",
		"LandAgora":         "Readers who keep up the daily challenge earn their seat in the Agora, a little world where everyone has their character and their rank.",
		"LandAgoraLink":     "Visit the Agora →",
		"AgoraVisitorHint":  "Drag to explore, tap someone to see who they are.",
		"AgoraVisitorPitch": "Every day, Lydi readers learn the words of the books they love. Complete 30 daily challenges and take your seat on the tiers, dressed as you choose.",
		"AgoraVisitorJoin":  "Create my account",
		"AgoraVisitorCount": "%d in the Agora",
	},
	"fr": {
		"LandAgoraH":        "🏛️ L'Agora de Lydi",
		"LandAgora":         "Les lecteurs qui tiennent le défi du jour gagnent leur place dans l'Agora, un petit monde où chacun a son personnage et son rang.",
		"LandAgoraLink":     "Visiter l'Agora →",
		"AgoraVisitorHint":  "Fais glisser pour explorer, touche quelqu'un pour savoir qui c'est.",
		"AgoraVisitorPitch": "Chaque jour, les lecteurs de Lydi apprennent les mots des livres qu'ils aiment. Réussis 30 défis du jour et prends ta place sur les gradins, dans la tenue de ton choix.",
		"AgoraVisitorJoin":  "Créer mon compte",
		"AgoraVisitorCount": "%d dans l'Agora",
	},
	"es": {
		"LandAgoraH":        "🏛️ El Ágora de Lydi",
		"LandAgora":         "Los lectores que cumplen el desafío diario se ganan su lugar en el Ágora, un pequeño mundo donde cada uno tiene su personaje y su rango.",
		"LandAgoraLink":     "Visitar el Ágora →",
		"AgoraVisitorHint":  "Desliza para explorar, toca a alguien para saber quién es.",
		"AgoraVisitorPitch": "Cada día, los lectores de Lydi aprenden las palabras de los libros que les gustan. Completa 30 desafíos diarios y ocupa tu lugar en las gradas, con la ropa que elijas.",
		"AgoraVisitorJoin":  "Crear mi cuenta",
		"AgoraVisitorCount": "%d en el Ágora",
	},
	"pt": {
		"LandAgoraH":        "🏛️ A Ágora do Lydi",
		"LandAgora":         "Os leitores que cumprem o desafio diário ganham o seu lugar na Ágora, um pequeno mundo onde cada um tem a sua personagem e o seu posto.",
		"LandAgoraLink":     "Visitar a Ágora →",
		"AgoraVisitorHint":  "Arrasta para explorar, toca em alguém para saber quem é.",
		"AgoraVisitorPitch": "Todos os dias, os leitores do Lydi aprendem as palavras dos livros de que gostam. Completa 30 desafios diários e ocupa o teu lugar nas bancadas, com a roupa que escolheres.",
		"AgoraVisitorJoin":  "Criar a minha conta",
		"AgoraVisitorCount": "%d na Ágora",
	},
	"it": {
		"LandAgoraH":        "🏛️ L'Agorà di Lydi",
		"LandAgora":         "I lettori che portano avanti la sfida quotidiana si guadagnano un posto nell'Agorà, un piccolo mondo dove ognuno ha il suo personaggio e il suo rango.",
		"LandAgoraLink":     "Visita l'Agorà →",
		"AgoraVisitorHint":  "Trascina per esplorare, tocca qualcuno per sapere chi è.",
		"AgoraVisitorPitch": "Ogni giorno, i lettori di Lydi imparano le parole dei libri che amano. Completa 30 sfide quotidiane e prendi posto sulle gradinate, vestito come preferisci.",
		"AgoraVisitorJoin":  "Crea il mio account",
		"AgoraVisitorCount": "%d nell'Agorà",
	},
	"de": {
		"LandAgoraH":        "🏛️ Die Lydi-Agora",
		"LandAgora":         "Wer die Tagesaufgabe durchhält, verdient sich einen Platz in der Agora, einer kleinen Welt, in der jeder seine Figur und seinen Rang hat.",
		"LandAgoraLink":     "Die Agora besuchen →",
		"AgoraVisitorHint":  "Zum Erkunden ziehen, jemanden antippen, um zu sehen, wer es ist.",
		"AgoraVisitorPitch": "Jeden Tag lernen die Leser von Lydi die Wörter der Bücher, die sie lieben. Schaffe 30 Tagesaufgaben und nimm deinen Platz auf den Rängen ein, gekleidet, wie du willst.",
		"AgoraVisitorJoin":  "Konto erstellen",
		"AgoraVisitorCount": "%d in der Agora",
	},
	"nl": {
		"LandAgoraH":        "🏛️ De Lydi-Agora",
		"LandAgora":         "Lezers die de dagelijkse uitdaging volhouden, verdienen een plek in de Agora, een kleine wereld waar iedereen een eigen personage en rang heeft.",
		"LandAgoraLink":     "De Agora bezoeken →",
		"AgoraVisitorHint":  "Sleep om rond te kijken, tik iemand aan om te zien wie het is.",
		"AgoraVisitorPitch": "Elke dag leren de lezers van Lydi de woorden van de boeken waar ze van houden. Haal 30 dagelijkse uitdagingen en neem plaats op de tribune, gekleed zoals jij wilt.",
		"AgoraVisitorJoin":  "Mijn account aanmaken",
		"AgoraVisitorCount": "%d in de Agora",
	},
}

func init() {
	for code, extra := range visitorStrings {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
