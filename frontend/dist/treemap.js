// Squarified, nested treemap drawn on a canvas.
class Treemap {
  constructor(canvas, opts) {
    this.c = canvas;
    this.ctx = canvas.getContext("2d");
    this.opts = opts; // { color(node, depth, branch), label(node), onHover(node|null, ev), onClick(node, ev), onDblClick(node, ev), onContext(node, ev) }
    this.root = null;
    this.rects = [];
    this.hover = null;
    this.selectedKey = null;
    const ro = new ResizeObserver(() => this.draw());
    ro.observe(canvas);
    canvas.addEventListener("mousemove", (e) => this._move(e));
    canvas.addEventListener("mouseleave", () => { this._setHover(null); this.opts.onHover(null); });
    canvas.addEventListener("click", (e) => { const n = this._hit(e); if (n) this.opts.onClick(n, e); });
    canvas.addEventListener("dblclick", (e) => { const n = this._hit(e); if (n) this.opts.onDblClick(n, e); });
    canvas.addEventListener("contextmenu", (e) => { e.preventDefault(); const n = this._hit(e); if (n) this.opts.onContext(n, e); });
  }

  setData(root) { this.root = root; this.draw(); }

  draw() {
    const dpr = window.devicePixelRatio || 1;
    const w = this.c.clientWidth, h = this.c.clientHeight;
    if (!w || !h) return;
    if (this.c.width !== Math.round(w * dpr) || this.c.height !== Math.round(h * dpr)) {
      this.c.width = Math.round(w * dpr);
      this.c.height = Math.round(h * dpr);
    }
    const ctx = this.ctx;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, w, h);
    this.rects = [];
    this.css = getComputedStyle(document.documentElement);
    if (!this.root || this.root.s <= 0) return;
    this._layoutNode(this.root, 0, 0, w, h, 0, null, true);
    this._paintOverlay();
  }

  // --- layout ---------------------------------------------------------
  _layoutNode(node, x, y, w, h, depth, branch, isRoot) {
    if (w < 1 || h < 1) return;
    if (!isRoot) this.rects.push({ node, x, y, w, h, depth, branch });
    const kids = node.c;
    if (!kids || !kids.length) { if (!isRoot) this._paintTile(node, x, y, w, h, depth, branch); return; }

    let ix = x, iy = y, iw = w, ih = h;
    if (!isRoot) {
      // folder frame with a caption strip when there is room
      this._paintFolder(node, x, y, w, h, depth, branch);
      const head = (h > 44 && w > 70) ? 17 : 0;
      const pad = w > 12 && h > 12 ? 2 : 0;
      ix += pad; iy += pad + head; iw -= pad * 2; ih -= pad * 2 + head;
      if (iw < 2 || ih < 2) return;
    }
    const total = kids.reduce((a, k) => a + k.s, 0);
    if (total <= 0) return;
    const scale = (iw * ih) / total;
    const items = kids.filter((k) => k.s > 0).map((k) => ({ n: k, a: k.s * scale }));
    this._squarify(items, ix, iy, iw, ih, (it, rx, ry, rw, rh) => {
      const b = isRoot ? it.n : branch;
      this._layoutNode(it.n, rx, ry, rw, rh, depth + 1, b, false);
    });
  }

  _squarify(items, x, y, w, h, place) {
    let i = 0;
    while (i < items.length) {
      const short = Math.min(w, h);
      if (short <= 0) return;
      let row = [items[i]], sum = items[i].a, worst = this._worst(row, sum, short);
      let j = i + 1;
      while (j < items.length) {
        const ns = sum + items[j].a;
        row.push(items[j]);
        const nw = this._worst(row, ns, short);
        if (nw > worst) { row.pop(); break; }
        sum = ns; worst = nw; j++;
      }
      // lay out the row along the short side
      const thick = sum / short;
      let off = 0;
      for (const it of row) {
        const len = it.a / thick;
        if (w >= h) place(it, x, y + off, thick, len);
        else place(it, x + off, y, len, thick);
        off += len;
      }
      if (w >= h) { x += thick; w -= thick; } else { y += thick; h -= thick; }
      i = j;
    }
  }

  _worst(row, sum, short) {
    let mx = 0, mn = Infinity;
    for (const r of row) { if (r.a > mx) mx = r.a; if (r.a < mn) mn = r.a; }
    const s2 = sum * sum, w2 = short * short;
    return Math.max((w2 * mx) / s2, s2 / (w2 * mn));
  }

  // --- painting -------------------------------------------------------
  _paintTile(node, x, y, w, h, depth, branch) {
    const ctx = this.ctx;
    const col = this.opts.color(node, depth, branch);
    const gx = w > 3 ? 0.5 : 0, gy = h > 3 ? 0.5 : 0;
    ctx.fillStyle = col;
    ctx.fillRect(x + gx, y + gy, Math.max(w - 2 * gx, 0.5), Math.max(h - 2 * gy, 0.5));
    if (w > 10 && h > 10) {
      // soft light from the top left gives tiles some depth
      const g = ctx.createLinearGradient(x, y, x + w, y + h);
      g.addColorStop(0, "rgba(255,255,255,.18)");
      g.addColorStop(1, "rgba(0,0,0,.14)");
      ctx.fillStyle = g;
      ctx.fillRect(x + gx, y + gy, w - 2 * gx, h - 2 * gy);
    }
    if (node.r) { // aggregated small items: hatch
      ctx.save();
      ctx.beginPath(); ctx.rect(x, y, w, h); ctx.clip();
      ctx.strokeStyle = "rgba(255,255,255,.22)"; ctx.lineWidth = 1;
      for (let k = -h; k < w; k += 6) { ctx.beginPath(); ctx.moveTo(x + k, y + h); ctx.lineTo(x + k + h, y); ctx.stroke(); }
      ctx.restore();
    }
    if (w > 46 && h > 18) this._text(this.opts.label(node), x + 5, y + 13, w - 10, "rgba(255,255,255,.95)", 600);
    if (w > 46 && h > 34) this._text(this.opts.sizeText(node), x + 5, y + 27, w - 10, "rgba(255,255,255,.75)", 400);
  }

  _paintFolder(node, x, y, w, h, depth, branch) {
    const ctx = this.ctx;
    ctx.fillStyle = this.opts.folderColor(node, depth, branch);
    ctx.fillRect(x + 0.5, y + 0.5, Math.max(w - 1, 0.5), Math.max(h - 1, 0.5));
    if (h > 44 && w > 70) this._text(this.opts.label(node) + "  " + this.opts.sizeText(node), x + 5, y + 12.5, w - 10, this.opts.folderInk(depth), 600);
  }

  _text(str, x, y, maxW, color, weight) {
    const ctx = this.ctx;
    ctx.font = `${weight} 11px ${this.css.getPropertyValue("--font")}`;
    ctx.fillStyle = color;
    ctx.textBaseline = "alphabetic";
    if (ctx.measureText(str).width > maxW) {
      let lo = 0, hi = str.length;
      while (lo < hi) { const mid = (lo + hi + 1) >> 1; if (ctx.measureText(str.slice(0, mid) + "…").width <= maxW) lo = mid; else hi = mid - 1; }
      if (lo < 2) return;
      str = str.slice(0, lo) + "…";
    }
    ctx.fillText(str, x, y);
  }

  _paintOverlay() {
    const ctx = this.ctx;
    const mark = (r, color, width) => { ctx.strokeStyle = color; ctx.lineWidth = width; ctx.strokeRect(r.x + width / 2, r.y + width / 2, r.w - width, r.h - width); };
    if (this.selectedKey) {
      const r = this.rects.find((q) => q.node.k === this.selectedKey);
      if (r) { mark(r, "#ffffff", 3); mark(r, "rgba(0,0,0,.65)", 1); }
    }
    if (this.hover) mark(this.hover, "rgba(255,255,255,.9)", 2);
  }

  // --- interaction ----------------------------------------------------
  _hit(e) {
    const b = this.c.getBoundingClientRect();
    const x = e.clientX - b.left, y = e.clientY - b.top;
    for (let i = this.rects.length - 1; i >= 0; i--) {
      const r = this.rects[i];
      if (x >= r.x && x < r.x + r.w && y >= r.y && y < r.y + r.h) return r;
    }
    return null;
  }

  _move(e) {
    const r = this._hit(e);
    this._setHover(r);
    this.opts.onHover(r ? r.node : null, e);
  }

  _setHover(r) {
    if (r === this.hover || (r && this.hover && r.node === this.hover.node)) return;
    this.hover = r;
    this._repaintSoon();
  }

  setSelected(key) { this.selectedKey = key; this._repaintSoon(); }

  _repaintSoon() {
    if (this._raf) return;
    this._raf = requestAnimationFrame(() => { this._raf = 0; this.draw(); });
  }
}
