// Characters from the Liberated Pixel Cup sprites (static/lpc/, credited on
// /credits): each character is a stack of layers — body, head, clothes,
// hair, beard, headwear — composited once into its own spritesheets, with
// skin, hair and cloth colors applied by palette swap (each source sheet is
// drawn in its material's base palette, every pixel of which is mapped to
// the chosen palette). Sheets are 64×64 frames; rows face up, left, down,
// right; "walk" has 9 frames (the first one standing), "idle" 2, "sit" 3
// (on the ground, cross-legged, on a bench).
window.LPC = (function () {
  'use strict';
  var BASE = '/static/lpc/';
  var VERSION = '?v=1'; // bump when the sheets change: they are cached
  var ANIMS = ['walk', 'idle', 'sit'];
  var palettes = null, images = {}, recolored = {};

  function loadImage(src) {
    if (!images[src]) {
      images[src] = new Promise(function (resolve) {
        var img = new Image();
        img.onload = function () { resolve(img); };
        img.onerror = function () { resolve(null); };
        img.src = src;
      });
    }
    return images[src];
  }

  function hex(c) { return [parseInt(c.slice(1, 3), 16), parseInt(c.slice(3, 5), 16), parseInt(c.slice(5, 7), 16)]; }

  // A copy of the sheet with its material's base palette swapped for the
  // named one (cached).
  function recolor(img, key, material, name) {
    if (!material || !name) return img;
    var id = key + '|' + material + '|' + name;
    if (recolored[id]) return recolored[id];
    var pal = palettes[material];
    var from = pal.base.map(hex), to = (pal.colors[name] || pal.base).map(hex);
    var c = document.createElement('canvas');
    c.width = img.width; c.height = img.height;
    var x = c.getContext('2d');
    x.drawImage(img, 0, 0);
    var data = x.getImageData(0, 0, c.width, c.height), px = data.data;
    for (var i = 0; i < px.length; i += 4) {
      if (!px[i + 3]) continue;
      for (var k = 0; k < from.length; k++) {
        var f = from[k];
        if (Math.abs(px[i] - f[0]) <= 1 && Math.abs(px[i + 1] - f[1]) <= 1 && Math.abs(px[i + 2] - f[2]) <= 1) {
          px[i] = to[k][0]; px[i + 1] = to[k][1]; px[i + 2] = to[k][2];
          break;
        }
      }
    }
    x.putImageData(data, 0, 0);
    recolored[id] = c;
    return c;
  }

  // The layers of a character, back to front.
  //   spec = { body: 'male'|'female', skin, hair: 'plain'|'long'|'bun'|'curly'|'ponytail'|'',
  //            hairColor, beard: 'short'|'long'|'', tunic, cape (cloth color or ''),
  //            head: 'headband'|'petasos'|'olive'|'hood'|'laurel'|'crown'|'', guard: bool }
  // (the wreaths and the petasos aren't LPC items: they are drawn over the
  // LPC heads, frame by frame, in the same pixel style)
  function layers(spec) {
    var b = spec.body === 'female' ? 'female' : 'male', L = [];
    function add(id, z, material, color) { L.push({ id: id, z: z, material: material, color: color }); }
    if (spec.cape) add('cape_bg', 5, 'cloth', spec.cape);
    if (spec.hair === 'ponytail') add('hair_ponytail_bg', 9, 'hair', spec.hairColor);
    add('body_' + b, 10, 'body', spec.skin);
    add('sandals_' + b, 15, 'cloth', 'brown');
    add('skirt_' + b, 20, 'cloth', spec.guard ? 'red' : spec.tunic);
    add('top_' + b, 35, 'cloth', spec.guard ? 'red' : spec.tunic);
    if (spec.guard) add('armour_legion', 60, 'metal', 'bronze');
    if (spec.cape) add('cape_fg', 85, 'cloth', spec.cape);
    add('head_' + b, 100, 'body', spec.skin);
    if (spec.beard) add('beard_' + spec.beard, 110, 'hair', spec.hairColor);
    var hairId = { plain: 'hair_plain', long: 'hair_long', bun: 'hair_bun', curly: 'hair_curly', ponytail: 'hair_ponytail_fg' }[spec.hair];
    if (hairId && !spec.guard && spec.head !== 'hood') add(hairId, 120, 'hair', spec.hairColor);
    if (spec.head === 'headband') add('headband', 125, 'cloth', 'white');
    if (spec.head === 'hood') add('hood', 130, 'cloth', spec.tunic);
    if (spec.head === 'petasos') add('petasos_' + b, 130, null, null);
    if (spec.head === 'olive') add('wreath_olive_' + b, 130, null, null);
    if (spec.head === 'laurel') add('wreath_laurel_' + b, 130, null, null);
    if (spec.head === 'crown') add('crown_gold', 130, null, null);
    if (spec.guard) { add('helmet_legion', 130, 'metal', 'bronze'); add('crest_centurion', 139, 'metal', 'gold'); }
    return L.sort(function (a, b) { return a.z - b.z; });
  }

  // Composites a character's sheets: resolves to { walk, idle, sit }
  // canvases, or only the animations asked for (sit lacks the cape, which
  // has no sitting frames). Characters dressed alike share their sheets.
  var built = {};
  function build(spec, anims) {
    anims = anims || ANIMS;
    var key = JSON.stringify(spec) + anims.join();
    if (built[key]) return built[key];
    var L = layers(spec);
    built[key] = Promise.all(anims.map(function (anim) {
      return Promise.all(L.map(function (l) {
        if (anim === 'sit' && /^cape_/.test(l.id)) return null; // capes have no sitting frames
        return loadImage(BASE + l.id + '/' + anim + '.png' + VERSION);
      })).then(function (imgs) {
        var first = imgs.filter(Boolean)[0];
        var c = document.createElement('canvas');
        c.width = first ? first.width : 64; c.height = first ? first.height : 256;
        var x = c.getContext('2d');
        imgs.forEach(function (img, i) {
          if (img) x.drawImage(recolor(img, L[i].id + '/' + anim, L[i].material, L[i].color), 0, 0);
        });
        return c;
      });
    })).then(function (sheets) {
      var out = {};
      anims.forEach(function (a, i) { out[a] = sheets[i]; });
      return out;
    });
    return built[key];
  }

  function ready() {
    if (palettes) return Promise.resolve();
    return fetch(BASE + 'palettes.json' + VERSION).then(function (r) { return r.json(); }).then(function (p) { palettes = p; });
  }

  // Draws one frame of a sheet, `h` units for the 64-px frame, with the
  // feet at (x, y).
  var DIR_ROW = { up: 0, left: 1, down: 2, right: 3 };
  function drawFrame(ctx, sheet, dir, frame, x, y, h) {
    var s = h / 64;
    ctx.drawImage(sheet, frame * 64, DIR_ROW[dir] * 64, 64, 64, x - 32 * s, y - 62 * s, 64 * s, 64 * s);
  }

  // Draws part of a frame (sx, sy, sw, sh in frame pixels) to fill a box,
  // keeping proportions: the character editor's close-ups.
  function drawCrop(ctx, sheet, dir, frame, sx, sy, sw, sh, bx, by, bw, bh) {
    var k = Math.min(bw / sw, bh / sh), w = sw * k, h = sh * k;
    ctx.drawImage(sheet, frame * 64 + sx, DIR_ROW[dir] * 64 + sy, sw, sh, bx + (bw - w) / 2, by + (bh - h) / 2, w, h);
  }

  return { ready: ready, build: build, drawFrame: drawFrame, drawCrop: drawCrop };
})();
