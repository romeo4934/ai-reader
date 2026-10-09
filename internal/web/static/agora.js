// The Lydi Agora as a little top-down 2D game, in a retro pixel-art style:
// the walled agora with its temple, the speakers' platform and stone tiers
// where members sit in order of prestige, and outside the market, the
// meadow and the square where aspirants wait at the gate. The viewer walks
// wherever they tap. Everything is drawn in code (no image assets) onto a
// half-resolution canvas scaled up without smoothing; names and speech
// bubbles are drawn on top at full resolution. Day, dusk and night follow
// the reader's clock. On e-ink (or with reduced motion) it's a still frame.
(function () {
  'use strict';
  var canvas = document.getElementById('agora-game');
  if (!canvas) return;
  var people = JSON.parse(document.getElementById('agora-people').textContent || '[]');
  var txt = canvas.dataset;
  var W = 480, H = 400, PX = 2; // world size; world units per pixel of the art
  var still = document.documentElement.hasAttribute('data-eink') ||
    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);

  var view = canvas.getContext('2d');
  var art = document.createElement('canvas'); art.width = W / PX; art.height = H / PX;
  var g = art.getContext('2d');
  var bg = document.createElement('canvas'); bg.width = W / PX; bg.height = H / PX;
  [view, g, bg.getContext('2d')].forEach(function (c) {
    if (!c.roundRect) c.roundRect = function (x, y, w, h) { this.rect(x, y, w, h); }; // older browsers
  });

  // --- layout (world units) ---
  var WALL = { x0: 60, y0: 40, x1: 420, y1: 300, t: 8 };
  var GATE = { x0: 214, x1: 266 };
  var TEMPLE = { x0: 168, y0: 50, x1: 312, y1: 100 };
  var BEMA = { x: 240, y: 122 };
  var TIERS = { cx: 240, cy: 118, r: [44, 64, 84, 104, 124, 144], from: 0.26, to: Math.PI - 0.26 };
  var STATUE = { x: 100, y: 262 }, FOUNTAIN = { x: 380, y: 262, r: 16 };
  var BRAZIERS = [{ x: 198, y: 282 }, { x: 282, y: 282 }, { x: 150, y: 108 }, { x: 330, y: 108 }];
  var treesOut = [[22, 372], [458, 372], [30, 316], [450, 318]];
  var obstaclesOut = treesOut.map(function (t) { return { x: t[0], y: t[1], r: 15 }; })
    .concat([{ x: 290, y: 352, r: 8 }, { x: 128, y: 386, r: 8 }, { x: 362, y: 388, r: 8 }]);
  var obstaclesIn = [{ x: STATUE.x, y: STATUE.y, r: 14 }, { x: FOUNTAIN.x, y: FOUNTAIN.y, r: 20 },
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
  function makeSeats() {
    var out = [];
    TIERS.r.forEach(function (r, tier) {
      var n = Math.floor((TIERS.to - TIERS.from) * r / 17), row = [];
      for (var k = 0; k < n; k++) {
        var a = TIERS.from + (TIERS.to - TIERS.from) * (k + 0.5) / n;
        row.push({ x: TIERS.cx + Math.cos(a) * r, y: TIERS.cy + Math.sin(a) * r, tier: tier, mid: Math.abs(a - Math.PI / 2) });
      }
      row.sort(function (a, b) { return a.mid - b.mid; });
      out = out.concat(row);
    });
    return out;
  }
  var seatList = makeSeats();

  // --- characters ---
  var actors = [];
  people.filter(function (p) { return p.member; })
    .sort(function (a, b) { return b.days - a.days; })
    .forEach(function (p, i) {
      var seat = seatList[i];
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

  canvas.addEventListener('click', function (e) {
    var r = canvas.getBoundingClientRect();
    var p = { x: (e.clientX - r.left) * W / r.width, y: (e.clientY - r.top) * H / r.height };
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
    c.save(); c.scale(1 / PX, 1 / PX);
    function rect(x, y, w, h, col) { c.fillStyle = col; c.fillRect(x, y, w, h); }
    function circle(x, y, r, col) { c.fillStyle = col; c.beginPath(); c.arc(x, y, r, 0, Math.PI * 2); c.fill(); }
    var x, y, i, k;
    // grass in two tones, tufts and flowers
    rect(0, 0, W, H, '#7fb24f');
    for (y = 0; y < H; y += 8) for (x = 0; x < W; x += 8) if ((x * 7 + y * 13) % 5 === 0) rect(x, y, 8, 8, '#76a849');
    for (i = 0; i < 260; i++) rect((i * 97) % W, (i * 61 + (i % 7) * 13) % H, 2, 4, '#5f9a3a');
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
    for (var t = TIERS.r.length - 1; t >= 0; t--) {
      c.strokeStyle = t % 2 ? '#cbb68b' : '#d8c59c'; c.lineWidth = 19;
      c.beginPath(); c.arc(TIERS.cx, TIERS.cy, TIERS.r[t], TIERS.from - 0.05, TIERS.to + 0.05); c.stroke();
      c.strokeStyle = 'rgba(120,95,55,0.35)'; c.lineWidth = 1.5;
      c.beginPath(); c.arc(TIERS.cx, TIERS.cy, TIERS.r[t] + 9.5, TIERS.from - 0.05, TIERS.to + 0.05); c.stroke();
    }
    // the speakers' platform
    rect(BEMA.x - 18, BEMA.y - 7, 36, 14, '#bba57a'); rect(BEMA.x - 16, BEMA.y - 7, 32, 3, '#d7c49c');
    // statue of Croesus with his crown, a fountain, planters, amphorae
    rect(STATUE.x - 10, STATUE.y - 2, 20, 14, '#b9ab90'); rect(STATUE.x - 10, STATUE.y - 2, 20, 3, '#d3c7ae');
    circle(STATUE.x, STATUE.y - 8, 6, '#d9d2c3'); circle(STATUE.x, STATUE.y - 15, 4, '#e6dfd1'); rect(STATUE.x - 4, STATUE.y - 20, 8, 2, '#d9b84c');
    circle(FOUNTAIN.x, FOUNTAIN.y, FOUNTAIN.r, '#c4b083'); circle(FOUNTAIN.x, FOUNTAIN.y, FOUNTAIN.r - 4, '#5aa8d6');
    [[82, 60], [398, 60], [82, 200], [398, 200]].forEach(function (p) {
      rect(p[0] - 8, p[1] - 6, 16, 12, '#9b6b3c');
      circle(p[0] - 3, p[1] - 4, 3, '#e85d75'); circle(p[0] + 3, p[1] - 5, 3, '#f4d35e'); circle(p[0], p[1] - 1, 3, '#5a9a3c');
    });
    [[140, 64], [146, 74], [334, 64], [340, 76]].forEach(function (p) { circle(p[0], p[1], 4, '#b5603a'); rect(p[0] - 1, p[1] - 7, 2, 3, '#8a4527'); });
    c.restore();
  }

  // --- per-frame art ---
  function rect(x, y, w, h, col) { g.fillStyle = col; g.fillRect(x, y, w, h); }
  function circle(x, y, r, col) { g.fillStyle = col; g.beginPath(); g.arc(x, y, r, 0, Math.PI * 2); g.fill(); }

  // The sash shows the rank: citizen, orator, philosopher, sage.
  var RANK_SASH = ['#f4efe2', '#3b7dd8', '#7d4bb3', '#e2b93b'];
  function rankTier(days) { return days >= 365 ? 3 : days >= 250 ? 2 : days >= 100 ? 1 : 0; }

  function drawPerson(a, t) {
    var x = a.pos.x, y = a.pos.y, walking = a.path && a.path.length > 0 && !still;
    var swing = walking ? Math.sin(t / 90 + a.phase) : 0, sit = a.sitting ? 2 : 0;
    circle(x, y + 7, 6, 'rgba(0,0,0,0.2)');
    if (!a.sitting) {
      rect(x - 3, y + 2 + Math.max(0, swing) * 2, 2, 5, '#5a3d22');
      rect(x + 1, y + 2 + Math.max(0, -swing) * 2, 2, 5, '#5a3d22');
    }
    g.fillStyle = a.guard ? '#8b6b3e' : a.p.color; g.beginPath(); g.roundRect(x - 5, y - 5 + sit, 10, 9, 3); g.fill();
    if (a.p && a.p.member) {
      g.strokeStyle = RANK_SASH[rankTier(a.p.days)]; g.lineWidth = 2;
      g.beginPath(); g.moveTo(x - 4, y - 4 + sit); g.lineTo(x + 4, y + 3 + sit); g.stroke();
    }
    circle(x, y - 9 + sit, 4, '#f0cfa8');
    circle(x, y - 11 + sit, 3, '#4a2f1c');
    if (a.p && a.p.laurel) { g.strokeStyle = '#7bbf4a'; g.lineWidth = 1.6; g.beginPath(); g.arc(x, y - 9 + sit, 4.5, Math.PI * 1.05, Math.PI * 1.95); g.stroke(); }
    if (a.guard) {
      g.fillStyle = '#b08d3c'; g.beginPath(); g.arc(x, y - 10, 4.5, Math.PI, 0); g.fill();
      rect(x - 1, y - 18, 2, 5, '#c0392b');
      rect(x + 7, y - 16, 1.5, 24, '#7a5530'); rect(x + 6, y - 18, 3.5, 3, '#c9c9c9');
      circle(x - 6, y + 1, 4.5, '#b08d3c'); circle(x - 6, y + 1, 2, '#8a6a2a');
    }
  }

  function drawWorld(t) {
    g.setTransform(1, 0, 0, 1, 0, 0);
    g.drawImage(bg, 0, 0);
    g.setTransform(1 / PX, 0, 0, 1 / PX, 0, 0);
    // water: river glints, fountain jet
    for (var x = 0; x < W; x += 30) rect((x + t / 40) % W, 16 + Math.sin(x / 40) * 3, 6, 1.5, 'rgba(255,255,255,0.55)');
    var jet = 2 + (Math.sin(t / 200) + 1) * 3;
    circle(FOUNTAIN.x, FOUNTAIN.y, jet + 3, 'rgba(255,255,255,0.35)'); circle(FOUNTAIN.x, FOUNTAIN.y, 2.5, '#eaf6fc');
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
    // people, back to front
    actors.concat([guard]).sort(function (a, b) { return a.pos.y - b.pos.y; }).forEach(function (a) { drawPerson(a, t); });
    // butterflies
    butterflies.forEach(function (b) {
      if (!still) { b.s += 0.02; b.x = (b.x + 0.4) % W; }
      var by = b.y + Math.sin(b.s * 3) * 6, w = Math.abs(Math.sin(b.s * 20)) * 2.5 + 0.5;
      rect(b.x - w, by, w, 2, b.c); rect(b.x + 0.5, by, w, 2, b.c);
    });
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
  function drawOverlay(t) {
    var s = canvas.width / W;
    view.setTransform(s, 0, 0, s, 0, 0);
    view.textAlign = 'center';
    view.lineJoin = 'round'; // no miter spikes on outlined text
    actors.forEach(function (a) {
      var name = a.p.name.length > 9 ? a.p.name.slice(0, 8) + '…' : a.p.name;
      view.font = (a.p.you ? 'bold ' : '') + '7px sans-serif';
      view.lineWidth = 2.5; view.strokeStyle = 'rgba(255,255,255,0.75)';
      view.strokeText(name, a.pos.x, a.pos.y + 17);
      view.fillStyle = '#2b2416'; view.fillText(name, a.pos.x, a.pos.y + 17);
      if (a.p.you) {
        var b = still ? 0 : Math.sin(t / 200) * 2;
        view.fillStyle = '#2f6f4f';
        view.beginPath(); view.moveTo(a.pos.x - 4, a.pos.y - 22 + b); view.lineTo(a.pos.x + 4, a.pos.y - 22 + b); view.lineTo(a.pos.x, a.pos.y - 17 + b); view.fill();
      }
    });
    view.font = 'bold 8px Georgia, serif'; view.fillStyle = '#5b3a22'; view.fillText('ΛΥΔΙ', 240, 68);
    view.font = '6px Georgia, serif'; view.fillStyle = '#4a3418'; view.fillText('ΣΑΡΔΕΙΣ', 289, 345);
    [guard].concat(actors).forEach(function (a) {
      if (!a.bubble || t > a.bubble.until) return;
      view.font = '8px sans-serif';
      var w = Math.min(view.measureText(a.bubble.text).width + 12, 220);
      var x = Math.max(4, Math.min(W - w - 4, a.pos.x - w / 2)), y = a.pos.y - 40;
      view.fillStyle = 'rgba(255,253,245,0.96)'; view.beginPath(); view.roundRect(x, y, w, 15, 5); view.fill();
      view.strokeStyle = 'rgba(60,40,10,0.35)'; view.lineWidth = 0.8; view.stroke();
      view.fillStyle = '#2b2416'; view.textAlign = 'left'; view.fillText(a.bubble.text, x + 6, y + 10.5, w - 12); view.textAlign = 'center';
    });
  }

  function frame(t) {
    drawWorld(t);
    view.setTransform(1, 0, 0, 1, 0, 0);
    view.imageSmoothingEnabled = false;
    view.drawImage(art, 0, 0, canvas.width, canvas.height);
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

  function resize() {
    var dpr = window.devicePixelRatio || 1, w = canvas.clientWidth;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(w * H / W * dpr);
    canvas.style.height = (w * H / W) + 'px';
    frame(performance.now());
  }

  drawBackground();
  window.addEventListener('resize', resize);
  resize();
  if (!still) {
    requestAnimationFrame(step);
    document.addEventListener('visibilitychange', function () {
      if (!document.hidden) { last = performance.now(); requestAnimationFrame(step); }
    });
  }
})();
