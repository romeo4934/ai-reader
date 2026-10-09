package i18n

// The evening reminder email and its setting.
var reminderStrings = map[string]Dict{
	"en": {
		"MailReminderSubject":       "%d cards are waiting for you on Lydi",
		"MailReminderSubjectStreak": "🔥 Your %d-day streak is on the line",
		"MailReminderBody":          "You haven't done today's challenge yet: %d cards are waiting for you (about %d min).\n\nReview now:\n%s\n\nTo stop getting this evening reminder:\n%s",
		"ReminderOffTitle":          "Reminders off",
		"ReminderOff":               "Done: you won't get the evening reminder anymore. You can turn it back on anytime in the settings.",
		"ReminderOffBad":            "This link is no longer valid.",
		"SettingsRemindersLabel":    "Evening reminder by email",
		"SettingsRemindersHint":     "An email around 7 pm if today's challenge isn't done yet.",
	},
	"fr": {
		"MailReminderSubject":       "%d cartes t'attendent sur Lydi",
		"MailReminderSubjectStreak": "🔥 Ta série de %d jours est en jeu",
		"MailReminderBody":          "Tu n'as pas encore fait ton défi du jour : %d cartes t'attendent (environ %d min).\n\nRéviser maintenant :\n%s\n\nPour ne plus recevoir ce rappel du soir :\n%s",
		"ReminderOffTitle":          "Rappels désactivés",
		"ReminderOff":               "C'est noté : tu ne recevras plus le rappel du soir. Tu peux le réactiver à tout moment dans les réglages.",
		"ReminderOffBad":            "Ce lien n'est plus valable.",
		"SettingsRemindersLabel":    "Rappel du soir par email",
		"SettingsRemindersHint":     "Un email vers 19 h si ton défi du jour n'est pas encore fait.",
	},
	"es": {
		"MailReminderSubject":       "%d tarjetas te esperan en Lydi",
		"MailReminderSubjectStreak": "🔥 Tu racha de %d días está en juego",
		"MailReminderBody":          "Todavía no has hecho el reto de hoy: %d tarjetas te esperan (unos %d min).\n\nRepasar ahora:\n%s\n\nPara dejar de recibir este recordatorio:\n%s",
		"ReminderOffTitle":          "Recordatorios desactivados",
		"ReminderOff":               "Hecho: ya no recibirás el recordatorio de la tarde. Puedes reactivarlo cuando quieras en los ajustes.",
		"ReminderOffBad":            "Este enlace ya no es válido.",
		"SettingsRemindersLabel":    "Recordatorio por email por la tarde",
		"SettingsRemindersHint":     "Un email hacia las 19 h si aún no has hecho el reto del día.",
	},
	"pt": {
		"MailReminderSubject":       "%d cartões esperam por você no Lydi",
		"MailReminderSubjectStreak": "🔥 Sua sequência de %d dias está em jogo",
		"MailReminderBody":          "Você ainda não fez o desafio de hoje: %d cartões esperam por você (cerca de %d min).\n\nRevisar agora:\n%s\n\nPara não receber mais este lembrete:\n%s",
		"ReminderOffTitle":          "Lembretes desativados",
		"ReminderOff":               "Pronto: você não vai mais receber o lembrete da noite. Pode reativá-lo quando quiser nas configurações.",
		"ReminderOffBad":            "Este link não é mais válido.",
		"SettingsRemindersLabel":    "Lembrete por email à noite",
		"SettingsRemindersHint":     "Um email por volta das 19 h se o desafio do dia ainda não estiver feito.",
	},
	"it": {
		"MailReminderSubject":       "%d carte ti aspettano su Lydi",
		"MailReminderSubjectStreak": "🔥 La tua serie di %d giorni è in gioco",
		"MailReminderBody":          "Non hai ancora fatto la sfida di oggi: %d carte ti aspettano (circa %d min).\n\nRipassa ora:\n%s\n\nPer non ricevere più questo promemoria:\n%s",
		"ReminderOffTitle":          "Promemoria disattivati",
		"ReminderOff":               "Fatto: non riceverai più il promemoria serale. Puoi riattivarlo quando vuoi nelle impostazioni.",
		"ReminderOffBad":            "Questo link non è più valido.",
		"SettingsRemindersLabel":    "Promemoria serale via email",
		"SettingsRemindersHint":     "Un'email verso le 19 se la sfida del giorno non è ancora fatta.",
	},
	"de": {
		"MailReminderSubject":       "%d Karten warten auf dich bei Lydi",
		"MailReminderSubjectStreak": "🔥 Deine Serie von %d Tagen steht auf dem Spiel",
		"MailReminderBody":          "Du hast das heutige Tagesziel noch nicht geschafft: %d Karten warten auf dich (etwa %d Min.).\n\nJetzt wiederholen:\n%s\n\nUm diese Abend-Erinnerung nicht mehr zu bekommen:\n%s",
		"ReminderOffTitle":          "Erinnerungen aus",
		"ReminderOff":               "Erledigt: du bekommst keine Abend-Erinnerung mehr. Du kannst sie jederzeit in den Einstellungen wieder einschalten.",
		"ReminderOffBad":            "Dieser Link ist nicht mehr gültig.",
		"SettingsRemindersLabel":    "Abend-Erinnerung per E-Mail",
		"SettingsRemindersHint":     "Eine E-Mail gegen 19 Uhr, wenn dein Tagesziel noch offen ist.",
	},
	"nl": {
		"MailReminderSubject":       "%d kaarten wachten op je bij Lydi",
		"MailReminderSubjectStreak": "🔥 Je reeks van %d dagen staat op het spel",
		"MailReminderBody":          "Je hebt het dagdoel van vandaag nog niet gehaald: %d kaarten wachten op je (ongeveer %d min).\n\nNu herhalen:\n%s\n\nOm deze avondherinnering niet meer te krijgen:\n%s",
		"ReminderOffTitle":          "Herinneringen uit",
		"ReminderOff":               "Gedaan: je krijgt de avondherinnering niet meer. Je kunt hem altijd weer aanzetten in de instellingen.",
		"ReminderOffBad":            "Deze link is niet meer geldig.",
		"SettingsRemindersLabel":    "Avondherinnering per e-mail",
		"SettingsRemindersHint":     "Een e-mail rond 19 uur als je dagdoel nog niet gehaald is.",
	},
}

func init() {
	for code, extra := range reminderStrings {
		for k, v := range extra {
			byCode[code][k] = v
		}
	}
}
