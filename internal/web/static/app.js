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
  //
  // Picks whichever side (above/below the selection) actually has more
  // room, rather than always preferring below and only flipping when it
  // doesn't fit at all — a word near the bottom of the (now fixed-height,
  // paginated) reading area would otherwise flip to "above" even when
  // there isn't really room there either, landing the popover right on
  // top of the sentence it's explaining. The popover's own max-height is
  // also capped to whatever room the chosen side actually has, so it
  // scrolls internally instead of spilling over the selection.
  function positionPopover(rect) {
    if (!rect) return;
    var margin = 12;
    var w = popover.offsetWidth;

    var left = rect.left + rect.width / 2 - w / 2;
    left = Math.max(margin, Math.min(left, window.innerWidth - w - margin));
    popover.style.left = left + 'px';

    var spaceBelow = window.innerHeight - margin - rect.bottom;
    var spaceAbove = rect.top - margin;
    var below = spaceBelow >= spaceAbove;
    var space = Math.max(80, below ? spaceBelow : spaceAbove);
    popover.style.maxHeight = Math.min(space, window.innerHeight * 0.7) + 'px';

    var h = popover.offsetHeight; // re-measure: maxHeight may have just clamped it
    var top = below ? rect.bottom + margin : rect.top - margin - h;
    top = Math.max(margin, Math.min(top, window.innerHeight - margin - h));
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

    openPopoverFor(text, hit.context, rect);
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

  // --- pagination, liseuse-style ---------------------------------------
  // #chapter is laid out as CSS columns exactly as wide as the viewport,
  // so the browser does the hard part (breaking paragraphs across pages);
  // "turning a page" is just translateX-ing by one column width. Crossing
  // a chapter boundary falls through to the real prev/next-chapter link
  // (or, if there is none, the button simply has nowhere further to go).

  var viewport = document.getElementById('chapter-viewport');
  var pagePrev = document.getElementById('page-prev');
  var pageNext = document.getElementById('page-next');
  var pageIndicator = document.getElementById('page-indicator');

  if (viewport) {
    var PAGE_GAP = 48;
    var currentPage = 0;
    var totalPages = 1;
    var pageStep = 0;

    function setTopnavHeight() {
      var topnav = document.querySelector('.topnav');
      var h = topnav ? Math.round(topnav.getBoundingClientRect().height) : 0;
      if (h > 0) document.documentElement.style.setProperty('--topnav-h', h + 'px');
    }

    function layoutPages() {
      var w = viewport.clientWidth;
      chapterEl.style.width = w + 'px';
      chapterEl.style.columnWidth = w + 'px';
      chapterEl.style.columnGap = PAGE_GAP + 'px';
      pageStep = w + PAGE_GAP;
      totalPages = Math.max(1, Math.round((chapterEl.scrollWidth + PAGE_GAP) / pageStep));
    }

    function updateControls() {
      if (pageIndicator) pageIndicator.textContent = (currentPage + 1) + ' / ' + totalPages;
      if (pagePrev) pagePrev.classList.toggle('page-edge', currentPage === 0 && pagePrev.tagName === 'BUTTON');
      if (pageNext) pageNext.classList.toggle('page-edge', currentPage === totalPages - 1 && pageNext.tagName === 'BUTTON');
    }

    function showPage(n, animate) {
      currentPage = Math.max(0, Math.min(n, totalPages - 1));
      chapterEl.style.transition = animate === false ? 'none' : '';
      chapterEl.style.transform = 'translateX(' + (-currentPage * pageStep) + 'px)';
      updateControls();
    }

    function goNext() {
      if (currentPage < totalPages - 1) showPage(currentPage + 1);
      else if (pageNext && pageNext.tagName === 'A') window.location.href = pageNext.href;
    }

    function goPrev() {
      if (currentPage > 0) showPage(currentPage - 1);
      else if (pagePrev && pagePrev.tagName === 'A') window.location.href = pagePrev.href;
    }

    if (pageNext) pageNext.addEventListener('click', function (e) {
      if (currentPage < totalPages - 1) { e.preventDefault(); showPage(currentPage + 1); }
    });
    if (pagePrev) pagePrev.addEventListener('click', function (e) {
      if (currentPage > 0) { e.preventDefault(); showPage(currentPage - 1); }
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'ArrowRight') goNext();
      else if (e.key === 'ArrowLeft') goPrev();
    });

    // Horizontal swipe to turn pages — the main interaction on iPad. Bails
    // out whenever a text selection is active so it never fights the
    // translate-on-select gesture above.
    var touchStartX = null, touchStartY = null, touchStartT = 0;
    viewport.addEventListener('touchstart', function (e) {
      if (e.touches.length !== 1) return;
      touchStartX = e.touches[0].clientX;
      touchStartY = e.touches[0].clientY;
      touchStartT = Date.now();
    }, { passive: true });
    viewport.addEventListener('touchend', function (e) {
      if (touchStartX === null) return;
      var t = e.changedTouches[0];
      var dx = t.clientX - touchStartX;
      var dy = t.clientY - touchStartY;
      var dt = Date.now() - touchStartT;
      touchStartX = null;
      var sel = window.getSelection();
      if (sel && !sel.isCollapsed) return;
      if (dt > 600 || Math.abs(dx) < 60 || Math.abs(dx) < Math.abs(dy) * 1.5) return;
      if (dx < 0) goNext(); else goPrev();
    }, { passive: true });

    var resizeTimer = null;
    window.addEventListener('resize', function () {
      if (resizeTimer) clearTimeout(resizeTimer);
      resizeTimer = setTimeout(function () {
        var frac = totalPages > 1 ? currentPage / (totalPages - 1) : 0;
        setTopnavHeight();
        layoutPages();
        showPage(Math.round(frac * (totalPages - 1)), false);
      }, 150);
    });

    setTopnavHeight();
    layoutPages();
    var enterEnd = /[?&]enter=end\b/.test(window.location.search);
    showPage(enterEnd ? totalPages - 1 : 0, false);
  }
})();
