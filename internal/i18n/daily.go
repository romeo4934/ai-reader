package i18n

// The daily review challenge (progress bar, "done" screen, streak) and its
// setting. Strings with %d are fmt verbs filled in by the caller.
var daily = map[string]Dict{
	"en": {
		"DailyProgress": "Today: %d / %d", "DailyNewBadge": "new",
		"DailyDoneTitle": "Today's challenge done!", "DailyDoneBody": "You reviewed %d cards today. See you tomorrow.",
		"DailyNothingToday": "Nothing to review today: your streak carries on. Read a bit and tap unknown words to add more.",
		"DailyStreakOne":    "🔥 1 day in a row", "DailyStreakMany": "🔥 %d days in a row",
		"DailyStreakHint":       "Missing one day a week won't break your streak.",
		"DailyWaiting":          "%d new words are waiting their turn: they'll come in over the next days.",
		"DailyMoreBtn":          "Add %d new words now",
		"SettingsDailyNewLabel": "New words per day",
		"SettingsDailyNewHint":  "Due reviews always come first. Beyond this limit, new words wait for the following days.",
	},
	"fr": {
		"DailyProgress": "Aujourd'hui : %d / %d", "DailyNewBadge": "nouveau",
		"DailyDoneTitle": "Défi du jour réussi !", "DailyDoneBody": "Tu as révisé %d cartes aujourd'hui. À demain !",
		"DailyNothingToday": "Rien à réviser aujourd'hui : ta série continue. Lis un peu et touche les mots inconnus pour en ajouter.",
		"DailyStreakOne":    "🔥 1 jour d'affilée", "DailyStreakMany": "🔥 %d jours d'affilée",
		"DailyStreakHint":       "Un jour manqué par semaine ne casse pas ta série.",
		"DailyWaiting":          "%d nouveaux mots attendent leur tour : ils arriveront les jours suivants.",
		"DailyMoreBtn":          "Ajouter %d nouveaux mots maintenant",
		"SettingsDailyNewLabel": "Nouveaux mots par jour",
		"SettingsDailyNewHint":  "Les révisions dues passent toujours en premier. Au-delà de cette limite, les nouveaux mots attendent les jours suivants.",
	},
	"es": {
		"DailyProgress": "Hoy: %d / %d", "DailyNewBadge": "nueva",
		"DailyDoneTitle": "¡Reto del día cumplido!", "DailyDoneBody": "Has repasado %d tarjetas hoy. ¡Hasta mañana!",
		"DailyNothingToday": "Nada que repasar hoy: tu racha sigue. Lee un poco y toca las palabras que no conoces para añadir más.",
		"DailyStreakOne":    "🔥 1 día seguido", "DailyStreakMany": "🔥 %d días seguidos",
		"DailyStreakHint":       "Fallar un día por semana no rompe tu racha.",
		"DailyWaiting":          "%d palabras nuevas esperan su turno: llegarán en los próximos días.",
		"DailyMoreBtn":          "Añadir %d palabras nuevas ahora",
		"SettingsDailyNewLabel": "Palabras nuevas por día",
		"SettingsDailyNewHint":  "Los repasos pendientes siempre van primero. Más allá de este límite, las palabras nuevas esperan a los días siguientes.",
	},
	"pt": {
		"DailyProgress": "Hoje: %d / %d", "DailyNewBadge": "nova",
		"DailyDoneTitle": "Desafio do dia concluído!", "DailyDoneBody": "Você revisou %d cartões hoje. Até amanhã!",
		"DailyNothingToday": "Nada para revisar hoje: sua sequência continua. Leia um pouco e toque nas palavras desconhecidas para adicionar mais.",
		"DailyStreakOne":    "🔥 1 dia seguido", "DailyStreakMany": "🔥 %d dias seguidos",
		"DailyStreakHint":       "Perder um dia por semana não quebra sua sequência.",
		"DailyWaiting":          "%d palavras novas esperam a vez: elas chegam nos próximos dias.",
		"DailyMoreBtn":          "Adicionar %d palavras novas agora",
		"SettingsDailyNewLabel": "Palavras novas por dia",
		"SettingsDailyNewHint":  "As revisões pendentes sempre vêm primeiro. Além desse limite, as palavras novas esperam os dias seguintes.",
	},
	"it": {
		"DailyProgress": "Oggi: %d / %d", "DailyNewBadge": "nuova",
		"DailyDoneTitle": "Sfida del giorno completata!", "DailyDoneBody": "Hai ripassato %d carte oggi. A domani!",
		"DailyNothingToday": "Niente da ripassare oggi: la tua serie continua. Leggi un po' e tocca le parole che non conosci per aggiungerne altre.",
		"DailyStreakOne":    "🔥 1 giorno di fila", "DailyStreakMany": "🔥 %d giorni di fila",
		"DailyStreakHint":       "Saltare un giorno a settimana non interrompe la tua serie.",
		"DailyWaiting":          "%d parole nuove aspettano il loro turno: arriveranno nei prossimi giorni.",
		"DailyMoreBtn":          "Aggiungi %d parole nuove ora",
		"SettingsDailyNewLabel": "Parole nuove al giorno",
		"SettingsDailyNewHint":  "I ripassi in scadenza vengono sempre prima. Oltre questo limite, le parole nuove aspettano i giorni successivi.",
	},
	"de": {
		"DailyProgress": "Heute: %d / %d", "DailyNewBadge": "neu",
		"DailyDoneTitle": "Tagesziel geschafft!", "DailyDoneBody": "Du hast heute %d Karten wiederholt. Bis morgen!",
		"DailyNothingToday": "Heute gibt es nichts zu wiederholen: deine Serie läuft weiter. Lies ein bisschen und tippe unbekannte Wörter an, um neue hinzuzufügen.",
		"DailyStreakOne":    "🔥 1 Tag in Folge", "DailyStreakMany": "🔥 %d Tage in Folge",
		"DailyStreakHint":       "Ein verpasster Tag pro Woche unterbricht deine Serie nicht.",
		"DailyWaiting":          "%d neue Wörter warten: sie kommen in den nächsten Tagen dran.",
		"DailyMoreBtn":          "Jetzt %d neue Wörter hinzufügen",
		"SettingsDailyNewLabel": "Neue Wörter pro Tag",
		"SettingsDailyNewHint":  "Fällige Wiederholungen kommen immer zuerst. Über dieses Limit hinaus warten neue Wörter auf die nächsten Tage.",
	},
	"nl": {
		"DailyProgress": "Vandaag: %d / %d", "DailyNewBadge": "nieuw",
		"DailyDoneTitle": "Dagdoel gehaald!", "DailyDoneBody": "Je hebt vandaag %d kaarten herhaald. Tot morgen!",
		"DailyNothingToday": "Vandaag niets te herhalen: je reeks loopt door. Lees wat en tik op onbekende woorden om er meer toe te voegen.",
		"DailyStreakOne":    "🔥 1 dag op rij", "DailyStreakMany": "🔥 %d dagen op rij",
		"DailyStreakHint":       "Eén gemiste dag per week breekt je reeks niet.",
		"DailyWaiting":          "%d nieuwe woorden wachten op hun beurt: ze komen de komende dagen.",
		"DailyMoreBtn":          "Nu %d nieuwe woorden toevoegen",
		"SettingsDailyNewLabel": "Nieuwe woorden per dag",
		"SettingsDailyNewHint":  "Herhalingen die aan de beurt zijn gaan altijd voor. Boven deze limiet wachten nieuwe woorden op de volgende dagen.",
	},
}

func init() {
	for code, extra := range daily {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
