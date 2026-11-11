"use strict";

// ---------------------------------------------------------------- helpers
const $ = (s, r = document) => r.querySelector(s);
const el = (tag, cls, html) => { const e = document.createElement(tag); if (cls) e.className = cls; if (html != null) e.innerHTML = html; return e; };
const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
const api = new Proxy({}, { get: (_, m) => (...a) => window.go.main.App[m](...a) });

const ICON = {
  folder: '<svg viewBox="0 0 16 16"><path fill="currentColor" d="M1.5 3.5A1.5 1.5 0 0 1 3 2h3.2l1.6 1.6H13a1.5 1.5 0 0 1 1.5 1.5v7.4A1.5 1.5 0 0 1 13 14H3a1.5 1.5 0 0 1-1.5-1.5z"/></svg>',
  file: '<svg viewBox="0 0 16 16"><path fill="currentColor" d="M4 1.5h5l3.5 3.5v8.5A1 1 0 0 1 11.5 14.5h-7.5a1 1 0 0 1-1-1V2.5a1 1 0 0 1 1-1zm4.5.8v3.2h3.2z"/></svg>',
  chevron: '<svg viewBox="0 0 12 12"><path d="M4.5 2.5 8 6l-3.5 3.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  pick: '<svg viewBox="0 0 20 20"><path d="M2.5 5.5a1.5 1.5 0 0 1 1.5-1.5h3.6l1.8 1.8H16a1.5 1.5 0 0 1 1.5 1.5v7.2A1.5 1.5 0 0 1 16 16H4a1.5 1.5 0 0 1-1.5-1.5z" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  drive: '<svg viewBox="0 0 20 20"><rect x="2.5" y="5" width="15" height="10" rx="2" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="14" cy="10" r="1.2" fill="currentColor"/><path d="M5 10h5" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/></svg>',
  driveBig: '<svg viewBox="0 0 32 32"><rect x="3" y="8" width="26" height="16" rx="3" fill="none" stroke="currentColor" stroke-width="2"/><circle cx="23" cy="16" r="1.8" fill="currentColor"/><path d="M8 16h8" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
  netBig: '<svg viewBox="0 0 32 32"><rect x="5" y="4" width="22" height="13" rx="2.5" fill="none" stroke="currentColor" stroke-width="2"/><path d="M16 17v6M6 27h20M16 23v4" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
  usbBig: '<svg viewBox="0 0 32 32"><rect x="10" y="12" width="12" height="17" rx="2.5" fill="none" stroke="currentColor" stroke-width="2"/><path d="M12 12V4h8v8M14 7v1M18 7v1" stroke="currentColor" stroke-width="2" stroke-linecap="round"/></svg>',
  export: '<svg viewBox="0 0 20 20"><path d="M10 3v9m0 0-3.5-3.5M10 12l3.5-3.5M4 13.5V16h12v-2.5" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  shield: '<svg viewBox="0 0 20 20"><path d="M10 2.5 4 5v4.6c0 3.6 2.5 6.6 6 7.9 3.5-1.3 6-4.3 6-7.9V5z" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linejoin="round"/></svg>',
  gear: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="2.6" fill="none" stroke="currentColor" stroke-width="1.6"/><path d="M10 2.5v2M10 15.5v2M17.5 10h-2M4.5 10h-2M15.3 4.7l-1.4 1.4M6.1 13.9l-1.4 1.4M15.3 15.3l-1.4-1.4M6.1 6.1 4.7 4.7" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/></svg>',
  open: '<svg viewBox="0 0 20 20"><path d="M11 3.5h5.5V9M16.5 3.5 9 11M14 12v4.5H3.5V6H8" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  copy: '<svg viewBox="0 0 20 20"><rect x="6.5" y="6.5" width="10" height="10" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M13.5 4V3.5a1 1 0 0 0-1-1h-9a1 1 0 0 0-1 1v9a1 1 0 0 0 1 1H4" fill="none" stroke="currentColor" stroke-width="1.5"/></svg>',
  refresh: '<svg viewBox="0 0 20 20"><path d="M16 10a6 6 0 1 1-1.8-4.3M16 3.5v3.2h-3.2" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  target: '<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="6" fill="none" stroke="currentColor" stroke-width="1.5"/><circle cx="10" cy="10" r="2" fill="currentColor"/></svg>',
  trash: '<svg viewBox="0 0 20 20"><path d="M4 5.5h12M8 5.5V4h4v1.5M5.5 5.5l.8 10.5h7.4l.8-10.5" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  map: '<svg viewBox="0 0 20 20"><rect x="3" y="3" width="14" height="14" rx="1.5" fill="none" stroke="currentColor" stroke-width="1.5"/><path d="M10 3v14M10 10h7" stroke="currentColor" stroke-width="1.5"/></svg>',
  expand: '<svg viewBox="0 0 20 20"><path d="M6 8l4 4 4-4" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  play: '<svg viewBox="0 0 20 20"><path d="M6 4.5v11l9-5.5z" fill="currentColor"/></svg>',
  stop: '<svg viewBox="0 0 20 20"><rect x="5" y="5" width="10" height="10" rx="1.5" fill="currentColor"/></svg>',
};

// ---------------------------------------------------------------- state
const S = {
  settings: null, lang: "de", drives: [], admin: false, platform: "windows", version: "",
  status: null, scanId: 0,
  expanded: new Set(), children: new Map(), flat: [],
  selected: null, focusDir: null, sort: { by: "size", desc: true },
  tab: "map", topFilter: "", lastSideRefresh: 0, polling: false,
};

