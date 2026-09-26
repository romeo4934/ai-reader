// Selection -> translate -> save, on the reading view. No-op on any other page.
(function () {
  'use strict';
  var chapterEl = document.getElementById('chapter');
  if (!chapterEl) return;

  var popover = document.getElementById('translate-popover');
  var elLoading = document.getElementById('tp-loading');
  var elError = document.getElementById('tp-error');
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
    elSentence.hidden = true;
  }

  // Renders `sentence` as text, with the exact substring `highlight` (if it
  // actually occurs in it) wrapped for emphasis — the word's translation
  // shown in place instead of as a separate line.
  function renderSentence(el, sentence, highlight) {
    if (!highlight) {
      el.textContent = sentence;
      return;
    }
    var idx = sentence.indexOf(highlight);
    if (idx < 0) {
      el.textContent = sentence;
      return;
    }
    el.innerHTML =
      escapeHtml(sentence.slice(0, idx)) +
      '<mark class="word-highlight">' + escapeHtml(sentence.slice(idx, idx + highlight.length)) + '</mark>' +
      escapeHtml(sentence.slice(idx + highlight.length));
  }

  // Positions the popover near the selection rather than pinned to the
  // bottom of the screen, so it doesn't end up far from what it's about.
  // Called twice: once on open (loading state) and again once the result
  // renders, since the taller content can push it past a viewport edge.
  function positionPopover(rect) {
    if (!rect) return;
    var margin = 12;
    var w = popover.offsetWidth;
    var h = popover.offsetHeight;

    var left = rect.left + rect.width / 2 - w / 2;
    left = Math.max(margin, Math.min(left, window.innerWidth - w - margin));

    var top = rect.bottom + margin;
    if (top + h > window.innerHeight - margin) {
      top = rect.top - h - margin; // no room below — flip above the selection
    }
    top = Math.max(margin, top);

    popover.style.left = left + 'px';
    popover.style.top = top + 'px';
  }

  function closestPara(node) {
    var el = node.nodeType === 1 ? node : node.parentElement;
    while (el && !el.classList.contains('para')) el = el.parentElement;
    return el;
  }

  // --- highlight both the selected word and the sentence it sits in ---
  // The reply shows a translated sentence as well as the word; without a
  // visual anchor for each there's no way to tell which is which in the text.

  var highlightedPara = null;

  function escapeHtml(s) {
    return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
  }

  function clearHighlight() {
    if (!highlightedPara) return;
    highlightedPara.textContent = highlightedPara.textContent; // drops the <mark>s, keeps the text
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

  // Double-click/double-tap word selection stops at hyphens (native browser
  // behavior), so clicking "arms" in "man-at-arms" only selects "arms". Widen
  // any selection edge that lands mid-word out to the full hyphenated token,
  // since that's the actual lexical unit worth translating. Harmless for
  // ordinary multi-word selections too, since a space always stops it.
  var WORD_CHAR = /[\p{L}\p{N}'-]/u;

  function expandToWordBoundaries(text, start, end) {
    while (start > 0 && WORD_CHAR.test(text[start - 1])) start--;
    while (end < text.length && WORD_CHAR.test(text[end])) end++;
    return { start: start, end: end };
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

  // Wraps the sentence in one color and the word inside it in another,
  // nested — the sentence mark still shows either side of the word.
  function highlightSentence(paraEl, wordStart, wordEnd, bounds) {
    var text = paraEl.textContent;
    clearHighlight();
    paraEl.innerHTML =
      escapeHtml(text.slice(0, bounds.start)) +
      '<mark class="sentence-highlight">' +
        escapeHtml(text.slice(bounds.start, wordStart)) +
        '<mark class="word-highlight">' + escapeHtml(text.slice(wordStart, wordEnd)) + '</mark>' +
        escapeHtml(text.slice(wordEnd, bounds.end)) +
      '</mark>' +
      escapeHtml(text.slice(bounds.end));
    highlightedPara = paraEl;
  }

  // selectionchange fires continuously while a selection is being dragged —
  // once per character on a touch drag, or on every handle nudge on iPad.
  // Debounce so a translate call (and the auto-save it triggers) only
  // happens once the selection has actually settled, not once per fragment.
  var settleTimer = null;
  var SETTLE_MS = 300;

  document.addEventListener('selectionchange', function () {
    if (settleTimer) clearTimeout(settleTimer);
    settleTimer = setTimeout(function () {
      settleTimer = null;
      var sel = window.getSelection();
      if (!sel || sel.isCollapsed || sel.rangeCount === 0) return;
      if (!chapterEl.contains(sel.anchorNode)) return;

      var range = sel.getRangeAt(0);
      var paraEl = closestPara(range.startContainer);
      if (!paraEl) return;

      var rect = range.getBoundingClientRect();
      var context = paraEl.textContent;
      var rawStart = offsetInPara(paraEl, range.startContainer, range.startOffset);
      var rawEnd = offsetInPara(paraEl, range.endContainer, range.endOffset);
      var word = expandToWordBoundaries(context, rawStart, rawEnd);
      var text = context.slice(word.start, word.end).trim();
      if (!text) return;
      var bounds = sentenceBounds(context, word.start, word.end);
      highlightSentence(paraEl, word.start, word.end, bounds);

      openPopoverFor(text, context, rect);
    }, SETTLE_MS);
  });

  var requestSeq = 0;

  function openPopoverFor(phrase, context, rect) {
    var myReq = ++requestSeq;
    resetPopoverBody();
    popover.hidden = false;
    elLoading.hidden = false;
    positionPopover(rect);

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
          positionPopover(rect);
          return;
        }
        renderSentence(elSentence, r.body.sentence_translation || '', r.body.sentence_translation_highlight);
        elSentence.hidden = !r.body.sentence_translation;
        positionPopover(rect); // content grew — re-clamp to the viewport
      })
      .catch(function () {
        if (myReq !== requestSeq) return;
        elLoading.hidden = true;
        elError.hidden = false;
        elError.textContent = 'Connexion impossible';
        positionPopover(rect);
      });
  }

  elClose.addEventListener('click', closePopover);
})();
