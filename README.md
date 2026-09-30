# ai-reader

Lire un livre pour apprendre une langue : sélectionner un mot ou une clause
fait apparaître sa traduction en contexte (via Claude), le mot part
automatiquement dans un tas de révision façon Anki (système Leitner, rappel
actif). Priorité à la lecture sur grand écran (iPad).

Multi-utilisateur : chacun a son compte, sa bibliothèque et son deck de mots,
privés. Inscriptions fermées par défaut, ouvertes via un code d'invitation
partagé.

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

1. **Comptes** (`/signup`, `/login`) — identifiant + mot de passe (bcrypt),
   pas d'email. `/signup` demande le code d'invitation défini dans
   `AI_READER_INVITE_CODE` ; vide = inscriptions désactivées. Chaque compte a
   sa propre langue maternelle, sa bibliothèque, son deck de mots — invisibles
   aux autres comptes.
2. **Bibliothèque** (`/`) — on dépose un `.epub`, il est parsé (métadonnées +
   texte de chaque chapitre, tags HTML retirés) et stocké en base, privé au
   compte qui l'a ajouté.
3. **Lecture** (`/books/{id}`) — sélectionner un mot (double-tap) ou une
   clause plus longue fait apparaître une bulle avec la phrase traduite, le
   mot mis en évidence dedans (Claude reçoit tout le paragraphe comme
   contexte). Enregistrement automatique dans le deck de révision — pas de
   bouton — sauf pour une sélection trop longue (> 12 mots, une phrase
   entière n'est pas un item de vocabulaire).
4. **Révision** (`/review`) — Leitner à 5 boîtes (intervalles 1/3/7/14/30
   jours), triée par fréquence réelle du mot (corpus anglais 10k mots,
   `internal/frequency`) puis par échéance — les mots courants d'abord.
   Rappel actif : Claude génère une nouvelle phrase à trous à chaque
   révision (pas la phrase d'origine), avec la traduction comme indice ; la
   phrase d'origine reste visible après révélation comme repère secondaire.
5. **Réglages** (`/settings`) — langue maternelle du compte connecté.

Sans `ANTHROPIC_API_KEY`, tout fonctionne sauf la traduction (`/api/translate`
répond 503) et le rappel actif (repli silencieux sur une carte de révision
statique) — la lecture, la bibliothèque et les mots déjà enregistrés restent
utilisables.

## Déploiement

```sh
cd /opt/ai-reader && git pull --ff-only && ./deploy/deploy.sh
```

Exposé publiquement sur `book.getlydi.com` (nginx + Let's Encrypt, même
schéma que calgoal) puisque plusieurs personnes hors du Tailscale d'Antoine
l'utilisent. `AI_READER_ADDR` écoute en local (127.0.0.1), nginx fait la
terminaison TLS.
