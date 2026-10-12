// Pronunciation: a [data-say] button reads its word aloud, in its
// data-lang, with the device's own voices. Hidden where there are none.
(function () {
  'use strict';
  var ok = 'speechSynthesis' in window && 'SpeechSynthesisUtterance' in window;

  function hideAll() {
    if (ok) return;
    document.querySelectorAll('[data-say]').forEach(function (b) { b.hidden = true; });
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', hideAll);
  else hideAll();

  // A voice of the language when the device has one: some browsers
  // otherwise read with the interface language's voice.
  function voiceFor(lang) {
    var voices = speechSynthesis.getVoices(), best = null;
    for (var i = 0; i < voices.length; i++) {
      var v = voices[i];
      if (v.lang.toLowerCase().split(/[-_]/)[0] !== lang) continue;
      if (!best || (v.localService && !best.localService)) best = v;
    }
    return best;
  }

  window.lydiSay = function (text, lang) {
    if (!ok || !text) return;
    speechSynthesis.cancel();
    var u = new SpeechSynthesisUtterance(text);
    if (lang) {
      u.lang = lang;
      var v = voiceFor(lang.toLowerCase());
      if (v) u.voice = v;
    }
    u.rate = 0.9;
    speechSynthesis.speak(u);
  };
  if (ok) speechSynthesis.getVoices(); // starts loading them

  // Captured, so a button inside a <summary> doesn't open the row too.
  document.addEventListener('click', function (e) {
    var b = e.target.closest && e.target.closest('[data-say]');
    if (!b) return;
    e.preventDefault();
    e.stopPropagation();
    window.lydiSay(b.getAttribute('data-say'), b.getAttribute('data-lang'));
  }, true);
})();
