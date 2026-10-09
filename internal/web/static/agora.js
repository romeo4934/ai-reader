// The Lydi Agora as a little top-down 2D game: the walled square with its
// temple and fountain, members strolling inside, aspirants outside, and the
// viewer's own character walking wherever they tap. Everything is drawn
// on a canvas — no image assets. On e-ink (or with reduced motion) it's a
// single still frame.
(function () {
  'use strict';
  var canvas = document.getElementById('agora-game');
  if (!canvas) return;
  var people = JSON.parse(document.getElementById('agora-people').textContent || '[]');
  var ctx = canvas.getContext('2d');
  if (!ctx.roundRect) ctx.roundRect = function (x, y, w, h) { this.rect(x, y, w, h); }; // older browsers
  var W = 480, H = 400;
  var still = document.documentElement.hasAttribute('data-eink') ||
    (window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  var txt = canvas.dataset;

  // --- map ---
  var WALL = { x0: 60, y0: 40, x1: 420, y1: 300, t: 8 };
  var GATE = { x0: 214, x1: 266 };
  var TEMPLE = { x0: 150, y0: 52, x1: 330, y1: 118 };
  var FOUNTAIN = { x: 240, y: 205, r: 28 };
  var treesIn = [[92, 76], [388, 76], [92, 272], [388, 272]];
  var treesOut = [[24, 70], [30, 190], [452, 120], [448, 250], [40, 340], [440, 360], [120, 372], [360, 380]];
  var obstacles = [{ x: FOUNTAIN.x, y: FOUNTAIN.y, r: FOUNTAIN.r + 8 }]
    .concat(treesIn.map(function (t) { return { x: t[0], y: t[1], r: 18 }; }))
    .concat(treesOut.map(function (t) { return { x: t[0], y: t[1], r: 16 }; }));
  var GUARD = { x: 200, y: 318 };

  function inside(p) { return p.x > WALL.x0 + WALL.t && p.x < WALL.x1 - WALL.t && p.y > WALL.y0 + WALL.t && p.y < WALL.y1; }
  function walkable(p, isInside) {
    if (isInside) {
      if (p.x < 76 || p.x > 404 || p.y < 132 || p.y > 288) return false;
    } else {
      // Outside means the square in front of the gate: walking straight
      // between two spots there never cuts through the walls.
      if (p.x < 10 || p.x > 470 || p.y < 318 || p.y > 390) return false;
    }
    for (var i = 0; i < obstacles.length; i++) {
      var o = obstacles[i];
      if (Math.hypot(p.x - o.x, p.y - o.y) < o.r) return false;
    }
    return true;
  }
  function segmentClear(a, b) {
    for (var t = 0.1; t < 1; t += 0.1) {
      var p = { x: a.x + (b.x - a.x) * t, y: a.y + (b.y - a.y) * t };
      for (var i = 0; i < obstacles.length; i++) {
        if (Math.hypot(p.x - obstacles[i].x, p.y - obstacles[i].y) < obstacles[i].r - 2) return false;
      }
    }
    return true;
  }
  function randomSpot(from, isInside) {
    for (var i = 0; i < 60; i++) {
      var p = { x: 10 + Math.random() * 460, y: 12 + Math.random() * 378 };
      if (walkable(p, isInside) && (!from || segmentClear(from, p))) return p;
    }
    return from || { x: 240, y: isInside ? 250 : 350 };
  }

  // --- characters ---
  var actors = people.map(function (p) {
    var a = {
      p: p, inside: p.member, path: [], wait: Math.random() * 3,
      speed: 18 + Math.random() * 12, phase: Math.random() * 6, bubble: null,
    };
    a.pos = randomSpot(null, a.inside);
    return a;
  });
  var me = actors.filter(function (a) { return a.p.you; })[0];
  var guard = { pos: GUARD, bubble: null };

  function say(who, text, secs) { who.bubble = { text: text, until: performance.now() + (secs || 3) * 1000 }; }

  // Through the gate when crossing the wall.
  function route(a, target) {
    var tIn = inside(target);
    var path = [];
    if (a.inside && !tIn) path.push({ x: 240, y: 292 }, { x: 240, y: 322 });
    if (!a.inside && tIn) path.push({ x: 240, y: 322 }, { x: 240, y: 292 });
    path.push(target);
    a.path = path;
  }

  canvas.addEventListener('click', function (e) {
    var r = canvas.getBoundingClientRect();
    var p = { x: (e.clientX - r.left) * W / r.width, y: (e.clientY - r.top) * H / r.height };
    // A tap on someone shows who they are.
    for (var i = 0; i < actors.length; i++) {
      var a = actors[i];
      if (a !== me && Math.hypot(a.pos.x - p.x, a.pos.y - 6 - p.y) < 13) {
        say(a, a.p.member ? a.p.name + ' · ' + a.p.rank + ' · ' + txt.days.replace('%d', a.p.days)
          : a.p.name + ' · ' + a.p.days + ' / ' + txt.threshold);
        draw(performance.now());
        return;
      }
    }
    if (Math.hypot(guard.pos.x - p.x, guard.pos.y - 6 - p.y) < 13) {
      say(guard, me && !me.p.member ? txt.guard.replace('%d', Number(txt.threshold) - me.p.days) : txt.hello);
      draw(performance.now());
      return;
    }
    if (!me) return;
    var tIn = inside(p);
    if (tIn && !me.p.member) {
      // The guard keeps aspirants out — with a word of encouragement.
      route(me, { x: 236, y: 330 });
      say(guard, txt.guard.replace('%d', Number(txt.threshold) - me.p.days), 3.5);
      return;
    }
    if (!walkable(p, tIn)) p = randomSpot(p, tIn);
    route(me, p);
    me.wait = 0;
  });

  // --- drawing ---
  function rr(x, y, w, h, r, fill) { ctx.fillStyle = fill; ctx.beginPath(); ctx.roundRect(x, y, w, h, r); ctx.fill(); }

  function drawMap(t) {
    // grass
    ctx.fillStyle = '#9cc46a'; ctx.fillRect(0, 0, W, H);
    ctx.fillStyle = '#8fb95f';
    for (var i = 0; i < 140; i++) {
      var gx = (i * 73) % W, gy = (i * 151) % H;
      ctx.fillRect(gx, gy, 2, 2);
    }
    // path to the gate
    rr(222, 300, 36, 100, 4, '#e3cf9e');
    // paving
    ctx.fillStyle = '#eadfc4'; ctx.fillRect(WALL.x0, WALL.y0, WALL.x1 - WALL.x0, WALL.y1 - WALL.y0);
    ctx.strokeStyle = 'rgba(160,140,100,0.18)'; ctx.lineWidth = 1;
    for (var x = WALL.x0; x < WALL.x1; x += 20) { ctx.beginPath(); ctx.moveTo(x, WALL.y0); ctx.lineTo(x, WALL.y1); ctx.stroke(); }
    for (var y = WALL.y0; y < WALL.y1; y += 20) { ctx.beginPath(); ctx.moveTo(WALL.x0, y); ctx.lineTo(WALL.x1, y); ctx.stroke(); }
    // walls, with the gate gap
    ctx.fillStyle = '#c4a978';
    ctx.fillRect(WALL.x0, WALL.y0, WALL.x1 - WALL.x0, WALL.t);
    ctx.fillRect(WALL.x0, WALL.y0, WALL.t, WALL.y1 - WALL.y0);
    ctx.fillRect(WALL.x1 - WALL.t, WALL.y0, WALL.t, WALL.y1 - WALL.y0);
    ctx.fillRect(WALL.x0, WALL.y1 - WALL.t, GATE.x0 - WALL.x0, WALL.t);
    ctx.fillRect(GATE.x1, WALL.y1 - WALL.t, WALL.x1 - GATE.x1, WALL.t);
    // gate pillars
    rr(GATE.x0 - 8, WALL.y1 - 14, 12, 18, 2, '#b2955f');
    rr(GATE.x1 - 4, WALL.y1 - 14, 12, 18, 2, '#b2955f');
    // temple, seen from above: two roof slopes, a ridge, columns in front
    rr(TEMPLE.x0 - 8, TEMPLE.y1 - 4, TEMPLE.x1 - TEMPLE.x0 + 16, 18, 3, '#d9c9a2');
    ctx.fillStyle = '#c2683f'; ctx.fillRect(TEMPLE.x0, TEMPLE.y0, TEMPLE.x1 - TEMPLE.x0, (TEMPLE.y1 - TEMPLE.y0) / 2);
    ctx.fillStyle = '#a9532f'; ctx.fillRect(TEMPLE.x0, (TEMPLE.y0 + TEMPLE.y1) / 2, TEMPLE.x1 - TEMPLE.x0, (TEMPLE.y1 - TEMPLE.y0) / 2);
    ctx.strokeStyle = 'rgba(0,0,0,0.12)';
    for (var rx = TEMPLE.x0 + 10; rx < TEMPLE.x1; rx += 10) { ctx.beginPath(); ctx.moveTo(rx, TEMPLE.y0); ctx.lineTo(rx, TEMPLE.y1); ctx.stroke(); }
    ctx.fillStyle = '#e9d9b6'; ctx.fillRect(TEMPLE.x0, (TEMPLE.y0 + TEMPLE.y1) / 2 - 2, TEMPLE.x1 - TEMPLE.x0, 4);
    for (var c = 0; c < 8; c++) {
      ctx.fillStyle = '#f4ecda';
      ctx.beginPath(); ctx.arc(TEMPLE.x0 + 8 + c * ((TEMPLE.x1 - TEMPLE.x0 - 16) / 7), TEMPLE.y1 + 6, 5, 0, Math.PI * 2); ctx.fill();
    }
    ctx.fillStyle = '#7a5a2a'; ctx.font = 'bold 9px Georgia, serif'; ctx.textAlign = 'center';
    ctx.fillText('ΛΥΔΙ', 240, (TEMPLE.y0 + TEMPLE.y1) / 2 - 8);
    // fountain
    ctx.fillStyle = '#c9b88f'; ctx.beginPath(); ctx.arc(FOUNTAIN.x, FOUNTAIN.y, FOUNTAIN.r, 0, Math.PI * 2); ctx.fill();
    ctx.fillStyle = '#6fb3d6'; ctx.beginPath(); ctx.arc(FOUNTAIN.x, FOUNTAIN.y, FOUNTAIN.r - 6, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = 'rgba(255,255,255,0.7)'; ctx.lineWidth = 1.2;
    for (var k = 0; k < 2; k++) {
      var rad = ((t / 60 + k * 11) % 22);
      ctx.globalAlpha = 1 - rad / 22;
      ctx.beginPath(); ctx.arc(FOUNTAIN.x, FOUNTAIN.y, 3 + rad, 0, Math.PI * 2); ctx.stroke();
    }
    ctx.globalAlpha = 1;
    ctx.fillStyle = '#e8e1cf'; ctx.beginPath(); ctx.arc(FOUNTAIN.x, FOUNTAIN.y, 4, 0, Math.PI * 2); ctx.fill();
    // benches
    rr(150, 238, 26, 6, 2, '#a07a4a'); rr(304, 238, 26, 6, 2, '#a07a4a');
    // trees
    treesIn.concat(treesOut).forEach(function (tr, i) {
      ctx.fillStyle = 'rgba(0,0,0,0.15)'; ctx.beginPath(); ctx.ellipse(tr[0] + 3, tr[1] + 8, 15, 7, 0, 0, Math.PI * 2); ctx.fill();
      ctx.fillStyle = i < 4 ? '#6f8b45' : '#5f8a3f';
      ctx.beginPath(); ctx.arc(tr[0], tr[1], 14, 0, Math.PI * 2); ctx.fill();
      ctx.fillStyle = 'rgba(255,255,255,0.12)'; ctx.beginPath(); ctx.arc(tr[0] - 4, tr[1] - 4, 6, 0, Math.PI * 2); ctx.fill();
    });
  }

  function drawPerson(pos, color, symbol, label, opts, t) {
    var bob = opts.walking && !still ? Math.sin(t / 110 + opts.phase) * 1.2 : 0;
    var x = pos.x, y = pos.y + bob;
    ctx.fillStyle = 'rgba(0,0,0,0.18)'; ctx.beginPath(); ctx.ellipse(pos.x, pos.y + 7, 7, 3, 0, 0, Math.PI * 2); ctx.fill();
    // body (toga) and head
    ctx.fillStyle = color; ctx.beginPath(); ctx.arc(x, y, 7, 0, Math.PI * 2); ctx.fill();
    ctx.strokeStyle = opts.laurel ? '#e2b93b' : 'rgba(255,255,255,0.75)'; ctx.lineWidth = opts.laurel ? 2 : 1;
    ctx.stroke();
    ctx.fillStyle = '#f1d3b0'; ctx.beginPath(); ctx.arc(x, y - 9, 4.2, 0, Math.PI * 2); ctx.fill();
    if (opts.helmet) {
      ctx.fillStyle = '#9a7b2f'; ctx.beginPath(); ctx.arc(x, y - 10, 4.6, Math.PI, 0); ctx.fill();
      ctx.fillStyle = '#c0392b'; ctx.fillRect(x - 1, y - 18, 2, 5);
    }
    ctx.fillStyle = '#fff'; ctx.font = '7px Georgia, serif'; ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
    ctx.fillText(symbol, x, y + 0.5);
    ctx.textBaseline = 'alphabetic';
    if (label) {
      ctx.font = (opts.you ? 'bold ' : '') + '7px sans-serif';
      ctx.fillStyle = 'rgba(40,30,10,0.85)';
      ctx.fillText(label, x, y + 18);
    }
    if (opts.you) {
      var b = still ? 0 : Math.sin(t / 200) * 2;
      ctx.fillStyle = '#2f6f4f';
      ctx.beginPath(); ctx.moveTo(x - 4, y - 22 + b); ctx.lineTo(x + 4, y - 22 + b); ctx.lineTo(x, y - 17 + b); ctx.fill();
    }
  }

  function drawBubble(pos, text) {
    ctx.font = '8px sans-serif';
    var w = Math.min(ctx.measureText(text).width + 12, 200);
    var x = Math.max(4, Math.min(W - w - 4, pos.x - w / 2)), y = pos.y - 40;
    rr(x, y, w, 15, 5, 'rgba(255,255,255,0.95)');
    ctx.strokeStyle = 'rgba(0,0,0,0.25)'; ctx.lineWidth = 0.8; ctx.beginPath(); ctx.roundRect(x, y, w, 15, 5); ctx.stroke();
    ctx.fillStyle = '#2b2416'; ctx.textAlign = 'left'; ctx.fillText(text, x + 6, y + 10.5, w - 12);
  }

  function draw(t) {
    ctx.setTransform(canvas.width / W, 0, 0, canvas.height / H, 0, 0);
    drawMap(t);
    var all = actors.slice().sort(function (a, b) { return a.pos.y - b.pos.y; });
    drawPerson(guard.pos, '#8b6b3e', '⚔', txt.guardName, { helmet: true, phase: 0 }, t);
    all.forEach(function (a) {
      drawPerson(a.pos, a.p.color, a.p.symbol, a.p.name.length > 9 ? a.p.name.slice(0, 8) + '…' : a.p.name,
        { walking: a.path.length > 0, phase: a.phase, laurel: a.p.laurel, you: a.p.you }, t);
    });
    [guard].concat(actors).forEach(function (a) {
      if (a.bubble && t < a.bubble.until) drawBubble(a.pos, a.bubble.text);
    });
  }

  // --- simulation ---
  var last = performance.now(), nextHello = last + 4000;
  function step(t) {
    var dt = Math.min(0.05, (t - last) / 1000);
    last = t;
    actors.forEach(function (a) {
      if (a.path.length) {
        var target = a.path[0];
        var dx = target.x - a.pos.x, dy = target.y - a.pos.y, d = Math.hypot(dx, dy);
        var v = (a === me ? 45 : a.speed) * dt;
        if (d <= v) {
          a.pos = { x: target.x, y: target.y };
          a.path.shift();
          a.inside = inside(a.pos);
          if (!a.path.length) a.wait = 1 + Math.random() * 4;
        } else {
          a.pos = { x: a.pos.x + dx / d * v, y: a.pos.y + dy / d * v };
        }
      } else if (a !== me) {
        a.wait -= dt;
        if (a.wait <= 0) route(a, randomSpot(a.pos, a.inside));
      }
    });
    if (t > nextHello && actors.length) {
      var members = actors.filter(function (a) { return a.p.member && a !== me; });
      var who = (members.length ? members : actors)[Math.floor(Math.random() * (members.length || actors.length))];
      if (who !== me) say(who, txt.hello, 2.5);
      nextHello = t + 5000 + Math.random() * 5000;
    }
    draw(t);
    if (!document.hidden) requestAnimationFrame(step);
  }

  function resize() {
    var dpr = window.devicePixelRatio || 1;
    var w = canvas.clientWidth;
    canvas.width = Math.round(w * dpr);
    canvas.height = Math.round(w * H / W * dpr);
    canvas.style.height = (w * H / W) + 'px';
    draw(performance.now());
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
