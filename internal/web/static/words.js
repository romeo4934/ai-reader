// The words page as a quiz: hide the translations (or the words) and tap
// a row to reveal it — a second tap opens the row as usual. The choice is
// remembered on this device. While quizzing, the words of each level come
// in a new random order every time, so the order itself isn't learned.
(function () {
  'use strict';
  var bar = document.querySelector('.words-mask');
  if (!bar) return;
  var list = document.querySelector('main.page');
  var KEY = 'words-mask';
  var lists = Array.prototype.slice.call(list.querySelectorAll('.word-list'));
  var original = lists.map(function (ul) { return Array.prototype.slice.call(ul.children); });
  function order(shuffle) {
    lists.forEach(function (ul, i) {
      var items = original[i].slice();
      for (var j = items.length - 1; shuffle && j > 0; j--) {
        var k = Math.floor(Math.random() * (j + 1)), x = items[j];
        items[j] = items[k]; items[k] = x;
      }
      items.forEach(function (li) { ul.appendChild(li); });
    });
  }
  function set(mode) {
    if (mode !== 'tr' && mode !== 'word') mode = '';
    if (mode) list.setAttribute('data-mask', mode); else list.removeAttribute('data-mask');
    bar.querySelectorAll('button').forEach(function (b) { b.setAttribute('aria-pressed', String(b.dataset.mask === mode)); });
    list.querySelectorAll('.word-row.revealed').forEach(function (r) { r.classList.remove('revealed'); });
    list.querySelectorAll('.word-list details[open]').forEach(function (d) { d.open = false; });
    order(!!mode);
    try { localStorage.setItem(KEY, mode); } catch (e) {}
  }
  var saved = '';
  try { saved = localStorage.getItem(KEY) || ''; } catch (e) {}
  set(saved);
  bar.addEventListener('click', function (e) {
    var b = e.target.closest('button');
    if (b) set(b.dataset.mask);
  });
  list.addEventListener('click', function (e) {
    var row = e.target.closest('.word-row');
    if (!row || !list.hasAttribute('data-mask') || row.classList.contains('revealed')) return;
    e.preventDefault(); // reveal first, open on the next tap
    row.classList.add('revealed');
  });
})();
