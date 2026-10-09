// The words page as a quiz: hide the translations (or the words) and tap
// a row to reveal it — a second tap opens the row as usual. The choice is
// remembered on this device.
(function () {
  'use strict';
  var bar = document.querySelector('.words-mask');
  if (!bar) return;
  var list = document.querySelector('main.page');
  var KEY = 'words-mask';
  function set(mode) {
    if (mode !== 'tr' && mode !== 'word') mode = '';
    if (mode) list.setAttribute('data-mask', mode); else list.removeAttribute('data-mask');
    bar.querySelectorAll('button').forEach(function (b) { b.setAttribute('aria-pressed', String(b.dataset.mask === mode)); });
    list.querySelectorAll('.word-row.revealed').forEach(function (r) { r.classList.remove('revealed'); });
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
