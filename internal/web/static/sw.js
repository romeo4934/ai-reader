// Lydi's service worker: what makes it installable as an app, and a page
// that says so when there is no connection (Lydi needs the network to
// translate, so nothing else is cached — pages are always fresh).
var CACHE = 'lydi-offline-v1';
var OFFLINE = '/static/offline.html';

self.addEventListener('install', function (e) {
  e.waitUntil(caches.open(CACHE).then(function (c) { return c.add(OFFLINE); }));
  self.skipWaiting();
});

self.addEventListener('activate', function (e) {
  e.waitUntil(caches.keys().then(function (keys) {
    return Promise.all(keys.filter(function (k) { return k !== CACHE; }).map(function (k) { return caches.delete(k); }));
  }).then(function () { return self.clients.claim(); }));
});

self.addEventListener('fetch', function (e) {
  if (e.request.mode !== 'navigate' || e.request.method !== 'GET') return;
  e.respondWith(fetch(e.request).catch(function () { return caches.match(OFFLINE); }));
});
