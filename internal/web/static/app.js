// Selection -> translate -> save, on the reading view. No-op on any other page.
(function () {
  'use strict';
  var chapterEl = document.getElementById('chapter');
  if (!chapterEl) return;

  var popover = document.getElementById('translate-popover');
  var elPhrase = document.getElementById('tp-phrase');
  var elLoading = document.getElementById('tp-loading');
  var elError = document.getElementById('tp-error');
  var elResult = document.getElementById('tp-result');
  var elTranslation = document.getElementById('tp-translation');
  var elLemma = document.getElementById('tp-lemma');
  var elNote = document.getElementById('tp-note');
  var elSentence = document.getElementById('tp-sentence');
  var elSaved = document.getElementById('tp-saved');
  var elClose = document.getElementById('tp-close');

  var bookID = chapterEl.dataset.bookId;
  var chapterID = chapterEl.dataset.chapterId;

  function closePopover() {
    popover.hidden = true;
    window.getSelection().removeAllRanges();
  }

  function resetPopoverBody() {
    elLoading.hidden = true;
    elError.hidden = true;
    elResult.hidden = true;
    elSaved.textContent = '';
  }

  function contextFor(node) {
    var el = node.nodeType === 1 ? node : node.parentElement;
    while (el && !el.classList.contains('para')) el = el.parentElement;
    return el ? el.textContent.trim() : '';
  }

  // selectionchange fires continuously while a selection is being dragged —
  // once per character on a touch drag, or on every handle nudge on iPad.
  // Debounce so a translate call (and the auto-save it triggers) only
  // happens once the selection has actually settled, not once per fragment.
  var settleTimer = null;
  var SETTLE_MS = 500;

  document.addEventListener('selectionchange', function () {
    if (settleTimer) clearTimeout(settleTimer);
    settleTimer = setTimeout(function () {
      settleTimer = null;
      var sel = window.getSelection();
      if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;
      var text = sel.toString().trim();
      if (!text || !chapterEl.contains(sel.anchorNode)) return;

      openPopoverFor(text, contextFor(sel.anchorNode));
    }, SETTLE_MS);
  });

  var requestSeq = 0;

  function openPopoverFor(phrase, context) {
    var myReq = ++requestSeq;
    resetPopoverBody();
    elPhrase.textContent = phrase;
    popover.hidden = false;
    elLoading.hidden = false;

    fetch('/api/translate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        book_id: Number(bookID),
        chapter_id: Number(chapterID),
        phrase: phrase,
        context: context,
      }),
    })
      .then(function (res) {
        return res.json().then(function (body) { return { ok: res.ok, body: body }; });
      })
      .then(function (r) {
        if (myReq !== requestSeq) return; // superseded by a newer selection
        elLoading.hidden = true;
        if (!r.ok) {
          elError.hidden = false;
          elError.textContent = (r.body && r.body.error) || 'Erreur de traduction';
          return;
        }
        elTranslation.textContent = r.body.translation;
        elLemma.textContent = r.body.lemma || '';
        elLemma.hidden = !r.body.lemma;
        elNote.textContent = r.body.note || '';
        elNote.hidden = !r.body.note;
        elSentence.textContent = r.body.sentence_translation || '';
        elSentence.hidden = !r.body.sentence_translation;
        elSaved.textContent = r.body.saved ? '✓ enregistré dans mes mots' : '✓ déjà dans mes mots';
        elResult.hidden = false;
      })
      .catch(function () {
        if (myReq !== requestSeq) return;
        elLoading.hidden = true;
        elError.hidden = false;
        elError.textContent = 'Connexion impossible';
      });
  }

  elClose.addEventListener('click', closePopover);
})();
