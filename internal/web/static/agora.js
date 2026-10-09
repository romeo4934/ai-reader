// The Lydi Agora as a little top-down 2D game:
// the walled agora with its temple, the speakers' platform and stone tiers
// where members sit in order of prestige, and outside the market, the
// meadow and the square where aspirants wait at the gate. The viewer walks
// wherever they tap. The scenery is drawn in code at the screen's full
// resolution, once per size into an offscreen canvas; the readers are LPC
// pixel-art sprites (static/lpc.js) dressed as each one chose, walking,
// standing or seated on the tiers. Day, dusk and night follow
// the reader's clock. On e-ink (or with reduced motion) it's a still frame.
(function () {
  'use strict';
  var canvas = document.getElementById('agora-game');
  if (!canvas) return;
  var people = JSON.parse(document.getElementById('agora-people').textContent || '[]');
  var txt = canvas.dataset;
  var W = 480, H = 400; // world size
  var still = document.documentElement.hasAttribute('data-eink') ||
    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);

  var view = canvas.getContext('2d');
  var g = view;
  var bg = document.createElement('canvas'); // the scenery, at screen resolution
  [view, bg.getContext('2d')].forEach(function (c) {
    if (!c.roundRect) c.roundRect = function (x, y, w, h) { this.rect(x, y, w, h); }; // older browsers
  });

  // --- layout (world units) ---
  var WALL = { x0: 60, y0: 40, x1: 420, y1: 300, t: 8 };
  var GATE = { x0: 214, x1: 266 };
  var TEMPLE = { x0: 168, y0: 50, x1: 312, y1: 100 };
  var BEMA = { x: 240, y: 122 };
  var TIERS = { cx: 240, cy: 118, r: [44, 64, 84, 104, 124, 144], from: 0.26, to: Math.PI - 0.26 };
  var STATUE = { x: 100, y: 262 }, FOUNTAIN = { x: 378, y: 258, r: 20 };
  var BRAZIERS = [{ x: 198, y: 282 }, { x: 282, y: 282 }, { x: 150, y: 108 }, { x: 330, y: 108 }];
  var treesOut = [[22, 372], [458, 372], [30, 316], [450, 318]];
  var obstaclesOut = treesOut.map(function (t) { return { x: t[0], y: t[1], r: 15 }; })
    .concat([{ x: 290, y: 352, r: 8 }, { x: 128, y: 386, r: 8 }, { x: 362, y: 388, r: 8 }]);
  var obstaclesIn = [{ x: STATUE.x, y: STATUE.y, r: 14 }, { x: FOUNTAIN.x, y: FOUNTAIN.y, r: 25 },
    { x: BEMA.x, y: BEMA.y, r: 12 }].concat(BRAZIERS.map(function (b) { return { x: b.x, y: b.y, r: 8 }; }));

  function inside(p) { return p.x > WALL.x0 && p.x < WALL.x1 && p.y > WALL.y0 && p.y < WALL.y1; }
  function blocked(p, list) {
    for (var i = 0; i < list.length; i++) if (Math.hypot(p.x - list[i].x, p.y - list[i].y) < list[i].r) return true;
    return false;
  }
  function walkable(p, isIn) {
    if (isIn) return p.x > 74 && p.x < 406 && p.y > 112 && p.y < 290 && !blocked(p, obstaclesIn);
    return p.x > 8 && p.x < 472 && p.y > 318 && p.y < 392 && !blocked(p, obstaclesOut);
  }
  function randomSpot(isIn) {
    for (var i = 0; i < 80; i++) {
      var p = { x: 8 + Math.random() * 464, y: 112 + Math.random() * 280 };
      if (walkable(p, isIn)) return p;
    }
    return { x: 240, y: isIn ? 270 : 350 };
  }

  // --- seats: tiers by prestige, the most assiduous in the middle ---
  // Each tier belongs to a rank (front row: Sages' thrones, then the
  // Philosophers, two rows of Orators, two of Citizens): seats show even
  // when empty, so everyone sees the places waiting for them.
  var TIER_RANK = [3, 2, 1, 1, 0, 0];
  // Stairs cut the hemicycle into sections, like a Greek theatre.
  var AISLES = [Math.PI / 2 - 0.62, Math.PI / 2 + 0.62];
  function inAisle(a) { return AISLES.some(function (x) { return Math.abs(a - x) < 0.075; }); }
  function makeSeats() {
    var out = [];
    TIERS.r.forEach(function (r, tier) {
      var n = Math.floor((TIERS.to - TIERS.from) * r / 17), row = [];
      for (var k = 0; k < n; k++) {
        var a = TIERS.from + (TIERS.to - TIERS.from) * (k + 0.5) / n;
        if (inAisle(a)) continue;
        row.push({ x: TIERS.cx + Math.cos(a) * r, y: TIERS.cy + Math.sin(a) * r, a: a, r: r, tier: tier, rank: TIER_RANK[tier], mid: Math.abs(a - Math.PI / 2) });
      }
      row.sort(function (a, b) { return a.mid - b.mid; });
      out = out.concat(row);
    });
    return out;
  }
  var seatList = makeSeats();
  // Seated, everyone turns toward the stage: in profile on the side
  // sections, from behind in the middle one.
  function seatDir(st) {
    var dx = TIERS.cx - st.x, dy = TIERS.cy - st.y;
    return Math.abs(dx) > Math.abs(dy) * 0.8 ? (dx > 0 ? 'right' : 'left') : 'up';
  }

  // --- characters ---
  var actors = [];
  // A seat in one's own rank's tiers, else further back, else anywhere.
  function takeSeat(rank) {
    var free = seatList.filter(function (s) { return !s.taken; });
    var seat = free.filter(function (s) { return s.rank === rank; })[0] ||
      free.filter(function (s) { return s.rank < rank; })[0] || free[0];
    if (seat) seat.taken = true;
    return seat;
  }
  people.filter(function (p) { return p.member; })
    .sort(function (a, b) { return b.days - a.days; })
    .forEach(function (p) {
      var seat = takeSeat(rankTier(p.days));
      if (seat) seat.dir = seatDir(seat);
      actors.push({ p: p, inside: true, seat: seat, sitting: !!seat, path: [], wait: 3 + Math.random() * 6,
        phase: Math.random() * 6, pos: seat ? { x: seat.x, y: seat.y } : randomSpot(true) });
    });
  people.filter(function (p) { return !p.member; }).forEach(function (p) {
    actors.push({ p: p, inside: false, path: [], wait: Math.random() * 3, phase: Math.random() * 6,
      speed: 16 + Math.random() * 12, pos: randomSpot(false) });
  });
  var me = actors.filter(function (a) { return a.p.you; })[0];
  if (me && !me.p.member) me.pos = { x: 240, y: 340 };
  var guard = { pos: { x: 202, y: 318 }, guard: true, bubble: null };
  var orator = actors.filter(function (a) { return a.p.member && a !== me; })[0];

  // Each character's sprites, built from their look (a Sage who chose no
  // headwear still wears the laurel; the guard is a legionary).
  function specOf(a) {
    if (a.guard) return { body: 'male', skin: 'light', hair: 'plain', hairColor: 'dark_brown', guard: true };
    var s = {}, L = a.p.look || {};
    for (var k in L) s[k] = L[k];
    if (a.p.laurel && !s.head) s.head = 'laurel';
    return s;
  }
  if (window.LPC) {
    LPC.ready().then(function () {
      actors.concat([guard]).forEach(function (a) {
        LPC.build(specOf(a)).then(function (sh) { a.sheets = sh; if (still) frame(performance.now()); });
      });
    });
  }

  // Ambient life.
  var pigeons = [], sheep = [], butterflies = [];
  for (var i = 0; i < 6; i++) {
    var home = { x: 150 + Math.random() * 180, y: 272 + Math.random() * 14 };
    pigeons.push({ home: home, pos: { x: home.x, y: home.y }, fly: 0 });
  }
  for (i = 0; i < 4; i++) sheep.push({ pos: { x: 28, y: 100 + i * 40 }, t: i * 1.7 });
  for (i = 0; i < 4; i++) butterflies.push({ x: Math.random() * W, y: 324 + Math.random() * 60, s: Math.random() * 10, c: ['#f7d14c', '#ffffff', '#f29fc5', '#9fd1f2'][i] });

  function say(who, text, secs) { who.bubble = { text: text, until: performance.now() + (secs || 3) * 1000 }; }

  // Through the gate when crossing the wall.
  function route(a, target) {
    var tIn = inside(target), path = [];
    if (a.inside && !tIn) path.push({ x: 240, y: 290 }, { x: 240, y: 324 });
    if (!a.inside && tIn) path.push({ x: 240, y: 324 }, { x: 240, y: 290 });
    path.push(target);
    a.path = path;
    a.sitting = false;
  }

  // --- camera ---
  // The map covers the whole canvas (scaled to fill it, cropping the longer
  // side); the camera follows the reader's character, Zelda-style, and a
  // drag pans it to look around until the next tap.
  var cam = { x: 0, y: 0, scale: 1, base: 1, zoom: 1, follow: true };
  var bgScale = 1; // the scale the scenery was last drawn at
  var ZOOM_MAX = 3;
  cam.zoomMin = 1; // set on resize: the whole map fits
  function viewSize() { return { w: canvas.width / cam.scale, h: canvas.height / cam.scale }; }
  // Zoomed out past the map's edge on one side, it's centered on that side.
  function clampCam() {
    var v = viewSize();
    cam.x = v.w >= W ? (W - v.w) / 2 : Math.max(0, Math.min(W - v.w, cam.x));
    cam.y = v.h >= H ? (H - v.h) / 2 : Math.max(0, Math.min(H - v.h, cam.y));
  }
  function updateCamera(snap) {
    if (!cam.follow) return;
    var v = viewSize(), focus = me ? me.pos : { x: W / 2, y: H / 2 };
    var tx = focus.x - v.w / 2, ty = focus.y - v.h / 2;
    cam.x = snap ? tx : cam.x + (tx - cam.x) * 0.08;
    cam.y = snap ? ty : cam.y + (ty - cam.y) * 0.08;
    clampCam();
  }
  function toWorld(e) {
    var r = canvas.getBoundingClientRect(), k = canvas.width / r.width;
    return { x: cam.x + (e.clientX - r.left) * k / cam.scale, y: cam.y + (e.clientY - r.top) * k / cam.scale };
  }
  // Zoom around a point of the screen (the pinch's middle, the mouse), which
  // stays put; the scenery is redrawn sharp once the gesture settles.
  var redrawTimer = null;
  function setZoom(z, clientX, clientY) {
    var r = canvas.getBoundingClientRect(), k = canvas.width / r.width;
    var px = (clientX === undefined ? r.width / 2 : clientX - r.left) * k;
    var py = (clientY === undefined ? r.height / 2 : clientY - r.top) * k;
    var wx = cam.x + px / cam.scale, wy = cam.y + py / cam.scale;
    cam.zoom = Math.max(cam.zoomMin, Math.min(ZOOM_MAX, z));
    cam.scale = cam.base * cam.zoom;
    cam.x = wx - px / cam.scale; cam.y = wy - py / cam.scale;
    cam.follow = false;
    clampCam();
    clearTimeout(redrawTimer);
    redrawTimer = setTimeout(redrawScenery, 250);
    if (still) frame(performance.now());
  }
  canvas.addEventListener('wheel', function (e) {
    e.preventDefault();
    setZoom(cam.zoom * Math.exp(-e.deltaY * 0.0015), e.clientX, e.clientY);
  }, { passive: false });
  document.querySelectorAll('[data-zoom]').forEach(function (b) {
    b.addEventListener('click', function () { setZoom(cam.zoom * Number(b.dataset.zoom)); });
  });

  var drag = null, dragged = false, pointers = {}, pinch = null;
  canvas.addEventListener('pointerdown', function (e) {
    pointers[e.pointerId] = { x: e.clientX, y: e.clientY };
    var ids = Object.keys(pointers);
    if (ids.length === 2) {
      var a = pointers[ids[0]], b = pointers[ids[1]];
      pinch = { d: Math.hypot(a.x - b.x, a.y - b.y), zoom: cam.zoom };
      drag = null; dragged = true;
      return;
    }
    drag = { x: e.clientX, y: e.clientY, cx: cam.x, cy: cam.y }; dragged = false;
  });
  window.addEventListener('pointermove', function (e) {
    if (pointers[e.pointerId]) pointers[e.pointerId] = { x: e.clientX, y: e.clientY };
    if (pinch) {
      var ids = Object.keys(pointers);
      if (ids.length < 2) return;
      var a = pointers[ids[0]], b = pointers[ids[1]];
      setZoom(pinch.zoom * Math.hypot(a.x - b.x, a.y - b.y) / pinch.d, (a.x + b.x) / 2, (a.y + b.y) / 2);
      return;
    }
    if (!drag) return;
    var dx = e.clientX - drag.x, dy = e.clientY - drag.y;
    if (!dragged && Math.hypot(dx, dy) < 8) return;
    dragged = true; cam.follow = false;
    var k = canvas.width / canvas.getBoundingClientRect().width / cam.scale;
    cam.x = drag.cx - dx * k; cam.y = drag.cy - dy * k;
    clampCam();
    if (still) frame(performance.now());
  });
  function pointerEnd(e) {
    delete pointers[e.pointerId];
    if (Object.keys(pointers).length < 2) pinch = null;
    drag = null;
  }
  window.addEventListener('pointerup', pointerEnd);
  window.addEventListener('pointercancel', pointerEnd);

  canvas.addEventListener('click', function (e) {
    if (dragged) { dragged = false; return; }
    cam.follow = true;
    var p = toWorld(e);
    for (var i = 0; i < actors.length; i++) {
      var a = actors[i];
      if (a !== me && Math.hypot(a.pos.x - p.x, a.pos.y - 5 - p.y) < 12) {
        say(a, a.p.member ? a.p.name + ' · ' + a.p.rank + ' · ' + txt.days.replace('%d', a.p.days)
          : a.p.name + ' · ' + a.p.days + ' / ' + txt.threshold);
        if (still) frame(performance.now());
        return;
      }
    }
    if (Math.hypot(guard.pos.x - p.x, guard.pos.y - 5 - p.y) < 12) {
      say(guard, me && !me.p.member ? txt.guard.replace('%d', Number(txt.threshold) - me.p.days) : txt.hello);
      if (still) frame(performance.now());
      return;
    }
    if (!me) return;
    var tIn = inside(p);
    if (tIn && !me.p.member) {
      // The guard keeps aspirants out — with a word of encouragement.
      route(me, { x: 236, y: 332 });
      say(guard, txt.guard.replace('%d', Number(txt.threshold) - me.p.days), 3.5);
      return;
    }
    // A member tapping their own seat sits back down.
    if (me.seat && Math.hypot(me.seat.x - p.x, me.seat.y - p.y) < 12) {
      route(me, { x: me.seat.x, y: me.seat.y });
      me.toSeat = true;
      return;
    }
    if (!walkable(p, tIn)) p = randomSpot(tIn);
    route(me, p);
  });

  // --- static art, drawn once ---
  function drawBackground() {
    var c = bg.getContext('2d');
    c.save(); c.scale(bg.width / W, bg.height / H);
    function rect(x, y, w, h, col) { c.fillStyle = col; c.fillRect(x, y, w, h); }
    function circle(x, y, r, col) { c.fillStyle = col; c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill(); }
    var x, y, i, k;
    // grass in two tones, tufts and flowers
    rect(0, 0, W, H, '#7fb24f');
    // soft patches of darker grass, then blades
    for (i = 0; i < 70; i++) {
      c.fillStyle = 'rgba(70,120,40,0.18)';
      c.beginPath(); c.ellipse((i * 157) % W, (i * 89 + (i % 5) * 31) % H, 14 + (i % 4) * 5, 8 + (i % 3) * 3, 0, 0, Math.PI * 2); c.fill();
    }
    c.strokeStyle = '#5f9a3a'; c.lineWidth = 1;
    for (i = 0; i < 320; i++) {
      var bx = (i * 97) % W, by = (i * 61 + (i % 7) * 13) % H;
      c.beginPath(); c.moveTo(bx, by + 4); c.lineTo(bx - 1, by); c.moveTo(bx + 1, by + 4); c.lineTo(bx + 2, by + 1); c.stroke();
    }
    var flowers = ['#f4d35e', '#fff7e8', '#e85d75', '#b58cf0'];
    for (i = 0; i < 90; i++) {
      var fx = (i * 131) % W, fy = (i * 83) % H;
      if (!inside({ x: fx, y: fy })) rect(fx, fy, 3, 3, flowers[i % 4]);
    }
    // the river along the top, and a wooden bridge
    c.fillStyle = '#4f9fd1'; c.beginPath(); c.moveTo(0, 6);
    for (x = 0; x <= W; x += 20) c.lineTo(x, 8 + Math.sin(x / 40) * 4);
    for (x = W; x >= 0; x -= 20) c.lineTo(x, 26 + Math.sin(x / 40 + 1) * 4);
    c.fill();
    for (x = 10; x < W; x += 34) rect(x, 14 + Math.sin(x / 40) * 3, 10, 2, '#7cc0e8');
    rect(226, 0, 28, 36, '#9b6b3c'); for (y = 2; y < 36; y += 6) rect(226, y, 28, 2, '#7d5530');
    rect(224, 0, 3, 36, '#6b4527'); rect(253, 0, 3, 36, '#6b4527');
    // sheep pen on the west meadow
    c.strokeStyle = '#8a6239'; c.lineWidth = 2; c.strokeRect(6, 70, 46, 200);
    for (y = 70; y <= 270; y += 20) { rect(4, y - 2, 4, 6, '#6b4527'); rect(50, y - 2, 4, 6, '#6b4527'); }
    // market on the east side: striped stalls — coins, amphorae, fruit
    [[430, 70, '#d9534f'], [430, 150, '#3b7dd8'], [430, 230, '#e3a72f']].forEach(function (s, n) {
      rect(s[0], s[1] + 22, 44, 18, '#a7804f');
      for (var sx = 0; sx < 44; sx += 8) rect(s[0] + sx, s[1], 8, 22, sx % 16 ? '#f5efe0' : s[2]);
      rect(s[0], s[1] + 22, 44, 3, 'rgba(0,0,0,0.2)');
      for (var cx = 0; cx < 5; cx++) {
        if (n === 0) circle(s[0] + 8 + cx * 7, s[1] + 31, 2.4, '#f2c94c');
        if (n === 1 && cx < 4) circle(s[0] + 9 + cx * 9, s[1] + 31, 3.4, '#b5603a');
        if (n === 2) circle(s[0] + 8 + cx * 7, s[1] + 31, 2.6, cx % 2 ? '#7a9c3b' : '#9b3b5a');
      }
    });
    // path to the gate, with edge stones
    rect(222, 300, 36, 100, '#d9c08a');
    for (y = 304; y < H; y += 10) { rect(219, y, 3, 5, '#b9a06c'); rect(258, y + 5, 3, 5, '#b9a06c'); }
    // rocks, bushes, signpost
    [[290, 352], [128, 386], [362, 388]].forEach(function (r) { circle(r[0], r[1], 7, '#9a958c'); circle(r[0] - 2, r[1] - 2, 3, '#bdb8ae'); });
    [[90, 330], [160, 360], [330, 334], [400, 362], [70, 390], [190, 392]].forEach(function (b) { circle(b[0], b[1], 6, '#4e8a33'); circle(b[0] - 2, b[1] - 2, 3, '#6aa845'); });
    rect(287, 344, 3, 16, '#6b4527'); rect(272, 338, 34, 9, '#a8784a');
    // trees outside
    treesOut.forEach(function (t) {
      circle(t[0] + 3, t[1] + 9, 13, 'rgba(0,0,0,0.18)'); rect(t[0] - 2, t[1] + 2, 4, 9, '#6b4527');
      circle(t[0], t[1], 13, '#3f7d2d'); circle(t[0] - 4, t[1] - 4, 7, '#5a9a3c'); circle(t[0] + 5, t[1] + 2, 5, '#36702a');
    });
    // agora floor: flagstones
    rect(WALL.x0, WALL.y0, WALL.x1 - WALL.x0, WALL.y1 - WALL.y0, '#e6d9b8');
    c.strokeStyle = 'rgba(150,125,80,0.22)'; c.lineWidth = 1;
    for (y = WALL.y0; y < WALL.y1; y += 12) for (x = WALL.x0 + ((y / 12) % 2) * 9; x < WALL.x1; x += 18) c.strokeRect(x, y, 18, 12);
    // walls with crenels, and the gate
    c.fillStyle = '#bfa06a';
    c.fillRect(WALL.x0, WALL.y0, WALL.x1 - WALL.x0, WALL.t);
    c.fillRect(WALL.x0, WALL.y0, WALL.t, WALL.y1 - WALL.y0);
    c.fillRect(WALL.x1 - WALL.t, WALL.y0, WALL.t, WALL.y1 - WALL.y0);
    c.fillRect(WALL.x0, WALL.y1 - WALL.t, GATE.x0 - WALL.x0, WALL.t);
    c.fillRect(GATE.x1, WALL.y1 - WALL.t, WALL.x1 - GATE.x1, WALL.t);
    for (x = WALL.x0; x < WALL.x1; x += 12) rect(x, WALL.y0, 6, 3, '#a98a55');
    [GATE.x0 - 10, GATE.x1 - 2].forEach(function (gx) { rect(gx, WALL.y1 - 16, 12, 20, '#a98a55'); rect(gx, WALL.y1 - 16, 12, 4, '#c9ad78'); });
    // temple: steps, tiled roof with a ridge, columns, the doorway
    rect(TEMPLE.x0 - 12, TEMPLE.y1, TEMPLE.x1 - TEMPLE.x0 + 24, 6, '#d6c49c');
    rect(TEMPLE.x0 - 8, TEMPLE.y1 + 6, TEMPLE.x1 - TEMPLE.x0 + 16, 5, '#cdb98f');
    rect(TEMPLE.x0, TEMPLE.y0, TEMPLE.x1 - TEMPLE.x0, (TEMPLE.y1 - TEMPLE.y0) / 2, '#c96a3f');
    rect(TEMPLE.x0, (TEMPLE.y0 + TEMPLE.y1) / 2, TEMPLE.x1 - TEMPLE.x0, (TEMPLE.y1 - TEMPLE.y0) / 2, '#a8522d');
    for (x = TEMPLE.x0; x < TEMPLE.x1; x += 8) for (y = TEMPLE.y0; y < TEMPLE.y1; y += 6) rect(x + ((y / 6) % 2) * 4, y, 1, 6, 'rgba(0,0,0,0.12)');
    rect(TEMPLE.x0, (TEMPLE.y0 + TEMPLE.y1) / 2 - 2, TEMPLE.x1 - TEMPLE.x0, 4, '#ecdcb5');
    for (k = 0; k < 8; k++) {
      var colx = TEMPLE.x0 + 6 + k * (TEMPLE.x1 - TEMPLE.x0 - 12) / 7;
      circle(colx, TEMPLE.y1 + 2, 4, '#f6efe0'); circle(colx, TEMPLE.y1 + 2, 2, '#e1d4b6');
    }
    rect(232, TEMPLE.y1 - 4, 16, 6, '#5b3a22');
    // the hemicycle of stone tiers facing the temple
    // (each step: a stone band, its lit front edge facing the stage, the
    // shadow cast by the step behind)
    var A0 = TIERS.from - 0.05, A1 = TIERS.to + 0.05;
    function arcStroke(r, w, col) { c.strokeStyle = col; c.lineWidth = w; c.beginPath(); c.arc(TIERS.cx, TIERS.cy, r, A0, A1); c.stroke(); }
    for (var t = TIERS.r.length - 1; t >= 0; t--) {
      var R = TIERS.r[t];
      arcStroke(R, 20, t % 2 ? '#d2bf95' : '#dccaa2');
      arcStroke(R + 8.5, 3, 'rgba(110,85,50,0.28)');   // shadow under the next step up
      arcStroke(R - 9, 1.6, 'rgba(255,248,230,0.85)');  // lit edge of the step
    }
    // outer wall of the hemicycle, and its end caps
    arcStroke(TIERS.r[TIERS.r.length - 1] + 12, 4, '#b89f72');
    [A0, A1].forEach(function (a) {
      var r0 = TIERS.r[0] - 10, r1 = TIERS.r[TIERS.r.length - 1] + 14;
      c.strokeStyle = '#b89f72'; c.lineWidth = 4; c.beginPath();
      c.moveTo(TIERS.cx + Math.cos(a) * r0, TIERS.cy + Math.sin(a) * r0); c.lineTo(TIERS.cx + Math.cos(a) * r1, TIERS.cy + Math.sin(a) * r1); c.stroke();
    });
    // stairs running up through the tiers
    AISLES.forEach(function (a) {
      var w = 0.06;
      c.fillStyle = '#e9dcbb'; c.beginPath();
      c.arc(TIERS.cx, TIERS.cy, TIERS.r[TIERS.r.length - 1] + 10, a - w, a + w);
      c.arc(TIERS.cx, TIERS.cy, TIERS.r[0] - 10, a + w, a - w, true); c.fill();
      c.strokeStyle = 'rgba(120,95,55,0.4)'; c.lineWidth = 1;
      for (var rr = TIERS.r[0] - 10; rr <= TIERS.r[TIERS.r.length - 1] + 10; rr += 5) {
        c.beginPath(); c.arc(TIERS.cx, TIERS.cy, rr, a - w, a + w); c.stroke();
      }
    });
    // a red carpet from the speakers' platform to the thrones
    c.fillStyle = '#a83232'; c.fillRect(BEMA.x - 5, BEMA.y + 6, 10, TIERS.r[0] - 18);
    c.fillStyle = '#d4a531'; c.fillRect(BEMA.x - 5, BEMA.y + 6, 1.2, TIERS.r[0] - 18); c.fillRect(BEMA.x + 3.8, BEMA.y + 6, 1.2, TIERS.r[0] - 18);
    // the speakers' platform
    rect(BEMA.x - 18, BEMA.y - 7, 36, 14, '#bba57a'); rect(BEMA.x - 16, BEMA.y - 7, 32, 3, '#d7c49c');
    // statue of Croesus with his crown, a fountain, planters, amphorae
    rect(STATUE.x - 10, STATUE.y - 2, 20, 14, '#b9ab90'); rect(STATUE.x - 10, STATUE.y - 2, 20, 3, '#d3c7ae');
    circle(STATUE.x, STATUE.y - 8, 6, '#d9d2c3'); circle(STATUE.x, STATUE.y - 15, 4, '#e6dfd1'); rect(STATUE.x - 4, STATUE.y - 20, 8, 2, '#d9b84c');
    // the fountain: an octagonal stone basin, deep water, a pedestal
    function octagon(r, col) {
      c.fillStyle = col; c.beginPath();
      for (var o = 0; o < 8; o++) { var oa = Math.PI / 8 + o * Math.PI / 4; c.lineTo(FOUNTAIN.x + Math.cos(oa) * r, FOUNTAIN.y + Math.sin(oa) * r); }
      c.closePath(); c.fill();
    }
    octagon(FOUNTAIN.r + 3, 'rgba(0,0,0,0.15)');
    octagon(FOUNTAIN.r + 2, '#cdb98f'); octagon(FOUNTAIN.r, '#e6d7b2'); octagon(FOUNTAIN.r - 3, '#3f8fc4');
    var wg = c.createRadialGradient(FOUNTAIN.x - 4, FOUNTAIN.y - 4, 2, FOUNTAIN.x, FOUNTAIN.y, FOUNTAIN.r);
    wg.addColorStop(0, '#7cc3ea'); wg.addColorStop(1, '#3a86bd');
    c.fillStyle = wg; octagon(FOUNTAIN.r - 3.5, wg);
    circle(FOUNTAIN.x, FOUNTAIN.y, 5, '#d8cba8'); circle(FOUNTAIN.x, FOUNTAIN.y, 3.4, '#efe6cf');
    [[82, 200], [398, 200]].forEach(function (p) {
      rect(p[0] - 8, p[1] - 6, 16, 12, '#9b6b3c');
      circle(p[0] - 3, p[1] - 4, 3, '#e85d75'); circle(p[0] + 3, p[1] - 5, 3, '#f4d35e'); circle(p[0], p[1] - 1, 3, '#5a9a3c');
    });
    // seats by rank: thrones, purple and blue cushions, stone benches
    // Citizens' rows: one continuous stone bench per tier, with joints.
    [4, 5].forEach(function (t) {
      var R = TIERS.r[t];
      arcStroke(R + 1, 7, '#efe5cf'); arcStroke(R + 4.5, 1.2, 'rgba(110,85,50,0.3)');
      for (var a = TIERS.from; a < TIERS.to; a += 0.09) {
        if (inAisle(a)) continue;
        c.strokeStyle = 'rgba(110,85,50,0.25)'; c.lineWidth = 0.8; c.beginPath();
        c.moveTo(TIERS.cx + Math.cos(a) * (R - 2.5), TIERS.cy + Math.sin(a) * (R - 2.5));
        c.lineTo(TIERS.cx + Math.cos(a) * (R + 4.5), TIERS.cy + Math.sin(a) * (R + 4.5)); c.stroke();
      }
    });
    // Each seat drawn turned toward the stage: x along the row, y away from
    // the centre (the back of the seat is at +y).
    seatList.forEach(function (st) {
      if (st.rank === 0) return;
      c.save();
      c.translate(st.x, st.y);
      c.rotate(st.a - Math.PI / 2);
      if (st.rank === 3) {
        // throne: gold frame, high back, armrests, purple velvet, lion feet
        c.fillStyle = 'rgba(0,0,0,0.18)'; c.beginPath(); c.roundRect(-6.5, -4, 14, 13, 2); c.fill();
        c.fillStyle = '#b8901f'; c.beginPath(); c.roundRect(-7, -5, 14, 13, 2); c.fill();
        c.fillStyle = '#e6c25a'; c.beginPath(); c.roundRect(-7, 3.5, 14, 4.5, 2); c.fill();      // back
        c.fillStyle = '#f5dd8a'; c.fillRect(-5.5, 6.5, 11, 1);
        c.fillStyle = '#6d2f8a'; c.beginPath(); c.roundRect(-4.5, -3.5, 9, 7, 1.5); c.fill();     // velvet
        c.fillStyle = '#8e46b0'; c.fillRect(-4, -3, 8, 1.6);
        c.fillStyle = '#d4a531'; c.fillRect(-7, -5, 2.2, 9); c.fillRect(4.8, -5, 2.2, 9);         // armrests
        [-6, 6].forEach(function (fx) { c.beginPath(); c.arc(fx, -5.5, 1.2, 0, Math.PI * 2); c.fill(); });
      } else {
        // cushion with a sheen and a shadow; tassels for the philosophers
        var base = st.rank === 2 ? '#6f3fa5' : '#2f6fc4', lite = st.rank === 2 ? '#9a6cd0' : '#6aa4ee';
        c.fillStyle = 'rgba(0,0,0,0.16)'; c.beginPath(); c.roundRect(-4.5, -2.4, 10, 6.4, 2.4); c.fill();
        c.fillStyle = base; c.beginPath(); c.roundRect(-5, -3, 10, 6.4, 2.4); c.fill();
        c.fillStyle = lite; c.beginPath(); c.roundRect(-3.8, -2.2, 7.6, 2, 1); c.fill();
        if (st.rank === 1) { c.strokeStyle = '#f4efe2'; c.lineWidth = 0.7; c.beginPath(); c.roundRect(-4.4, -2.4, 8.8, 5.2, 2); c.stroke(); }
        else { c.fillStyle = '#e2b93b'; [[-5, -3], [5, -3], [-5, 3.4], [5, 3.4]].forEach(function (q) { c.beginPath(); c.arc(q[0], q[1], 0.9, 0, Math.PI * 2); c.fill(); }); }
      }
      c.restore();
    });
    // the library (scrolls on its shelves) and Croesus's treasury (gold)
    rect(78, 52, 56, 40, '#efe3c6'); rect(78, 52, 56, 14, '#5d7a99'); rect(78, 64, 56, 2, '#3f5a78');
    for (k = 0; k < 4; k++) rect(82 + k * 13, 70, 9, 16, '#9b6b3c');
    for (k = 0; k < 4; k++) for (var sc = 0; sc < 3; sc++) circle(86.5 + k * 13, 73 + sc * 5, 1.8, '#f3ead2');
    rect(346, 52, 56, 40, '#efe3c6'); rect(346, 52, 56, 14, '#b8862f'); rect(346, 64, 56, 2, '#8a6420');
    rect(352, 70, 44, 18, '#6b4a2a');
    [[358, 84], [364, 84], [370, 84], [376, 84], [382, 84], [388, 84], [361, 80], [367, 80], [373, 80], [379, 80], [385, 80], [364, 76], [370, 76], [376, 76], [382, 76], [373, 72]].forEach(function (cn) { circle(cn[0], cn[1], 2.6, '#f2c94c'); circle(cn[0] - 0.6, cn[1] - 0.6, 1, '#fff2b0'); });
    c.restore();
  }

  // --- per-frame art ---
  function rect(x, y, w, h, col) { g.fillStyle = col; g.fillRect(x, y, w, h); }
  function circle(x, y, r, col) { g.fillStyle = col; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.fill(); }

  function rankTier(days) { return days >= 365 ? 3 : days >= 250 ? 2 : days >= 100 ? 1 : 0; }

  // A character: their sprite — walking, standing (breathing) or seated on
  // a bench — with a shadow, and what their rank carries. While they speak
  // (a bubble), they turn to face the viewer.
  var SPRITE = 30; // a 64-px sprite frame, in world units: figures ~22 tall
  function drawPerson(a, t) {
    var x = a.pos.x, y = a.pos.y + 6, walking = a.path && a.path.length > 0 && !still;
    g.fillStyle = 'rgba(0,0,0,0.22)'; g.beginPath(); g.ellipse(x, y, 6, 2.2, 0, 0, Math.PI * 2); g.fill();
    if (!a.sheets) return;
    var talking = a.bubble && t < a.bubble.until;
    var dir = a.dir || 'down', sheet = a.sheets.idle, f = still ? 0 : Math.floor(t / 700 + a.phase) % 2;
    if (a.sitting) { sheet = a.sheets.sit; f = 2; dir = talking ? 'down' : (a.seat && a.seat.dir) || 'up'; }
    else if (walking) { sheet = a.sheets.walk; f = 1 + Math.floor((a.stride || 0) / 3) % 8; }
    else if (talking) dir = 'down';
    // Crisp pixels when each sprite pixel covers 2+ screen pixels; smoothed
    // below that, where nearest-neighbour would make them uneven.
    g.imageSmoothingEnabled = cam.scale * SPRITE / 64 < 2;
    LPC.drawFrame(g, sheet, dir, f, x, y, SPRITE);
    g.imageSmoothingEnabled = true;
    var sit = a.sitting ? 2 : 0;
    // what each rank carries: citizens an olive branch, orators a scroll,
    // philosophers a lamp, sages a golden staff
    if (a.p && a.p.member) {
      var hx = dir === 'left' ? x - 9 : x + 7, hh = a.pos.y + sit;
      switch (rankTier(a.p.days)) {
        case 0:
          g.strokeStyle = '#5a8a3a'; g.lineWidth = 1; g.beginPath(); g.moveTo(hx, hh + 2); g.lineTo(hx + 2, hh - 6); g.stroke();
          circle(hx + 0.5, hh - 2, 1.3, '#7bbf4a'); circle(hx + 1.8, hh - 4.5, 1.3, '#7bbf4a');
          break;
        case 1:
          rect(hx - 1, hh - 4, 3, 7, '#f3ead2'); rect(hx - 1.5, hh - 4.5, 4, 1.5, '#c9b07a'); rect(hx - 1.5, hh + 2.5, 4, 1.5, '#c9b07a');
          break;
        case 2:
          g.fillStyle = '#b5603a'; g.beginPath(); g.ellipse(hx + 1, hh, 3, 1.6, 0, 0, Math.PI * 2); g.fill();
          circle(hx + 3.5, hh - 1.5 - (still ? 0 : Math.abs(Math.sin(t / 120)) * 0.6), 1.3, '#f7c948');
          break;
        case 3:
          rect(hx, hh - 12, 1.6, 18, '#c9a227'); circle(hx + 0.8, hh - 13, 2.4, '#f2d36b');
          break;
      }
    }
    if (a.guard) { // a spear
      rect(x + 9, y - 26, 1.4, 26, '#7a5530');
      g.fillStyle = '#c9c9c9'; g.beginPath(); g.moveTo(x + 9.7, y - 31); g.lineTo(x + 11.4, y - 26); g.lineTo(x + 8, y - 26); g.fill();
    }
  }

  // The character editor: a big live preview of the reader's character,
  // turning on itself, and each option's tile drawn on their own character
  // (close on the head for hair, beard and headwear; capes from behind).
  var preview = document.getElementById('look-preview');
  if (preview && window.LPC) {
    var pal = JSON.parse(document.getElementById('look-palettes').textContent || '{}');
    var form = document.getElementById('look-form');
    var val = function (n) { var el = form.querySelector('input[name="' + n + '"]:checked'); return el ? el.value : ''; };
    var meP = me ? me.p : { member: false, days: 0 };
    var currentSpec = function (field, value) {
      var v = { body: val('body'), skin: val('skin'), hair: val('hair'), haircolor: val('haircolor'), beard: val('beard'), tunic: val('tunic'), cape: val('cape'), head: val('head') };
      if (field) v[field] = value;
      return {
        body: pal.bodies[Number(v.body)], skin: pal.skins[Number(v.skin)], hair: pal.hairStyles[Number(v.hair)],
        hairColor: pal.hairColors[Number(v.haircolor)], beard: pal.beards[Number(v.beard)],
        tunic: pal.tunics[v.tunic], cape: pal.capes[v.cape] || '', head: pal.heads[v.head] || '',
      };
    };
    // the head, or the whole figure, within a 64-px frame
    var CROP = { head: [14, 6, 36, 36], body: [10, 10, 44, 54] };
    var paint = function (cv, spec, zoom, field, dir) {
      if (zoom === 'head' && field !== 'head') spec.head = '';
      if (zoom !== 'head' && meP.laurel && !spec.head) spec.head = 'laurel';
      LPC.build(spec, ['walk']).then(function (sh) {
        var dpr = window.devicePixelRatio || 1;
        cv.width = Math.round(cv.clientWidth * dpr); cv.height = Math.round(cv.clientHeight * dpr);
        var c = cv.getContext('2d');
        c.imageSmoothingEnabled = false;
        c.clearRect(0, 0, cv.width, cv.height);
        var r = CROP[zoom] || CROP.body, pad = cv.width * 0.06;
        LPC.drawCrop(c, sh.walk, dir || (field === 'cape' ? 'up' : 'down'), 0, r[0], r[1], r[2], r[3], pad, pad, cv.width - 2 * pad, cv.height - 2 * pad);
      });
    };
    var DIRS = ['down', 'left', 'up', 'right'], turn = 0, turning = null;
    var drawPreview = function () {
      paint(preview, currentSpec(), 'body', '', DIRS[turn % 4]);
      form.querySelectorAll('.look-thumb').forEach(function (cv) { paint(cv, currentSpec(cv.dataset.field, cv.dataset.value), cv.dataset.zoom, cv.dataset.field); });
    };
    LPC.ready().then(function () {
      form.addEventListener('change', function () { turn = 0; drawPreview(); });
      document.querySelectorAll('[data-open="panel-look"]').forEach(function (b) {
        b.addEventListener('click', function () {
          setTimeout(drawPreview, 0);
          // the preview turns slowly while the panel is open
          if (!turning && !still) turning = setInterval(function () {
            if (document.getElementById('panel-look').hidden) { clearInterval(turning); turning = null; return; }
            turn++;
            paint(preview, currentSpec(), 'body', '', DIRS[turn % 4]);
          }, 1400);
        });
      });
    });
  }

  function drawWorld(t) {
    // water: river glints, fountain jet
    for (var x = 0; x < W; x += 30) rect((x + t / 40) % W, 16 + Math.sin(x / 40) * 3, 6, 1.5, 'rgba(255,255,255,0.55)');
    // fountain: ripples spreading, four arcs of water with droplets running
    // along them, little splashes where they land
    var ft = still ? 0 : t;
    for (var rp = 0; rp < 3; rp++) {
      var life = ((ft / 2200) + rp / 3) % 1;
      g.strokeStyle = 'rgba(255,255,255,' + (0.45 * (1 - life)) + ')'; g.lineWidth = 0.8;
      g.beginPath(); g.ellipse(FOUNTAIN.x, FOUNTAIN.y, 6 + life * 10, 5 + life * 8, 0, 0, Math.PI * 2); g.stroke();
    }
    for (var j = 0; j < 4; j++) {
      var ja = Math.PI / 4 + j * Math.PI / 2, ex = FOUNTAIN.x + Math.cos(ja) * 11, ey = FOUNTAIN.y + Math.sin(ja) * 11;
      var mx = (FOUNTAIN.x + ex) / 2, my = (FOUNTAIN.y + ey) / 2 - 10;
      g.strokeStyle = 'rgba(220,240,255,0.75)'; g.lineWidth = 1.4;
      g.beginPath(); g.moveTo(FOUNTAIN.x, FOUNTAIN.y - 3); g.quadraticCurveTo(mx, my, ex, ey); g.stroke();
      for (var dr = 0; dr < 3; dr++) {
        var u = ((ft / 600) + dr / 3 + j * 0.13) % 1, iu = 1 - u;
        var px = iu * iu * FOUNTAIN.x + 2 * iu * u * mx + u * u * ex, py = iu * iu * (FOUNTAIN.y - 3) + 2 * iu * u * my + u * u * ey;
        circle(px, py, 0.9, '#ffffff');
      }
      circle(ex, ey, 1.2 + Math.abs(Math.sin(ft / 150 + j)) * 1.4, 'rgba(255,255,255,0.5)');
    }
    circle(FOUNTAIN.x, FOUNTAIN.y - 6 - (still ? 0 : Math.abs(Math.sin(ft / 180)) * 2), 1.6, 'rgba(235,248,255,0.9)');
    // braziers
    BRAZIERS.forEach(function (b, i) {
      rect(b.x - 4, b.y - 1, 8, 5, '#5b4a3a');
      var f = still ? 0 : Math.sin(t / 70 + i * 2);
      circle(b.x, b.y - 4 - f, 3.5, '#f39c34'); circle(b.x, b.y - 6 - f * 1.5, 2, '#f7d34a');
    });
    // sheep grazing in their pen
    sheep.forEach(function (s) {
      if (!still) { s.t += 0.004; s.pos = { x: 28 + Math.cos(s.t * 1.7) * 12, y: 170 + Math.sin(s.t) * 90 }; }
      circle(s.pos.x, s.pos.y, 5, '#f7f4ec'); circle(s.pos.x + 4, s.pos.y - 2, 2.5, '#3b3530');
    });
    // pigeons fly off when someone comes near, then settle back
    pigeons.forEach(function (b) {
      var near = actors.some(function (a) { return Math.hypot(a.pos.x - b.pos.x, a.pos.y - b.pos.y) < 22; });
      if (near && !still) b.fly = 60;
      if (b.fly > 0) { b.fly--; b.pos.y -= 1.6; b.pos.x += 0.8; }
      else { b.pos.x += (b.home.x - b.pos.x) * 0.03; b.pos.y += (b.home.y - b.pos.y) * 0.03; }
      circle(b.pos.x, b.pos.y, 2.4, '#8d8f99'); rect(b.pos.x + 1.5, b.pos.y - 1, 1.5, 1, '#e3a72f');
    });
    var tt = still ? 0 : t;
    // banners fluttering on the wall corners
    [[WALL.x0 + 2, WALL.y0 - 2, '#c0392b'], [WALL.x1 - 2, WALL.y0 - 2, '#2f5d9c'], [GATE.x0 - 4, WALL.y1 - 18, '#c0392b'], [GATE.x1 + 4, WALL.y1 - 18, '#2f5d9c']].forEach(function (f, i) {
      rect(f[0] - 0.75, f[1] - 14, 1.5, 16, '#6b4527');
      var wave = Math.sin(tt / 180 + i) * 2;
      g.fillStyle = f[2]; g.beginPath(); g.moveTo(f[0], f[1] - 14); g.lineTo(f[0] + 11, f[1] - 11 + wave); g.lineTo(f[0], f[1] - 7); g.fill();
    });
    // merchants behind their stalls, a glint on the coins
    [[452, 88, '#8e5a9c'], [452, 168, '#3c8c6e'], [452, 248, '#b5603a']].forEach(function (m) {
      circle(m[0], m[1] + 4, 4.5, m[2]); circle(m[0], m[1] - 1, 3.2, '#f0cfa8'); circle(m[0], m[1] - 3, 2.4, '#4a2f1c');
    });
    if (Math.sin(tt / 400) > 0.92) { rect(445, 99, 1.5, 4, '#fff'); rect(443.5, 100.5, 4.5, 1.5, '#fff'); }
    // ducks on the river, and now and then a fish jumps
    for (var d = 0; d < 3; d++) {
      var dx = ((tt / 90 + d * 150) % (W + 40)) - 20, dy = 17 + Math.sin(dx / 40) * 3 + d % 2;
      circle(dx, dy, 3.2, '#f2f0e6'); circle(dx + 2.6, dy - 2, 1.8, '#3f7d47'); rect(dx + 4, dy - 2.3, 2, 1, '#e3a72f');
    }
    var jump = (tt / 1000) % 7;
    if (jump < 0.8) {
      var fx = 120 + Math.floor(tt / 7000) % 5 * 60, fy = 18 - Math.sin(jump / 0.8 * Math.PI) * 9;
      g.fillStyle = '#d9822b'; g.beginPath(); g.ellipse(fx + jump * 10, fy, 3, 1.6, -0.6 + jump, 0, Math.PI * 2); g.fill();
      if (jump > 0.65) circle(fx + 8, 19, 3 * (jump - 0.6) * 5, 'rgba(255,255,255,0.5)');
    }
    // a cat asleep by Croesus's statue, tail swishing
    circle(STATUE.x + 16, STATUE.y + 10, 4.5, '#e08a3c'); circle(STATUE.x + 19.5, STATUE.y + 8, 2.6, '#e08a3c');
    g.strokeStyle = '#e08a3c'; g.lineWidth = 1.6; g.beginPath(); g.moveTo(STATUE.x + 12, STATUE.y + 11);
    g.quadraticCurveTo(STATUE.x + 7, STATUE.y + 14 + Math.sin(tt / 300) * 2, STATUE.x + 6, STATUE.y + 8); g.stroke();
    // smoke wisps rising from the braziers
    BRAZIERS.forEach(function (b, i) {
      for (var k = 0; k < 3; k++) {
        var life = ((tt / 1400 + k / 3 + i * 0.17) % 1);
        circle(b.x + Math.sin(life * 6 + i) * 3, b.y - 8 - life * 22, 1.5 + life * 3, 'rgba(120,120,120,' + (0.25 * (1 - life)) + ')');
      }
    });
    // people, back to front
    actors.concat([guard]).sort(function (a, b) { return a.pos.y - b.pos.y; }).forEach(function (a) { drawPerson(a, t); });
    // butterflies
    butterflies.forEach(function (b) {
      if (!still) { b.s += 0.02; b.x = (b.x + 0.4) % W; }
      var by = b.y + Math.sin(b.s * 3) * 6, w = Math.abs(Math.sin(b.s * 20)) * 2.5 + 0.5;
      rect(b.x - w, by, w, 2, b.c); rect(b.x + 0.5, by, w, 2, b.c);
    });
    // a flock of birds crossing the sky now and then
    var flight = (tt / 1000) % 25;
    if (flight < 9) {
      var bx = -30 + flight * 60, by = 60 + Math.sin(flight) * 10;
      g.strokeStyle = 'rgba(40,40,40,0.75)'; g.lineWidth = 1.2;
      [[0, 0], [-8, -5], [-8, 5], [-16, -10], [-16, 10]].forEach(function (o, k) {
        var flap = Math.sin(tt / 90 + k) * 1.6, x0 = bx + o[0], y0 = by + o[1];
        g.beginPath(); g.moveTo(x0 - 3, y0 - flap); g.lineTo(x0, y0); g.lineTo(x0 + 3, y0 - flap); g.stroke();
      });
    }
    // cloud shadows drifting over everything
    if (!still) for (var k = 0; k < 2; k++) {
      var cx = ((t / 120 + k * 260) % (W + 200)) - 100;
      g.fillStyle = 'rgba(30,40,60,0.07)'; g.beginPath(); g.ellipse(cx, 120 + k * 150, 70, 30, 0, 0, Math.PI * 2); g.fill();
    }
    // time of day, from the reader's clock
    var h = new Date().getHours(), night = h >= 21 || h < 6, dusk = (h >= 18 && h < 21) || h === 6;
    if (night || dusk) {
      g.fillStyle = night ? 'rgba(18,24,60,0.5)' : 'rgba(240,120,40,0.16)'; g.fillRect(0, 0, W, H);
      if (night) {
        g.globalCompositeOperation = 'lighter';
        BRAZIERS.forEach(function (b) {
          var grd = g.createRadialGradient(b.x, b.y - 4, 2, b.x, b.y - 4, 46);
          grd.addColorStop(0, 'rgba(255,170,70,0.45)'); grd.addColorStop(1, 'rgba(255,170,70,0)');
          g.fillStyle = grd; g.fillRect(b.x - 46, b.y - 50, 92, 92);
        });
        g.globalCompositeOperation = 'source-over';
      }
    }
  }

  // --- names and bubbles, at full resolution ---
  // Names and bubbles keep the same size on screen whatever the zoom: they
  // are positioned in world units, sized in screen pixels (u = one CSS px).
  function drawOverlay(t) {
    var s = cam.scale, u = (window.devicePixelRatio || 1) / s;
    view.setTransform(s, 0, 0, s, -cam.x * s, -cam.y * s);
    view.textAlign = 'center';
    view.lineJoin = 'round'; // no miter spikes on outlined text
    actors.forEach(function (a) {
      var name = a.p.name.length > 12 ? a.p.name.slice(0, 11) + '…' : a.p.name;
      view.font = (a.p.you ? 'bold ' : '') + (11 * u) + 'px sans-serif';
      view.lineWidth = 3 * u; view.strokeStyle = 'rgba(255,255,255,0.8)';
      view.strokeText(name, a.pos.x, a.pos.y + 9 + 11 * u);
      view.fillStyle = '#2b2416'; view.fillText(name, a.pos.x, a.pos.y + 9 + 11 * u);
      if (a.p.you) {
        var b = still ? 0 : Math.sin(t / 200) * 3 * u;
        var top = a.pos.y - 19 - 12 * u + b;
        view.fillStyle = '#2f6f4f';
        view.beginPath(); view.moveTo(a.pos.x - 6 * u, top); view.lineTo(a.pos.x + 6 * u, top); view.lineTo(a.pos.x, top + 8 * u); view.fill();
      }
    });
    view.font = 'bold 8px Georgia, serif'; view.fillStyle = '#5b3a22'; view.fillText('ΛΥΔΙ', 240, 68);
    view.font = '5px Georgia, serif'; view.fillStyle = '#f4ecda';
    view.fillText('ΒΙΒΛΙΟΘΗΚΗ', 106, 61); view.fillText('ΘΗΣΑΥΡΟΣ', 374, 61);
    // the rank of each block of tiers, engraved at its end
    var ranks = (txt.ranks || '').split('|');
    // (written along the tier, at its left end, so the blocks don't collide)
    view.font = 'italic 6px Georgia, serif'; view.fillStyle = 'rgba(90,65,30,0.85)';
    [[TIERS.r[0], 3], [TIERS.r[1], 2], [(TIERS.r[2] + TIERS.r[3]) / 2, 1], [(TIERS.r[4] + TIERS.r[5]) / 2, 0]].forEach(function (z) {
      if (!ranks[z[1]]) return;
      var a = TIERS.to - 0.02;
      view.save();
      view.translate(TIERS.cx + Math.cos(a) * z[0], TIERS.cy + Math.sin(a) * z[0]);
      view.rotate(Math.atan2(Math.cos(a), -Math.sin(a)));
      view.textAlign = 'left'; view.fillText(ranks[z[1]], 2, 2);
      view.restore();
    });
    view.textAlign = 'center';
    view.font = '6px Georgia, serif'; view.fillStyle = '#4a3418'; view.fillText('ΣΑΡΔΕΙΣ', 289, 345);
    [guard].concat(actors).forEach(function (a) {
      if (!a.bubble || t > a.bubble.until) return;
      view.font = (12 * u) + 'px sans-serif';
      var pad = 8 * u, h = 22 * u;
      var w = Math.min(view.measureText(a.bubble.text).width + 2 * pad, 300 * u);
      var x = a.pos.x - w / 2, y = a.pos.y - 21 - h;
      view.fillStyle = 'rgba(255,253,245,0.96)'; view.beginPath(); view.roundRect(x, y, w, h, 7 * u); view.fill();
      view.strokeStyle = 'rgba(60,40,10,0.35)'; view.lineWidth = u; view.stroke();
      view.beginPath(); view.moveTo(a.pos.x - 5 * u, y + h); view.lineTo(a.pos.x + 5 * u, y + h); view.lineTo(a.pos.x, y + h + 6 * u); view.fill();
      view.fillStyle = '#2b2416'; view.textAlign = 'left'; view.fillText(a.bubble.text, x + pad, y + h * 0.68, w - 2 * pad); view.textAlign = 'center';
    });
  }

  function frame(t) {
    updateCamera(false);
    var s = cam.scale, r = bgScale / s;
    view.setTransform(1, 0, 0, 1, 0, 0);
    view.fillStyle = '#6fa046'; view.fillRect(0, 0, canvas.width, canvas.height); // beyond the map's edges
    view.drawImage(bg, cam.x * bgScale, cam.y * bgScale, canvas.width * r, canvas.height * r, 0, 0, canvas.width, canvas.height);
    view.setTransform(s, 0, 0, s, -cam.x * s, -cam.y * s);
    drawWorld(t);
    drawOverlay(t);
  }

  // --- simulation ---
  var last = performance.now(), nextTalk = last + 3500;
  function step(t) {
    var dt = Math.min(0.05, (t - last) / 1000);
    last = t;
    actors.forEach(function (a) {
      if (a.path.length) {
        var target = a.path[0], dx = target.x - a.pos.x, dy = target.y - a.pos.y, d = Math.hypot(dx, dy);
        var v = (a === me ? 46 : a.speed || 20) * dt;
        if (d <= v) {
          a.pos = { x: target.x, y: target.y };
          a.path.shift();
          a.inside = inside(a.pos);
          if (!a.path.length) {
            a.wait = 2 + Math.random() * 5;
            var atSeat = a.seat && Math.hypot(a.pos.x - a.seat.x, a.pos.y - a.seat.y) < 1;
            if (a.toSeat || (atSeat && a !== me)) { a.sitting = true; a.toSeat = false; }
          }
        } else {
          a.pos = { x: a.pos.x + dx / d * v, y: a.pos.y + dy / d * v };
          a.dir = Math.abs(dx) > Math.abs(dy) ? (dx > 0 ? 'right' : 'left') : (dy > 0 ? 'down' : 'up');
          a.stride = (a.stride || 0) + v;
        }
      } else if (a !== me) {
        a.wait -= dt;
        if (a.wait <= 0) {
          if (a.p.member && a.seat) {
            // Members mostly stay in their seat; now and then one stretches
            // their legs, then comes back.
            if (a.sitting && Math.random() < 0.15) route(a, randomSpot(true));
            else if (!a.sitting) route(a, { x: a.seat.x, y: a.seat.y });
            a.wait = 4 + Math.random() * 8;
          } else {
            route(a, randomSpot(a.inside));
          }
        }
      }
    });
    if (t > nextTalk) {
      // The most prestigious member addresses the agora from the platform;
      // the others greet each other.
      if (orator && orator.sitting && Math.random() < 0.35) {
        route(orator, { x: BEMA.x, y: BEMA.y + 1 });
        orator.wait = 6;
        say(orator, txt.speech, 4);
      } else {
        var talkers = actors.filter(function (a) { return a !== me; });
        if (talkers.length) say(talkers[Math.floor(Math.random() * talkers.length)], txt.hello, 2.5);
      }
      nextTalk = t + 5000 + Math.random() * 5000;
    }
    frame(t);
    if (!document.hidden) requestAnimationFrame(step);
  }

  // The scenery is drawn at the current scale, capped so a big zoom on a
  // big screen doesn't make a huge offscreen image.
  function redrawScenery() {
    var s = Math.min(cam.scale, 4096 / W);
    bg.width = Math.ceil(W * s); bg.height = Math.ceil(H * s);
    bgScale = s;
    drawBackground();
    if (still) frame(performance.now());
  }
  var resized = false;

  // The canvas takes its size from the page layout (the whole screen under
  // the top bar); the map is scaled to cover it.
  function resize() {
    var dpr = window.devicePixelRatio || 1;
    canvas.width = Math.round(canvas.clientWidth * dpr);
    canvas.height = Math.round(canvas.clientHeight * dpr);
    // Zoomed in past "cover", so the map is wider AND taller than the
    // screen: the camera (and a drag) can move every way to see it all.
    cam.base = Math.max(canvas.width / W, canvas.height / H);
    cam.zoomMin = Math.min(canvas.width / W, canvas.height / H) / cam.base;
    if (!resized) cam.zoom = canvas.clientWidth < 700 ? 1.6 : 1.3;
    resized = true;
    cam.scale = cam.base * cam.zoom;
    redrawScenery();
    updateCamera(true);
    frame(performance.now());
  }

  window.addEventListener('resize', resize);
  resize();
  if (!still) {
    requestAnimationFrame(step);
    document.addEventListener('visibilitychange', function () {
      if (!document.hidden) { last = performance.now(); requestAnimationFrame(step); }
    });
  }
})();
