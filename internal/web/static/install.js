// The "install the app" card (partial "install"): shows the one way that
// works on this device — the browser's own install, kept by the head
// script; the share-sheet steps on iPhone and iPad, which have no install
// button for web apps; or where the browser's menu has it. In the
// installed app it says so (settings) or stays hidden (library banner),
// as does a banner the reader closed.
(function () {
  'use strict';
  var cards = document.querySelectorAll('.install-card');
  if (!cards.length) return;
  var ua = navigator.userAgent;
  var ios = /iPhone|iPad|iPod/.test(ua) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1);
  var mobile = ios || /Android/.test(ua);
  var installed = (window.matchMedia && window.matchMedia('(display-mode: standalone)').matches) || navigator.standalone === true;
  var dismissed = false;
  try { dismissed = !!localStorage.getItem('install-dismissed'); } catch (e) {}

  function state() {
    if (installed) return 'installed';
    if (window.lydiInstallPrompt) return 'native';
    if (ios) return 'ios';
    return 'other';
  }
  function render() {
    var st = state();
    cards.forEach(function (c) {
      var banner = c.dataset.mode === 'banner';
      // The library banner: only on phones and tablets, until the app is
      // installed or the reader closes it.
      if (banner && (st === 'installed' || dismissed || !mobile)) { c.hidden = true; return; }
      c.querySelectorAll('[data-step]').forEach(function (el) { el.hidden = el.dataset.step !== st; });
      c.hidden = false;
    });
  }

  document.addEventListener('lydi-installable', render);
  window.addEventListener('appinstalled', function () { installed = true; window.lydiInstallPrompt = null; render(); });
  document.querySelectorAll('[data-install-go]').forEach(function (b) {
    b.addEventListener('click', function () {
      var p = window.lydiInstallPrompt;
      if (!p) return;
      p.prompt();
      p.userChoice.then(function (choice) {
        if (choice.outcome === 'accepted') installed = true;
        window.lydiInstallPrompt = null; // a prompt can be used only once
        render();
      });
    });
  });
  document.querySelectorAll('[data-install-close]').forEach(function (b) {
    b.addEventListener('click', function () {
      dismissed = true;
      try { localStorage.setItem('install-dismissed', '1'); } catch (e) {}
      render();
    });
  });
  render();
})();
