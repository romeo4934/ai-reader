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
  var elClose = document.getElementById('tp-close');

  var bookID = chapterEl.dataset.bookId;
  var chapterID = chapterEl.dataset.chapterId;

  function closePopover() {
    popover.hidden = true;
    window.getSelection().removeAllRanges();
    clearHighlight();
  }

  function resetPopoverBody() {
    elLoading.hidden = true;
    elError.hidden = true;
    elResult.hidden = true;
  }

  function closestPara(node) {
    var el = node.nodeType === 1 ? node : node.parentElement;
    while (el && !el.classList.contains('para')) el = el.parentElement;
    return el;
  }

  // --- highlight the sentence being translated, not just the selected word ---
  // The reply already shows a translated sentence; without this there's no
  // way to tell which sentence in the book it corresponds to.

  var highlightedPara = null;

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function clearHighlight() {
    if (!highlightedPara) return;
    highlightedPara.textContent = highlightedPara.textContent; // drops the <mark>, keeps the text
    highlightedPara = null;
  }

  // Character offset of a range's boundary within paraEl's full text, however
  // many text nodes the paragraph is made of.
  function offsetInPara(paraEl, container, offset) {
    var r = document.createRange();
    r.selectNodeContents(paraEl);
    r.setEnd(container, offset);
    return r.toString().length;
  }

  // Naive sentence-boundary heuristic: walk out to the enclosing ". ! ?" on
  // each side. Good enough for narrative prose; the odd abbreviation will
  // occasionally over- or under-shoot, which is a fine trade for no NLP
  // dependency in a personal reading app.
  function sentenceBounds(text, start, end) {
    var enders = /[.!?]/;
    var s = start;
    while (s > 0 && !enders.test(text[s - 1])) s--;
    while (s < text.length && /[\s"'“‘]/.test(text[s])) s++;
    var e = end;
    while (e < text.length && !enders.test(text[e])) e++;
    if (e < text.length) e++; // include the punctuation
    while (e < text.length && /["'”’]/.test(text[e])) e++;
    return { start: s, end: e };
  }

  function highlightSentence(paraEl, range) {
    var text = paraEl.textContent;
    var start = offsetInPara(paraEl, range.startContainer, range.startOffset);
    var end = offsetInPara(paraEl, range.endContainer, range.endOffset);
    var bounds = sentenceBounds(text, start, end);

    clearHighlight();
    paraEl.innerHTML =
      escapeHtml(text.slice(0, bounds.start)) +
      '<mark class="sentence-highlight">' + escapeHtml(text.slice(bounds.start, bounds.end)) + '</mark>' +
      escapeHtml(text.slice(bounds.end));
    highlightedPara = paraEl;
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

      var range = sel.getRangeAt(0);
      var paraEl = closestPara(range.startContainer);
      if (!paraEl) return;
      var context = paraEl.textContent;
      highlightSentence(paraEl, range);

      openPopoverFor(text, context);
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
