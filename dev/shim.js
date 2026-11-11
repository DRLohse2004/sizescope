// Browser shim: maps window.go.main.App.X(...) to the dev server.
window.go = { main: { App: new Proxy({}, { get: (_, m) => async (...args) => {
  const r = await fetch("/api/" + m, { method: "POST", body: JSON.stringify(args) });
  const txt = await r.text();
  let j;
  try { j = JSON.parse(txt); } catch (e) { console.error("shim", m, r.status, JSON.stringify(txt.slice(0, 80))); throw e; }
  if (j.error) throw j.error;
  return j.result;
} }) } };