// ---------------------------------------------------------------- i18n & formatting
function t(key, vars) {
  let s = (I18N[S.lang] && I18N[S.lang][key]) ?? I18N.en[key] ?? key;
  if (vars) for (const [k, v] of Object.entries(vars)) s = s.replaceAll(`{${k}}`, v);
  return s;
}
let NF, NF1, DF;
function setupFormats() {
  const loc = S.lang === "de" ? "de-DE" : "en-US";
  NF = new Intl.NumberFormat(loc);
  NF1 = new Intl.NumberFormat(loc, { minimumFractionDigits: 1, maximumFractionDigits: 1 });
  DF = new Intl.DateTimeFormat(loc, { dateStyle: "short", timeStyle: "short" });
}
const num = (n) => NF.format(n);
function fmtSize(b) {
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const fixed = S.settings.unit;
  if (fixed !== "auto") {
    const i = units.indexOf(fixed);
    const v = b / Math.pow(1024, i);
    return (v >= 100 ? NF.format(Math.round(v)) : NF1.format(v)) + " " + fixed;
  }
  if (b < 1024) return NF.format(b) + " B";
  let i = 0, v = b;
  while (v >= 1024 && i < units.length - 1) { v /= 1024; i++; }
  return (v >= 100 ? NF.format(Math.round(v)) : NF1.format(v)) + " " + units[i];
}
const fmtDate = (u) => (u > 0 ? DF.format(new Date(u * 1000)) : "");
function fmtDur(ms) {
  const s = Math.floor(ms / 1000), m = Math.floor(s / 60), h = Math.floor(m / 60);
  if (h) return `${h}:${String(m % 60).padStart(2, "0")}:${String(s % 60).padStart(2, "0")}`;
  return `${m}:${String(s % 60).padStart(2, "0")}`;
}
const fmtPct = (p) => (p >= 99.95 ? "100" : NF1.format(p)) + " %";
const sizeOf = (r) => (S.settings.sizeMode === "alloc" ? r.alloc : r.size);
// "lib.so.1.2" -> "so": trailing numeric parts are version numbers, not extensions
const extOf = (name) => {
  const parts = name.toLowerCase().split(".");
  if (parts.length < 2 || !parts[0]) return "";
  while (parts.length > 2 && /^\d+$/.test(parts[parts.length - 1])) parts.pop();
  const e = parts[parts.length - 1];
  return e.length <= 16 && !/^\d+$/.test(e) ? e : "";
};
const catOf = (ext) => EXT_CATEGORY[ext] || "other";
const cssVar = (n) => getComputedStyle(document.documentElement).getPropertyValue(n).trim();
const catColor = (cat) => cssVar("--t-" + cat);

function applyI18n() {
  document.documentElement.lang = S.lang;
  document.querySelectorAll("[data-i18n]").forEach((e) => (e.textContent = t(e.dataset.i18n)));
  document.querySelectorAll("[data-i18n-title]").forEach((e) => (e.title = t(e.dataset.i18nTitle)));
  document.querySelectorAll("[data-i18n-ph]").forEach((e) => (e.placeholder = t(e.dataset.i18nPh)));
}

