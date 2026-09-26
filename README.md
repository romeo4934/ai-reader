# ai-reader

Lire un livre pour apprendre une langue : on sélectionne un bout de phrase, l'IA
le traduit en tenant compte de la phrase autour, et le mot part dans un tas de
révision façon Anki (système Leitner). Pour une personne, priorité à la lecture
sur grand écran (iPad).

## Stack

Un binaire Go, une base SQLite, systemd. Pas de build front séparé — le HTML,
le CSS et le JS sont servis tels quels et embarqués dans le binaire (`embed.FS`).

```
/opt/ai-reader/
  .env        secrets (chmod 640 root:ai-reader), jamais commité
  data/       base SQLite
/usr/local/bin/ai-reader
```

## Fonctionnement

1. **Bibliothèque** (`/`) — on dépose un `.epub`, il est parsé (métadonnées +
   texte de chaque chapitre, tags HTML retirés) et stocké en base.
2. **Lecture** (`/books/{id}`) — le texte s'affiche en gros, sélectionner un
   mot ou une expression fait apparaître une bulle de traduction contextuelle
   (Claude reçoit la phrase entière autour de la sélection). Un bouton
   enregistre le mot avec sa phrase d'origine.
3. **Révision** (`/review`) — système Leitner à 5 boîtes (intervalles
   1 / 3 / 7 / 14 / 30 jours). La carte montre la phrase d'origine avec le mot
   surligné, jamais le mot isolé.
4. **Réglages** (`/settings`) — une seule langue maternelle pour toutes les
   traductions ; la langue du livre est détectée depuis les métadonnées epub.

Sans `ANTHROPIC_API_KEY`, tout fonctionne sauf la traduction (`/api/translate`
répond 503) — la lecture, la bibliothèque et la révision des mots déjà
enregistrés restent utilisables.

## Déploiement

```sh
cd /opt/ai-reader && git pull --ff-only && ./deploy/deploy.sh
```

Pas de nginx ni de domaine pour l'instant : le service écoute directement sur
l'IP Tailscale de la machine (`AI_READER_ADDR` dans `.env`), accessible depuis
l'iPad via l'app Tailscale. À revoir si un accès public devient utile.
