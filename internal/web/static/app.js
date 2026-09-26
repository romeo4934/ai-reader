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

  document.addEventListener('selectionchange', function () {
    var sel = window.getSelection();
    if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;
    var text = sel.toString().trim();
    if (!text || !chapterEl.contains(sel.anchorNode)) return;

    var context = contextFor(sel.anchorNode);
    openPopoverFor(text, context);
  });

  function openPopoverFor(phrase, context) {
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
        elSaved.textContent = r.body.saved ? '✓ enregistré dans mes mots' : '✓ déjà dans mes mots';
        elResult.hidden = false;
      })
      .catch(function () {
        elLoading.hidden = true;
        elError.hidden = false;
        elError.textContent = 'Connexion impossible';
      });
  }

  elClose.addEventListener('click', closePopover);
})();