function applyTheme() {
  const pref = S.settings.theme;
  const dark = pref === "dark" || (pref === "system" && matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.dataset.theme = dark ? "dark" : "light";
  map && map.draw();
}

function resolveLang() {
  const l = S.settings.language;
  S.lang = l === "auto" ? ((navigator.language || "en").toLowerCase().startsWith("de") ? "de" : "en") : l;
  setupFormats();
}

// ---------------------------------------------------------------- toasts & errors
function toast(msg, isErr) {
  const n = el("div", "toast" + (isErr ? " err" : ""), esc(msg));
  $("#toasts").append(n);
  setTimeout(() => n.remove(), isErr ? 6000 : 3200);
}
function showErr(e) {
  const m = String(e && e.message ? e.message : e);
  const map = { notadir: "eNotADir", busy: "eBusy", root: "eRoot" };
  toast(map[m] ? t(map[m]) : t("eGeneric", { msg: m }), true);
}

// ---------------------------------------------------------------- welcome screen
function renderWelcome() {
  const list = $("#driveList");
  list.innerHTML = "";
  for (const d of S.drives) {
    const used = d.total - d.free;
    const pct = d.total > 0 ? (used / d.total) * 100 : 0;
    const b = el("button", "drive");
    const icon = d.kind === "network" ? ICON.netBig : d.kind === "removable" ? ICON.usbBig : ICON.driveBig;
    const label = d.label || t("kind_" + d.kind);
    b.innerHTML = `
      <span class="d-icon">${icon}</span>
      <span>
        <span class="d-name">${esc(d.path.replace(/\\$/, ""))}<small>${esc(label)}${d.fs ? " (" + esc(d.fs) + ")" : ""}</small></span>
        ${d.total > 0 ? `<div class="d-bar"><span class="${pct > 90 ? "full" : ""}" style="width:${pct}%"></span></div>
        <span class="d-meta">${t("driveUsed", { used: fmtSize(used), total: fmtSize(d.total) })}, ${t("driveFree", { free: fmtSize(d.free) })}</span>` : ""}
      </span>
      <span class="d-go">${t("driveScan")}</span>`;
    b.onclick = () => startScan(d.path);
    list.append(b);
  }
  const rec = S.settings.recent || [];
  $("#recentWrap").hidden = rec.length === 0;
  const rl = $("#recentList");
  rl.innerHTML = "";
  for (const p of rec) {
    const b = el("button", null, esc(p));
    b.title = p;
    b.onclick = () => startScan(p);
    rl.append(b);
  }
}

function showDrivePopover() {
  const pop = $("#drivePop");
  if (!pop.hidden) { pop.hidden = true; return; }
  pop.innerHTML = "";
  for (const d of S.drives) {
    const b = el("button");
    const used = d.total - d.free;
    b.innerHTML = `${ICON.drive}<span>${esc(d.path.replace(/\\$/, ""))} <small>${esc(d.label || t("kind_" + d.kind))}</small></span><small>${d.total ? t("driveFree", { free: fmtSize(d.free) }) : ""}</small>`;
    b.onclick = () => { pop.hidden = true; startScan(d.path); };
    pop.append(b);
    void used;
  }
  const r = $("#driveBtn").getBoundingClientRect();
  pop.style.left = Math.max(8, r.right - 300) + "px";
  pop.style.top = r.bottom + 6 + "px";
  pop.hidden = false;
}

// ---------------------------------------------------------------- scanning
async function startScan(path) {
  path = (path || $("#pathInput").value).trim();
  if (!path) { $("#pathInput").focus(); return; }
  try {
    await api.StartScan(path);
  } catch (e) { showErr(e); return; }
  $("#pathInput").value = path;
  S.expanded = new Set(["d0"]);
  S.children.clear();
  S.flat = [];
  S.selected = "d0";
  S.focusDir = "d0";
  S.topFilter = "";
  S.status = null;
  $("#welcome").hidden = true;
  $("#workspace").hidden = false;
  $("#treeScroll").scrollTop = 0;
  poll();
}

async function poll() {
  if (S.polling) return;
  S.polling = true;
  try {
    for (;;) {
      const st = await api.Status();
      const fresh = !S.status || S.status.id !== st.id;
      S.status = st;
      renderStatus();
      await refreshTree();
      const now = Date.now();
      if (fresh || !st.running || now - S.lastSideRefresh > 2500) {
        S.lastSideRefresh = now;
        refreshSide();
      }
      if (!st.running) break;
      await new Promise((r) => setTimeout(r, 450));
    }
  } finally { S.polling = false; }
}

function renderStatus() {
  const st = S.status;
  const running = st && st.running;
  $("#scanBtn").hidden = !!running;
  $("#stopBtn").hidden = !running;
  $("#exportBtn").disabled = !st || !st.has || running;
  if (!st || !st.has) return;
  $("#rootName").textContent = st.rootPath;
  const state = $("#scanState");
  state.className = "scan-state" + (running ? " running" : "");
  state.textContent = running ? `${t("scanning")}  ${fmtDur(st.elapsedMs)}` : t(st.canceled ? "scanCanceled" : "scanDone", { t: fmtDur(st.elapsedMs) });

  if (st.volTotal > 0) {
    const used = st.volTotal - st.volFree;
    const scanned = Math.min(st.root.size, used);
    $("#volScanned").style.width = (scanned / st.volTotal) * 100 + "%";
    $("#volOther").style.width = ((used - scanned) / st.volTotal) * 100 + "%";
    $("#volText").textContent = t("volText", { scanned: fmtSize(st.root.size), used: fmtSize(used), total: fmtSize(st.volTotal) });
    $("#volWrap").hidden = false;
  } else $("#volWrap").hidden = true;

  $("#stFiles").textContent = t("stFiles", { n: num(st.files) });
  $("#stDirs").textContent = t("stDirs", { n: num(st.dirs) });
  const se = $("#stErrors");
  se.hidden = st.errors === 0;
  se.textContent = t("stErrors", { n: num(st.errors) });
  se.title = !S.admin && S.platform === "windows" ? t("adminHint") : "";
  $("#stCurrent").textContent = running ? st.current : "";
  $("#stTime").textContent = running ? "" : "";
}

// ---------------------------------------------------------------- tree
const ROW_H = 26;
const COLS = [
  { id: "name", label: "colName" }, { id: "size", label: "colSize" }, { id: "alloc", label: "colAlloc" },
  { id: "pct", label: "colPct", sort: S.settings?.sizeMode || "size" }, { id: "files", label: "colFiles" },
  { id: "dirs", label: "colDirs" }, { id: "mtime", label: "colMtime" },
];

function renderTreeHead() {
  const head = $("#treeHead");
  head.innerHTML = "";
  const row = el("div", "tree-cols");
  for (const c of COLS) {
    const sortKey = c.id === "pct" ? S.settings.sizeMode : c.id;
    const th = el("div", "th" + (S.sort.by === sortKey ? " sorted" : ""));
    th.innerHTML = esc(t(c.label)) + (S.sort.by === sortKey && c.id !== "pct" ? `<span class="arrow">${S.sort.desc ? "▼" : "▲"}</span>` : "");
    th.onclick = (e) => {
      if (e.target.classList.contains("grip")) return;
      if (S.sort.by === sortKey) S.sort.desc = !S.sort.desc;
      else { S.sort.by = sortKey; S.sort.desc = sortKey !== "name"; }
      renderTreeHead();
      refreshTree();
    };
    if (c.id !== "name") {
      const g = el("span", "grip");
      g.onmousedown = (e) => startColResize(e, c.id);
      th.append(g);
    }
    row.append(th);
  }
  head.append(row);
}

function startColResize(e, id) {
  e.preventDefault();
  const v = "--c-" + id;
  const start = e.clientX, w0 = parseFloat(cssVar(v));
  const move = (ev) => document.documentElement.style.setProperty(v, Math.max(48, w0 - (ev.clientX - start)) + "px");
  const up = () => { removeEventListener("mousemove", move); removeEventListener("mouseup", up); };
  addEventListener("mousemove", move);
  addEventListener("mouseup", up);
}

async function refreshTree() {
  const st = S.status;
  if (!st || !st.has) return;
  const keys = [...S.expanded];
  let res;
  try {
    res = await api.ChildrenMany(keys, S.sort.by, S.sort.desc, !!S.settings.showFiles);
  } catch (e) { return; }
  for (const k of keys) {
    if (res[k]) S.children.set(k, res[k]);
    else { S.expanded.delete(k); S.children.delete(k); }
  }
  // the root row comes from the status; refresh it from the latest children call
  if (res.d0) S.status.root = res.d0.parent;
  buildFlat();
  renderRows();
}

function buildFlat() {
  const out = [];
  const root = S.status.root;
  out.push({ row: root, depth: 0, parentSize: 0 });
  const walk = (key, depth) => {
    const c = S.children.get(key);
    if (!c) return;
    const ps = sizeOf(c.parent);
    for (const r of c.rows) {
      out.push({ row: r, depth, parentSize: ps });
      if (r.isDir && S.expanded.has(r.key)) walk(r.key, depth + 1);
    }
  };
  if (S.expanded.has(root.key)) walk(root.key, 1);
  S.flat = out;
}

function renderRows() {
  const sc = $("#treeScroll");
  $("#treeSizer").style.height = S.flat.length * ROW_H + "px";
  const first = Math.max(0, Math.floor(sc.scrollTop / ROW_H) - 5);
  const last = Math.min(S.flat.length, Math.ceil((sc.scrollTop + sc.clientHeight) / ROW_H) + 5);
  const box = $("#treeRows");
  box.style.transform = `translateY(${first * ROW_H}px)`;
  let html = "";
  for (let i = first; i < last; i++) html += rowHtml(S.flat[i], i);
  box.innerHTML = html;
  $("#treeHead").firstChild && ($("#treeHead").firstChild.style.transform = `translateX(${-sc.scrollLeft}px)`);
}

function rowHtml(item, i) {
  const r = item.row;
  const s = sizeOf(r);
  const pct = item.depth === 0 ? 100 : item.parentSize > 0 ? (s / item.parentSize) * 100 : 0;
  const open = r.isDir && S.expanded.has(r.key);
  const tw = r.isDir && r.hasKids && !r.linked ? `<span class="twisty${open ? " open" : ""}" data-tw="1">${ICON.chevron}</span>` : `<span class="twisty none"></span>`;
  const icon = r.isDir ? `<span class="ficon folder">${ICON.folder}</span>` : `<span class="ficon file">${ICON.file}</span>`;
  let badge = "";
  if (r.err) badge = `<span class="badge">${t(r.err === "denied" ? "errDenied" : "errOther")}</span>`;
  else if (r.linked) badge = `<span class="badge link">${t("linked")}</span>`;
  else if (r.isDir && !r.done) badge = `<span class="busy-dot"></span>`;
  const name = item.depth === 0 ? S.status.rootPath : r.name;
  return `<div class="row tree-cols${r.key === S.selected ? " sel" : ""}" data-i="${i}">
    <div class="cell name" style="padding-left:${4 + item.depth * 16}px">${tw}${icon}<span class="nm">${esc(name)}</span>${badge}</div>
    <div class="cell">${fmtSize(r.size)}</div>
    <div class="cell dim">${fmtSize(r.alloc)}</div>
    <div class="cell pct"><span class="bar"><span class="${r.isDir ? "" : "f"}" style="width:${Math.min(100, pct)}%"></span></span><span class="num">${fmtPct(pct)}</span></div>
    <div class="cell dim">${r.isDir ? num(r.files) : ""}</div>
    <div class="cell dim">${r.isDir ? num(r.dirs) : ""}</div>
    <div class="cell dim">${fmtDate(r.mtime)}</div>
  </div>`;
}

async function toggle(key, force) {
  const open = force ?? !S.expanded.has(key);
  if (open) S.expanded.add(key); else S.expanded.delete(key);
  await refreshTree();
}

function select(key, opts = {}) {
  S.selected = key;
  const isDir = key.startsWith("d");
  const dir = isDir ? key : parentKeyOf(key);
  renderRows();
  if (opts.scroll) scrollToKey(key);
  if (dir && dir !== S.focusDir) { S.focusDir = dir; S.topFilter = ""; refreshSide(); }
  map.setSelected(key);
}

function parentKeyOf(key) {
  if (key.startsWith("f")) return "d" + key.slice(1).split(":")[0];
  const it = S.flat.find((f) => f.row.key === key);
  if (!it) return null;
  const idx = S.flat.indexOf(it);
  for (let i = idx - 1; i >= 0; i--) if (S.flat[i].depth < it.depth) return S.flat[i].row.key;
  return null;
}

function scrollToKey(key) {
  const i = S.flat.findIndex((f) => f.row.key === key);
  if (i < 0) return;
  const sc = $("#treeScroll");
  const top = i * ROW_H;
  if (top < sc.scrollTop) sc.scrollTop = top;
  else if (top + ROW_H > sc.scrollTop + sc.clientHeight) sc.scrollTop = top - sc.clientHeight + ROW_H * 2;
  renderRows();
}

async function reveal(key) {
  const anc = await api.Ancestors(key);
  if (!anc) return;
  anc.forEach((k) => S.expanded.add(k));
  await refreshTree();
  select(key, { scroll: true });
  $("#treeScroll").focus();
}

function rowFromEvent(e) {
  const r = e.target.closest(".row");
  return r ? S.flat[+r.dataset.i] : null;
}

function setupTree() {
  const sc = $("#treeScroll");
  let raf = 0;
  sc.addEventListener("scroll", () => { if (!raf) raf = requestAnimationFrame(() => { raf = 0; renderRows(); }); });
  new ResizeObserver(() => renderRows()).observe(sc);
  sc.addEventListener("mousedown", (e) => {
    const it = rowFromEvent(e);
    if (!it) return;
    if (e.target.closest("[data-tw]")) { toggle(it.row.key); return; }
    select(it.row.key);
  });
  sc.addEventListener("dblclick", (e) => {
    const it = rowFromEvent(e);
    if (!it || e.target.closest("[data-tw]")) return;
    if (it.row.isDir) toggle(it.row.key); else api.OpenItem(it.row.key).catch(showErr);
  });
  sc.addEventListener("contextmenu", (e) => {
    e.preventDefault();
    const it = rowFromEvent(e);
    if (!it) return;
    select(it.row.key);
    openMenu(e.clientX, e.clientY, it.row, "tree");
  });
  sc.addEventListener("keydown", treeKeys);
}

function treeKeys(e) {
  const i = S.flat.findIndex((f) => f.row.key === S.selected);
  const it = S.flat[i];
  const go = (j) => { j = Math.max(0, Math.min(S.flat.length - 1, j)); select(S.flat[j].row.key, { scroll: true }); };
  const page = Math.floor($("#treeScroll").clientHeight / ROW_H) - 1;
  switch (e.key) {
    case "ArrowDown": go(i + 1); break;
    case "ArrowUp": go(i - 1); break;
    case "PageDown": go(i + page); break;
    case "PageUp": go(i - page); break;
    case "Home": go(0); break;
    case "End": go(S.flat.length - 1); break;
    case "ArrowRight":
      if (it && it.row.isDir && it.row.hasKids) { if (!S.expanded.has(it.row.key)) toggle(it.row.key, true); else go(i + 1); }
      break;
    case "ArrowLeft":
      if (it && it.row.isDir && S.expanded.has(it.row.key)) toggle(it.row.key, false);
      else if (it) { const p = parentKeyOf(it.row.key); if (p) select(p, { scroll: true }); }
      break;
    case "Enter": if (it) { if (it.row.isDir) toggle(it.row.key); else api.OpenItem(it.row.key).catch(showErr); } break;
    case "Delete": if (it) askDelete(it.row); break;
    case "F5": if (it) rescan(it.row.isDir ? it.row.key : parentKeyOf(it.row.key)); break;
    case "c": if (e.ctrlKey && it) copyPath(it.row.key); else return; break;
    default: return;
  }
  e.preventDefault();
}

// ---------------------------------------------------------------- context menu & actions
function openMenu(x, y, row, origin) {
  const m = $("#menu");
  m.innerHTML = "";
  const isDir = row.isDir || (row.key && row.key.startsWith("d"));
  const add = (icon, label, fn, cls, kbd) => {
    const b = el("button", cls || "", `${ICON[icon] || ""}<span>${esc(label)}</span>${kbd ? `<kbd>${kbd}</kbd>` : ""}`);
    b.onclick = () => { closeMenu(); fn(); };
    m.append(b);
  };
  const sep = () => m.append(el("hr"));
  if (!row.key) return; // aggregated treemap tiles have no target
  if (!isDir) add("open", t("mOpen"), () => api.OpenItem(row.key).catch(showErr), "", "Enter");
  add("pick", t("mExplorer"), () => api.ShowInExplorer(row.key).catch(showErr));
  add("copy", t("mCopyPath"), () => copyPath(row.key), "", "Strg+C".replace("Strg", S.lang === "de" ? "Strg" : "Ctrl"));
  if (origin !== "tree") add("target", t("mReveal"), () => reveal(row.key));
  if (isDir) {
    sep();
    if (origin === "tree") add("map", t("mMapRoot"), () => { select(row.key); setTab("map"); });
    add("refresh", t("mRescan"), () => rescan(row.key), "", "F5");
    add("play", t("mScanHere"), async () => startScan(await api.PathOf(row.key)));
  }
  if (row.key !== "d0") { sep(); add("trash", t("mDelete"), () => askDelete(row), "danger", "Entf".replace("Entf", S.lang === "de" ? "Entf" : "Del")); }
  m.hidden = false;
  const r = m.getBoundingClientRect();
  m.style.left = Math.min(x, innerWidth - r.width - 8) + "px";
  m.style.top = Math.min(y, innerHeight - r.height - 8) + "px";
  m.querySelector("button")?.focus();
}
function closeMenu() { $("#menu").hidden = true; }

async function copyPath(key) {
  try { await api.CopyText(await api.PathOf(key)); toast(t("copied")); } catch (e) { showErr(e); }
}

async function rescan(key) {
  if (!key) return;
  try { await api.Rescan(key); poll(); } catch (e) { showErr(e); }
}

async function askDelete(row) {
  if (!row.key || row.key === "d0") return;
  const path = await api.PathOf(row.key);
  if (S.settings.confirmDelete) {
    const dlg = $("#confirmDlg");
    $("#confirmTitle").textContent = t("confirmTitle");
    $("#confirmText").textContent = t("confirmText", { name: row.name || path, size: fmtSize(row.size ?? row.s ?? 0) });
    $("#confirmPath").textContent = path;
    $("#confirmNoAsk").checked = false;
    dlg.returnValue = "";
    dlg.showModal();
    $("#confirmOk").focus();
    const ok = await new Promise((res) => dlg.addEventListener("close", () => res(dlg.returnValue === "ok"), { once: true }));
    if (!ok) return;
    if ($("#confirmNoAsk").checked) { S.settings.confirmDelete = false; api.SaveSettings(S.settings); }
  }
  try {
    const neighbour = neighbourKey(row.key);
    await api.Delete(row.key);
    toast(t("deleted", { name: row.name || path }));
    // file keys of the same folder shift after a delete; drop cached rows
    S.children.clear();
    if (S.selected === row.key) S.selected = neighbour;
    await refreshTree();
    S.lastSideRefresh = 0;
    refreshSide();
  } catch (e) { showErr(e); }
}

function neighbourKey(key) {
  const i = S.flat.findIndex((f) => f.row.key === key);
  if (i < 0) return "d0";
  const p = parentKeyOf(key);
  return p || "d0";
}

// ---------------------------------------------------------------- side panel
function setTab(tab) {
  S.tab = tab;
  document.querySelectorAll(".tabs button").forEach((b) => b.setAttribute("aria-selected", b.dataset.tab === tab));
  for (const id of ["map", "types", "top", "age"]) $("#panel-" + id).hidden = id !== tab;
  refreshSide();
}

async function refreshSide() {
  if (!S.status || !S.status.has) return;
  const key = S.focusDir || "d0";
  renderCrumbs(key);
  try {
    if (S.tab === "map") await renderMap(key);
    else if (S.tab === "types") await renderTypes(key);
    else if (S.tab === "top") await renderTop(key);
    else await renderAges(key);
  } catch (e) {
    if (String(e) === "notfound") { S.focusDir = "d0"; refreshSide(); }
  }
}

async function renderCrumbs(key) {
  const box = $("#crumbs");
  const anc = (await api.Ancestors(key)) || [];
  const keys = [...anc];
  if (keys[keys.length - 1] !== key) keys.push(key);
  box.innerHTML = "";
  for (let i = 0; i < keys.length; i++) {
    const k = keys[i];
    const it = S.flat.find((f) => f.row.key === k);
    const name = i === 0 ? S.status.rootPath : it ? it.row.name : (await api.PathOf(k)).split(/[\\/]/).pop();
    if (i) box.append(el("span", "sep", "›"));
    const b = el("button", null, esc(name));
    b.onclick = () => reveal(k);
    box.append(b);
  }
}

// --- treemap
let map;
const DEPTH_HUES = [205, 28, 150, 265, 45, 340, 180, 95, 0, 230];
function setupMap() {
  map = new Treemap($("#mapCanvas"), {
    color: (n, depth, branch) => {
      if (n.r) return cssVar("--t-other");
      if (S.settings.mapColor === "depth") {
        const bi = branchIndex(branch);
        const l = Math.max(30, 52 - depth * 4);
        return `hsl(${DEPTH_HUES[bi % DEPTH_HUES.length]} 42% ${l}%)`;
      }
      return n.d ? cssVar("--t-other") : catColor(catOf(n.e || extOf(n.n)));
    },
    folderColor: (n, depth) => {
      const dark = document.documentElement.dataset.theme === "dark";
      const l = dark ? 14 + Math.min(depth, 6) * 3 : 88 - Math.min(depth, 6) * 5;
      return `hsl(210 14% ${l}%)`;
    },
    folderInk: (depth) => (document.documentElement.dataset.theme === "dark" ? "rgba(226,232,238,.85)" : `rgba(21,32,43,${depth > 3 ? 0.85 : 0.75})`),
    label: (n) => (n.r ? t("restItems", { n: num(n.r) }) : n.n),
    sizeText: (n) => fmtSize(n.s),
    onHover: (n, e) => showTip(n, e),
    onClick: (r) => { if (r.node.k) reveal(r.node.k); },
    onDblClick: (r) => { if (r.node.k && r.node.d) { S.focusDir = r.node.k; refreshSide(); } },
    onContext: (r, e) => { if (r.node.k) openMenu(e.clientX, e.clientY, { key: r.node.k, name: r.node.n, size: r.node.s, isDir: !!r.node.d }, "map"); },
  });
}
let branchMap = new Map();
function branchIndex(b) {
  if (!b) return 0;
  if (!branchMap.has(b.k)) branchMap.set(b.k, branchMap.size);
  return branchMap.get(b.k);
}

async function renderMap(key) {
  const data = await api.Treemap(key, 5000);
  branchMap = new Map();
  map.setData(data);
  map.setSelected(S.selected);
  const lg = $("#mapLegend");
  if (S.settings.mapColor === "depth") { lg.innerHTML = ""; return; }
  lg.innerHTML = CATEGORIES.map((c) => `<span><i style="background:${catColor(c)}"></i>${esc(t("cat_" + c))}</span>`).join("");
}

function showTip(n, e) {
  const tip = $("#tooltip");
  if (!n) { tip.hidden = true; return; }
  const total = S.status.root.size;
  const share = total > 0 ? (n.s / total) * 100 : 0;
  const title = n.r ? t("restItems", { n: num(n.r) }) : n.n;
  const sub = n.d ? t("cat_folder") : n.r ? "" : t("cat_" + catOf(n.e || extOf(n.n)));
  tip.innerHTML = `<b>${esc(title)}</b><span>${fmtSize(n.s)}  (${fmtPct(share)})</span>${sub ? `<div class="tp">${esc(sub)}</div>` : ""}`;
  tip.hidden = false;
  const r = tip.getBoundingClientRect();
  tip.style.left = Math.min(e.clientX + 14, innerWidth - r.width - 8) + "px";
  tip.style.top = Math.min(e.clientY + 16, innerHeight - r.height - 8) + "px";
}

// --- file types
async function renderTypes(key) {
  const list = await api.Extensions(key);
  const panel = $("#panel-types");
  const total = list.reduce((a, x) => a + x.size, 0);
  if (!list.length || total === 0) { panel.innerHTML = `<p class="empty">${t("emptyFolder")}</p>`; return; }
  const cats = {};
  for (const x of list) { const c = x.ext === "*" ? "other" : catOf(x.ext); cats[c] = (cats[c] || 0) + x.size; }
  const order = CATEGORIES.filter((c) => cats[c] > 0).sort((a, b) => cats[b] - cats[a]);
  let html = `<div class="strata">${order.map((c) => `<span title="${esc(t("cat_" + c))}: ${fmtSize(cats[c])}" style="width:${(cats[c] / total) * 100}%;background:${catColor(c)}"></span>`).join("")}</div>`;
  html += `<div class="strata-legend">${order.map((c) => `<span><i style="background:${catColor(c)}"></i>${esc(t("cat_" + c))} <b>${fmtPct((cats[c] / total) * 100)}</b></span>`).join("")}</div>`;
  html += `<table class="list"><colgroup><col><col style="width:150px"><col style="width:90px"><col style="width:96px"><col style="width:140px"></colgroup>
    <thead><tr><th>${t("colType")}</th><th></th><th>${t("colCount")}</th><th>${t("colSize")}</th><th>${t("colPct")}</th></tr></thead><tbody>`;
  const max = list[0].size || 1;
  for (const x of list) {
    const c = x.ext === "*" ? "other" : catOf(x.ext);
    const label = x.ext === "*" ? t("restExt") : x.ext === "" ? t("noExt") : "." + x.ext;
    html += `<tr class="click" data-ext="${esc(x.ext)}"><td><span class="swatch" style="background:${catColor(c)}"></span>${esc(label)}</td>
      <td class="left" style="color:var(--muted)">${esc(t("cat_" + c))}</td><td>${num(x.count)}</td><td>${fmtSize(x.size)}</td>
      <td><span class="mini"><span style="width:${(x.size / max) * 100}%;background:${catColor(c)}"></span></span>${fmtPct((x.size / total) * 100)}</td></tr>`;
  }
  panel.innerHTML = html + "</tbody></table>";
  panel.querySelectorAll("tr[data-ext]").forEach((tr) => {
    if (tr.dataset.ext === "*") return;
    tr.onclick = () => { S.topFilter = tr.dataset.ext === "" ? "-" : tr.dataset.ext; setTab("top"); };
  });
}

// --- largest files
async function renderTop(key) {
  const files = await api.TopFiles(key, S.settings.topFiles || 200, S.topFilter);
  const panel = $("#panel-top");
  let html = "";
  if (S.topFilter) html += `<div class="filter-chip">${esc(S.topFilter === "-" ? t("filterNoExt") : t("filterExt", { ext: S.topFilter }))}<button data-clear title="×">×</button></div>`;
  if (!files.length) { panel.innerHTML = html + `<p class="empty">${t("emptyFolder")}</p>`; bindChip(panel); return; }
  const root = S.status.rootPath.replace(/[\\/]$/, "");
  html += `<table class="list"><colgroup><col><col style="width:96px"><col style="width:136px"></colgroup>
    <thead><tr><th>${t("colName")}</th><th>${t("colSize")}</th><th>${t("colMtime")}</th></tr></thead><tbody>`;
  files.forEach((f, i) => {
    const rel = f.folder.startsWith(root) ? "…" + f.folder.slice(root.length) : f.folder;
    html += `<tr class="click${f.key === S.selected ? " sel" : ""}" data-i="${i}"><td title="${esc(f.folder)}">
      <span class="swatch" style="background:${catColor(catOf(f.ext))}"></span>${esc(f.name)}<span class="sub">${esc(rel || root)}</span></td>
      <td>${fmtSize(f.size)}</td><td style="color:var(--muted)">${fmtDate(f.mtime)}</td></tr>`;
  });
  panel.innerHTML = html + "</tbody></table>";
  bindChip(panel);
  panel.querySelectorAll("tr[data-i]").forEach((tr) => {
    const f = files[+tr.dataset.i];
    tr.onclick = () => { panel.querySelectorAll("tr.sel").forEach((x) => x.classList.remove("sel")); tr.classList.add("sel"); revealKeepSide(f.key); };
    tr.ondblclick = () => api.OpenItem(f.key).catch(showErr);
    tr.oncontextmenu = (e) => { e.preventDefault(); openMenu(e.clientX, e.clientY, { key: f.key, name: f.name, size: f.size, isDir: false }, "top"); };
  });
}
function bindChip(panel) {
  const c = panel.querySelector("[data-clear]");
  if (c) c.onclick = () => { S.topFilter = ""; refreshSide(); };
}
// reveal a file in the tree without switching the side panel to its folder
async function revealKeepSide(key) {
  const keep = S.focusDir;
  const anc = await api.Ancestors(key);
  anc.forEach((k) => S.expanded.add(k));
  await refreshTree();
  S.selected = key;
  scrollToKey(key);
  S.focusDir = keep;
}

// --- ages
async function renderAges(key) {
  const b = await api.Ages(key);
  const panel = $("#panel-age");
  const max = Math.max(1, ...b.map((x) => x.size));
  const total = b.reduce((a, x) => a + x.size, 0);
  if (!total) { panel.innerHTML = `<p class="empty">${t("emptyFolder")}</p>`; return; }
  panel.innerHTML = `<div class="ages">${b.map((x) => `<div class="age-col" title="${num(x.count)} · ${fmtSize(x.size)}">
      <span class="av">${fmtSize(x.size)}</span><div class="ab" style="height:${(x.size / max) * 82}%"></div></div>`).join("")}</div>
    <div class="age-labels">${b.map((x) => `<span>${esc(t("age_" + x.id))}<br><small>${x.count === 1 ? t("files1") : t("filesN", { n: num(x.count) })}</small></span>`).join("")}</div>
    <p class="age-note">${esc(t("ageNote"))}</p>`;
}

// ---------------------------------------------------------------- settings
function openSettings() {
  const f = $("#settingsForm");
  const s = S.settings;
  for (const [k, v] of Object.entries(s)) {
    const inp = f.elements[k];
    if (!inp) continue;
    if (inp.type === "checkbox") inp.checked = !!v;
    else if (k === "exclude") inp.value = (v || []).join("\n");
    else inp.value = v;
  }
  $("#about").textContent = t("about", { v: S.version });
  const dlg = $("#settingsDlg");
  dlg.returnValue = "";
  dlg.showModal();
  dlg.addEventListener("close", async () => {
    if (dlg.returnValue !== "ok") return;
    const n = { ...S.settings };
    for (const k of Object.keys(n)) {
      const inp = f.elements[k];
      if (!inp) continue;
      if (inp.type === "checkbox") n[k] = inp.checked;
      else if (inp.type === "number") n[k] = parseInt(inp.value, 10) || 0;
      else if (k === "exclude") n[k] = inp.value.split(/\r?\n/).map((x) => x.trim()).filter(Boolean);
      else n[k] = inp.value;
    }
    S.settings = n;
    try { await api.SaveSettings(n); } catch (e) { showErr(e); }
    resolveLang(); applyI18n(); applyTheme(); renderTreeHead();
    if (S.status && S.status.has) { await refreshTree(); refreshSide(); } else renderWelcome();
  }, { once: true });
}

// ---------------------------------------------------------------- splitter
function setupSplitter() {
  const sp = $("#splitter"), panes = $("#panes");
  sp.addEventListener("mousedown", (e) => {
    e.preventDefault();
    sp.classList.add("drag");
    const b = panes.getBoundingClientRect();
    const move = (ev) => panes.style.setProperty("--split", Math.min(80, Math.max(25, ((ev.clientX - b.left) / b.width) * 100)) + "%");
    const up = () => { sp.classList.remove("drag"); removeEventListener("mousemove", move); removeEventListener("mouseup", up); map.draw(); };
    addEventListener("mousemove", move);
    addEventListener("mouseup", up);
  });
}

// ---------------------------------------------------------------- boot
async function waitForBackend() {
  for (let i = 0; i < 100; i++) {
    if (window.go && window.go.main && window.go.main.App) return true;
    await new Promise((r) => setTimeout(r, 30));
  }
  return false;
}

async function boot() {
  if (!(await waitForBackend())) { document.body.innerHTML = "<p style='padding:20px'>Backend not available.</p>"; return; }
  const info = await api.Init();
  Object.assign(S, { settings: info.settings, drives: info.drives || [], admin: info.admin, platform: info.platform, version: info.version });
  resolveLang();
  applyTheme();
  applyI18n();
  matchMedia("(prefers-color-scheme: dark)").addEventListener("change", applyTheme);

  $("#pickBtn").innerHTML = ICON.pick;
  $("#driveBtn").innerHTML = ICON.drive;
  $("#exportBtn").innerHTML = ICON.export;
  $("#adminBtn").innerHTML = ICON.shield;
  $("#settingsBtn").innerHTML = ICON.gear;
  $("#scanBtn").insertAdjacentHTML("afterbegin", ICON.play);
  $("#stopBtn").insertAdjacentHTML("afterbegin", ICON.stop);
  $("#adminBtn").hidden = S.admin || S.platform !== "windows";

  $("#scanBtn").onclick = () => startScan();
  $("#stopBtn").onclick = () => api.Cancel();
  $("#pathInput").addEventListener("keydown", (e) => { if (e.key === "Enter") startScan(); });
  $("#pickBtn").onclick = async () => { const p = await api.PickFolder(); if (p) { $("#pathInput").value = p; startScan(p); } };
  $("#driveBtn").onclick = (e) => { e.stopPropagation(); showDrivePopover(); };
  $("#exportBtn").onclick = async () => { try { const p = await api.Export(S.focusDir || "d0"); if (p) toast(t("exported", { path: p })); } catch (e) { showErr(e); } };
  $("#adminBtn").onclick = () => api.RestartAsAdmin().catch(showErr);
  $("#stErrors").onclick = () => { if (!S.admin && S.platform === "windows") api.RestartAsAdmin().catch(showErr); };
  $("#settingsBtn").onclick = openSettings;
  document.querySelectorAll(".tabs button").forEach((b) => (b.onclick = () => setTab(b.dataset.tab)));
  document.addEventListener("mousedown", (e) => {
    if (!e.target.closest("#menu")) closeMenu();
    if (!e.target.closest("#drivePop") && !e.target.closest("#driveBtn")) $("#drivePop").hidden = true;
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") { closeMenu(); $("#drivePop").hidden = true; }
    if (e.key === "F5" && !e.target.closest("#treeScroll")) e.preventDefault();
  });
  document.addEventListener("contextmenu", (e) => { if (!e.target.closest("input, textarea")) e.preventDefault(); });

  setupTree();
  setupMap();
  setupSplitter();
  renderTreeHead();
  setTab("map");
  renderWelcome();
  renderStatus();

  if (info.startArg) startScan(info.startArg);
  else {
    const st = await api.Status();
    if (st.has) { S.status = st; $("#welcome").hidden = true; $("#workspace").hidden = false; S.expanded = new Set(["d0"]); S.selected = S.focusDir = "d0"; poll(); }
  }
}

boot();
