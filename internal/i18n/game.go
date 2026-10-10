package i18n

// The word game. Its name is a placeholder until one is picked.
var gameStrings = map[string]Dict{
	"en": {
		"NavGame":       "game",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 words from your list, 4 answers each. Find the right translation: the faster, the more points (%d seconds per word).",
		"GameBest":      "Your record: %d / %d",
		"GamePlay":      "Play",
		"GameListen":    "Listen",
		"GameNewRecord": "New record! 🎉",
		"GameAgain":     "Play again",
		"GameTooFew":    "Not enough words to play yet: look up a few words while you read, then come back.",
	},
	"fr": {
		"NavGame":       "jeu",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 mots de ta liste, 4 réponses à chaque fois. Trouve la bonne traduction : plus tu vas vite, plus tu marques (%d secondes par mot).",
		"GameBest":      "Ton record : %d / %d",
		"GamePlay":      "Jouer",
		"GameListen":    "Écouter",
		"GameNewRecord": "Nouveau record ! 🎉",
		"GameAgain":     "Rejouer",
		"GameTooFew":    "Pas encore assez de mots pour jouer : cherche quelques mots en lisant, puis reviens.",
	},
	"es": {
		"NavGame":       "juego",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 palabras de tu lista, 4 respuestas cada vez. Encuentra la traducción correcta: cuanto más rápido, más puntos (%d segundos por palabra).",
		"GameBest":      "Tu récord: %d / %d",
		"GamePlay":      "Jugar",
		"GameListen":    "Escuchar",
		"GameNewRecord": "¡Nuevo récord! 🎉",
		"GameAgain":     "Jugar otra vez",
		"GameTooFew":    "Aún no hay suficientes palabras para jugar: busca algunas mientras lees y vuelve.",
	},
	"pt": {
		"NavGame":       "jogo",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 palavras da sua lista, 4 respostas cada. Encontre a tradução certa: quanto mais rápido, mais pontos (%d segundos por palavra).",
		"GameBest":      "Seu recorde: %d / %d",
		"GamePlay":      "Jogar",
		"GameListen":    "Ouvir",
		"GameNewRecord": "Novo recorde! 🎉",
		"GameAgain":     "Jogar de novo",
		"GameTooFew":    "Ainda não há palavras suficientes para jogar: procure algumas enquanto lê e volte.",
	},
	"it": {
		"NavGame":       "gioco",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 parole dalla tua lista, 4 risposte ciascuna. Trova la traduzione giusta: più sei veloce, più punti fai (%d secondi per parola).",
		"GameBest":      "Il tuo record: %d / %d",
		"GamePlay":      "Gioca",
		"GameListen":    "Ascolta",
		"GameNewRecord": "Nuovo record! 🎉",
		"GameAgain":     "Gioca ancora",
		"GameTooFew":    "Non ci sono ancora abbastanza parole per giocare: cercane qualcuna mentre leggi, poi torna.",
	},
	"de": {
		"NavGame":       "Spiel",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 Wörter aus deiner Liste, je 4 Antworten. Finde die richtige Übersetzung: je schneller, desto mehr Punkte (%d Sekunden pro Wort).",
		"GameBest":      "Dein Rekord: %d / %d",
		"GamePlay":      "Spielen",
		"GameListen":    "Anhören",
		"GameNewRecord": "Neuer Rekord! 🎉",
		"GameAgain":     "Nochmal spielen",
		"GameTooFew":    "Noch nicht genug Wörter zum Spielen: schlag beim Lesen ein paar Wörter nach und komm dann zurück.",
	},
	"nl": {
		"NavGame":       "spel",
		"GameTitle":     "Adieulingo",
		"GameLead":      "5 woorden uit je lijst, telkens 4 antwoorden. Vind de juiste vertaling: hoe sneller, hoe meer punten (%d seconden per woord).",
		"GameBest":      "Je record: %d / %d",
		"GamePlay":      "Spelen",
		"GameListen":    "Luisteren",
		"GameNewRecord": "Nieuw record! 🎉",
		"GameAgain":     "Opnieuw spelen",
		"GameTooFew":    "Nog niet genoeg woorden om te spelen: zoek er een paar op tijdens het lezen en kom dan terug.",
	},
}

func init() {
	for code, extra := range gameStrings {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
