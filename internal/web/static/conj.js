// The conjugation exercise: the series is in window.conjData; each answer
// is checked here and recorded on the server (missed forms come back).
(function () {
  var data = window.conjData;
  if (!data || !data.questions || !data.questions.length) return;
  var qs = data.questions, i = 0, right = 0, missed = [], typedLast = '';
  function $(id) { return document.getElementById(id); }

  function norm(s) {
    return (s || '').toLowerCase().replace(/[’`´]/g, "'").replace(/\s+/g, ' ').replace(/[.!?,;]+$/, '').trim();
  }
  function bare(s) { return s.normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss'); }

  // What was typed, without the pronoun shown before the blank, if typed
  // too ("je mange", "que tu sois", "j'ai").
  function stripPronoun(t, pronoun) {
    t = t.replace(/^(que |qu'|che )/, '');
    var p = norm(pronoun).replace(/^(que |qu'|che )/, '');
    var alts = p ? p.split('/') : [];
    for (var k = 0; k < alts.length; k++) {
      var a = alts[k].trim();
      if (a && t.indexOf(a + ' ') === 0) return t.slice(a.length + 1);
    }
    if (data.lang === 'fr' && t.indexOf("j'") === 0) return t.slice(2);
    return t;
  }

  function grade(typed, q) {
    var t = stripPronoun(norm(typed), q.pronoun);
    if (!t) return null;
    var close = false;
    for (var k = 0; k < q.answers.length; k++) {
      var a = norm(q.answers[k]);
      if (t === a) return 'ok';
      if (bare(t) === bare(a)) close = true;
    }
    return close ? 'accents' : 'wrong';
  }

  function show(id, on) { $(id).hidden = !on; }

  function render() {
    var q = qs[i];
    $('conj-step').textContent = (i + 1) + ' / ' + qs.length;
    show('conj-again', q.again);
    $('conj-verb').textContent = q.verb;
    $('conj-say-verb').dataset.say = q.verb;
    $('conj-meaning').textContent = q.meaning || '';
    $('conj-tense').textContent = q.tenseName;
    $('conj-gloss').textContent = q.tenseGloss ? '(' + q.tenseGloss + ')' : '';
    $('conj-pronoun').textContent = q.pronoun;
    show('conj-pronoun', !!q.pronoun);
    var input = $('conj-typed');
    input.value = '';
    input.disabled = false;
    show('conj-form', true);
    show('conj-skip', true);
    show('conj-result', false);
    input.focus();
  }

  function answer(typed) {
    var q = qs[i];
    var g = typed === null ? 'wrong' : grade(typed, q);
    if (!g) { $('conj-typed').focus(); return; }
    typedLast = typed || '';
    show('conj-form', false);
    show('conj-skip', false);
    show('conj-result', true);
    show('conj-ok', g === 'ok');
    show('conj-accents', g === 'accents');
    show('conj-wrong', g === 'wrong');
    $('conj-accents-typed').textContent = typedLast;
    $('conj-wrong-typed').textContent = typedLast;
    $('conj-feedback').classList.toggle('type-feedback-ok', g === 'ok');
    $('conj-full').textContent = q.full;
    $('conj-say').dataset.say = q.full;
    var why = $('conj-why');
    why.hidden = !(g === 'wrong' && typedLast);
    why.disabled = false;
    show('conj-explain', false);

    var book = $('conj-book');
    book.hidden = !q.sentence;
    if (q.sentence) {
      $('conj-book-label').textContent = $('conj-book-label').dataset.label.replace('%s', q.book || '');
      var out = $('conj-sentence');
      out.textContent = '';
      var at = q.phrase ? q.sentence.toLowerCase().indexOf(q.phrase.toLowerCase()) : -1;
      if (at < 0) {
        out.textContent = q.sentence;
      } else {
        var mark = document.createElement('mark');
        mark.textContent = q.sentence.slice(at, at + q.phrase.length);
        out.append(q.sentence.slice(0, at), mark, q.sentence.slice(at + q.phrase.length));
      }
    }

    if (g === 'ok') right++; else missed.push(q.full);
    fetch('/conjugate/answer', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ lang: data.lang, verb: q.verb, tense: q.tense, person: q.person, ok: g === 'ok' })
    }).catch(function () {});
    $('conj-next').focus();
  }

  $('conj-form').addEventListener('submit', function (e) {
    e.preventDefault();
    answer($('conj-typed').value);
  });
  $('conj-skip').addEventListener('click', function () { answer(null); });

  $('conj-why').addEventListener('click', function () {
    var why = this, out = $('conj-explain'), q = qs[i];
    why.disabled = true;
    out.hidden = false;
    out.textContent = why.dataset.loading;
    fetch('/conjugate/explain', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ lang: data.lang, verb: q.verb, tense: q.tense, person: q.person, typed: typedLast, answer: q.answers[0] })
    }).then(function (res) {
      return res.json().then(function (d) {
        if (!res.ok) throw new Error(d.error || why.dataset.failed);
        return d;
      });
    }).then(function (d) {
      out.textContent = d.explanation;
      why.hidden = true;
    }).catch(function (err) {
      out.textContent = err.message || why.dataset.failed;
      why.disabled = false;
    });
  });

  $('conj-next').addEventListener('click', function () {
    i++;
    if (i < qs.length) { render(); return; }
    show('conj', false);
    show('conj-done', true);
    var score = $('conj-done-score');
    score.textContent = score.dataset.label.replace('%d', right).replace('%d', qs.length);
    show('conj-all-right', missed.length === 0);
    show('conj-missed', missed.length > 0);
    var list = $('conj-missed-list');
    missed.forEach(function (m) {
      var li = document.createElement('li');
      li.textContent = m;
      list.appendChild(li);
    });
    $('conj-done').querySelector('a').focus();
  });

  render();
})();
