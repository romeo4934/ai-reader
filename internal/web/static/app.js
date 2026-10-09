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

  // --- reading mode ---
  // "pages": the section is laid out in screen-sized columns, turned by
  // tapping the edges (or swiping, or the arrow keys) — what an e-ink
  // screen needs, where scrolling smears. "scroll": the page scrolls.
  // Auto (no setting): pages on a tablet, scroll on a phone.
  var reading = chapterEl.dataset.reading;
  function isPaged() {
    if (reading === 'pages') return true;
    if (reading === 'scroll') return false;
    return Math.min(window.screen.width, window.screen.height) >= 600 && window.innerWidth >= 600;
  }
  var paged = isPaged();

  // On a phone, or turning pages, the translation opens in a panel along
  // the bottom (or top) edge instead of a bubble next to the word: there's
  // no room for a bubble, and a panel never hides the word.
  function useSheet() {
    return paged || window.innerWidth < 700;
  }

  // With the panel, a zone at the bottom of the screen is set aside for it
  // (the page nav lives there while it's closed), and the text stops above
  // it — so the panel, always in the same place, never covers the text.
  function dock() {
    document.documentElement.classList.toggle('reader-docked', useSheet());
  }
  dock();
  window.addEventListener('resize', dock);

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
  //
  // Picks whichever side (above/below the selection) actually has more
  // room, rather than always preferring below and only flipping when it
  // doesn't fit at all. "Room below" stops at the prev/next section bar
  // when that's on screen, not just the bottom of the viewport — the
  // popover is position:fixed and sits above everything, so without this
  // it would happily cover the nav buttons for the last word of a section
  // (reported live: word right above the nav, popover opened below it and
  // hid "précédent/suivant" underneath).
  function positionPopover(rect) {
    if (!rect) return;
    if (useSheet()) {
      // Docked: the panel covers the reserved zone at the bottom, which the
      // text never runs into — nothing to place, nothing hidden.
      popover.classList.add('sheet');
      popover.style.left = '';
      popover.style.top = '';
      return;
    }
    popover.classList.remove('sheet');
    var margin = 12;
    var w = popover.offsetWidth;
    var h = popover.offsetHeight;

    var left = rect.left + rect.width / 2 - w / 2;
    left = Math.max(margin, Math.min(left, window.innerWidth - w - margin));

    var bottomLimit = window.innerHeight - margin;
    var navEl = document.querySelector('.chapter-nav');
    if (navEl) {
      var navTop = navEl.getBoundingClientRect().top;
      if (navTop < bottomLimit) bottomLimit = navTop - margin;
    }

    var spaceBelow = bottomLimit - rect.bottom;
    var spaceAbove = rect.top - margin;
    var top = (spaceBelow >= h || spaceBelow >= spaceAbove) ? rect.bottom + margin : rect.top - margin - h;
    top = Math.max(margin, Math.min(top, bottomLimit - h));

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

      openPopoverFor(text, context, rect, originalOf(context, word, bounds));
    }, SETTLE_MS);
  });

  // Single tap/click on a word, no dragging required — native text selection
  // on mobile needs a double-tap or long-press, which is slower than this
  // app needs to be for "I don't know this one word". Only fires when there
  // isn't already a real selection in progress (selectionchange above owns
  // that case) so a drag-select still works for a whole clause.
  function wordAtPoint(x, y) {
    var pos = null;
    if (document.caretPositionFromPoint) {
      var p = document.caretPositionFromPoint(x, y);
      if (p) pos = { node: p.offsetNode, offset: p.offset };
    } else if (document.caretRangeFromPoint) {
      var r = document.caretRangeFromPoint(x, y);
      if (r) pos = { node: r.startContainer, offset: r.startOffset };
    }
    if (!pos || pos.node.nodeType !== Node.TEXT_NODE) return null;
    var paraEl = closestPara(pos.node);
    if (!paraEl) return null;
    var context = paraEl.textContent;
    var clickOffset = offsetInPara(paraEl, pos.node, pos.offset);
    return { paraEl: paraEl, context: context, offset: clickOffset };
  }

  chapterEl.addEventListener('click', function (e) {
    var sel = window.getSelection();
    if (sel && !sel.isCollapsed) return; // a drag-selection, not a tap

    var hit = wordAtPoint(e.clientX, e.clientY);
    if (!hit) return;
    var word = expandToWordBoundaries(hit.context, hit.offset, hit.offset);
    var text = hit.context.slice(word.start, word.end).trim();
    if (!text) return;

    var bounds = sentenceBounds(hit.context, word.start, word.end);
    highlightSentence(hit.paraEl, word.start, word.end, bounds);

    // highlightSentence just rebuilt the DOM around the tapped word, so the
    // simplest reliable way to its bounding box is to measure the mark it
    // just created rather than re-deriving a Range from flattened offsets.
    var markEl = hit.paraEl.querySelector('.word-highlight');
    var rect = markEl ? markEl.getBoundingClientRect() : hit.paraEl.getBoundingClientRect();

    openPopoverFor(text, hit.context, rect, originalOf(hit.context, word, bounds));
  });

  // The sentence as it is in the book, with the looked-up word in it — sent
  // with the request so the translation is of this sentence, not another
  // one of the paragraph holding the same word. (Not shown: it's already
  // highlighted in the page, which the panel never covers.)
  function originalOf(text, word, bounds) {
    return {
      sentence: text.slice(bounds.start, bounds.end).trim(),
      word: text.slice(word.start, word.end).trim(),
    };
  }

  var requestSeq = 0;

  function openPopoverFor(phrase, context, rect, original) {
    var myReq = ++requestSeq;
    var hint = document.getElementById('first-hint');
    if (hint) hint.remove(); // they've got it
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
        sentence: original ? original.sentence : '',
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

  // First opening of a book: jump past the front matter to the paragraph
  // where the story starts.
  function startParaEl() {
    var n = Number(chapterEl.dataset.startPara || 0);
    return n > 0 ? chapterEl.querySelectorAll('.para')[n] : null;
  }

  // --- pages ---
  if (paged) setupPages();
  else {
    var startEl = startParaEl();
    if (startEl) {
      var scroller = document.documentElement.classList.contains('reader-docked') ? chapterEl : document.scrollingElement;
      var top = startEl.getBoundingClientRect().top - (scroller === chapterEl ? chapterEl.getBoundingClientRect().top : 0);
      scroller.scrollTop += top - 8;
    }
  }

  function setupPages() {
    document.documentElement.classList.add('paged');
    var GAP = 48;
    var EDGE = 0.15; // a tap this close to the left/right edge turns the page
    var page = 0, pages = 1;
    var navEl = document.querySelector('.chapter-nav');
    var indicator = document.querySelector('.page-indicator');
    var baseIndicator = indicator ? indicator.textContent : '';
    var prevLink = document.querySelector('[data-step="prev"]');
    var nextLink = document.querySelector('[data-step="next"]');

    function stride() { return chapterEl.clientWidth + GAP; }

    // One column per screen: the column is as wide as the text area and as
    // tall as what's left between the header and the nav.
    function layout() {
      // Docked, the flex layout already gives the text its height (down to
      // the bottom zone); otherwise, size it to what's left on screen.
      if (document.documentElement.classList.contains('reader-docked')) {
        chapterEl.style.height = '';
      } else {
        var top = chapterEl.getBoundingClientRect().top;
        var navH = navEl ? navEl.offsetHeight + 24 : 24;
        chapterEl.style.height = Math.max(200, window.innerHeight - top - navH) + 'px';
      }
      chapterEl.style.columnWidth = chapterEl.clientWidth + 'px';
      chapterEl.style.columnGap = GAP + 'px';
      pages = Math.max(1, Math.round((chapterEl.scrollWidth + GAP) / stride()));
      go(Math.min(page, pages - 1));
    }

    function go(n) {
      page = n;
      chapterEl.scrollLeft = n * stride();
      if (indicator) indicator.textContent = baseIndicator + (pages > 1 ? ' · ' + (n + 1) + '/' + pages : '');
    }

    // Past the last page: the next section (or chapter); before the first:
    // the previous one, opened on its last page.
    function next() {
      closePopover();
      if (page < pages - 1) go(page + 1);
      else if (nextLink) window.location = nextLink.href;
    }
    function prev() {
      closePopover();
      if (page > 0) go(page - 1);
      else if (prevLink) window.location = prevLink.href + '&pg=last';
    }

    if (nextLink) nextLink.addEventListener('click', function (e) { e.preventDefault(); next(); });
    if (prevLink) prevLink.addEventListener('click', function (e) { e.preventDefault(); prev(); });

    // Edge taps turn the page; taps anywhere else still translate (the
    // word-tap handler runs after this one, which stops it for edge taps).
    // Judged by position, not target: past the end of a short last page
    // there's no text under the finger, only the page itself.
    document.addEventListener('click', function (e) {
      if (popover.contains(e.target) || (navEl && navEl.contains(e.target))) return;
      var r = chapterEl.getBoundingClientRect();
      if (e.clientY < r.top || e.clientY > r.bottom) return;
      var x = (e.clientX - r.left) / r.width;
      if (x < EDGE) { e.stopPropagation(); e.preventDefault(); prev(); }
      else if (x > 1 - EDGE) { e.stopPropagation(); e.preventDefault(); next(); }
    }, true);

    var touch = null;
    chapterEl.addEventListener('touchstart', function (e) {
      var t = e.changedTouches[0];
      touch = { x: t.clientX, y: t.clientY, at: Date.now() };
    }, { passive: true });
    chapterEl.addEventListener('touchend', function (e) {
      if (!touch) return;
      var t = e.changedTouches[0];
      var dx = t.clientX - touch.x, dy = t.clientY - touch.y;
      var quick = Date.now() - touch.at < 600;
      touch = null;
      var sel = window.getSelection();
      if (!quick || Math.abs(dx) < 50 || Math.abs(dy) > 40 || (sel && !sel.isCollapsed)) return;
      if (dx < 0) next(); else prev();
    }, { passive: true });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowRight' || e.key === 'PageDown' || e.key === ' ') { e.preventDefault(); next(); }
      else if (e.key === 'ArrowLeft' || e.key === 'PageUp') { e.preventDefault(); prev(); }
    });

    var resizeTimer = null;
    window.addEventListener('resize', function () {
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(layout, 150);
    });

    layout();
    if (/[?&]pg=last\b/.test(window.location.search)) go(pages - 1);
    var startEl = startParaEl();
    if (startEl) go(Math.min(pages - 1, Math.floor((startEl.getBoundingClientRect().left - chapterEl.getBoundingClientRect().left) / stride())));
  }
})();
