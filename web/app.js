// WinForge dashboard — zero-dependency single-page app.
const $ = (sel) => document.querySelector(sel);

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try { msg = (await res.json()).error || msg; } catch (_) {}
    throw new Error(msg);
  }
  return res.json();
}

// ---- Navigation ----
document.querySelectorAll("nav a").forEach((a) => {
  a.addEventListener("click", (e) => {
    e.preventDefault();
    document.querySelectorAll("nav a").forEach((x) => x.classList.remove("active"));
    document.querySelectorAll(".view").forEach((x) => x.classList.remove("active"));
    a.classList.add("active");
    $("#view-" + a.dataset.view).classList.add("active");
    loadView(a.dataset.view);
  });
});

function loadView(name) {
  if (name === "dashboard") loadDashboard();
  if (name === "tweaks") loadTweaks();
  if (name === "apps") loadApps();
  if (name === "history") loadHistory();
}

// ---- Dashboard ----
async function loadStatus() {
  const s = await api("/api/status");
  $("#version").textContent = "v" + s.version;
  $("#os-name").textContent = s.os.productName || s.os.os;
  const pill = $("#elevation");
  if (s.elevated) { pill.textContent = "Administrator"; pill.classList.add("ok"); }
  else { pill.textContent = "Not elevated"; pill.classList.add("warn"); }
}

async function loadDashboard() {
  await loadStatus();
  const h = await api("/api/health");
  $("#health-score").textContent = h.score;
  $("#health-meter").style.width = h.score + "%";
  $("#tweaks-applied").textContent = `${h.appliedTweaks}/${h.totalTweaks}`;

  const rows = [
    ["Tweaks applied", `${h.appliedTweaks} / ${h.totalTweaks}`],
    ["Unapplied (low)", h.unappliedLow],
    ["Unapplied (medium)", h.unappliedMedium],
    ["Unapplied (high)", h.unappliedHigh],
    ["Bloatware detected", h.bloatwareCount],
  ];
  $("#health-breakdown").innerHTML = rows
    .map(([k, v]) => `<div class="row"><span>${k}</span><span>${v}</span></div>`)
    .join("");
}

// ---- Tweaks ----
let tweaksById = {};

async function loadTweaks() {
  const tweaks = await api("/api/tweaks");
  tweaksById = Object.fromEntries(tweaks.map((t) => [t.id, t]));

  const byCat = {};
  for (const t of tweaks) (byCat[t.category] ||= []).push(t);

  $("#tweaks-list").innerHTML = Object.entries(byCat)
    .map(([cat, list]) => {
      const cards = list
        .map(
          (t) => `
        <div class="tweak">
          <div class="info">
            <div class="name">${esc(t.name)}</div>
            <div class="desc">${esc(t.description)}</div>
          </div>
          <span class="risk ${t.risk}">${t.risk}</span>
          <label class="switch">
            <input type="checkbox" ${t.applied ? "checked" : ""} data-id="${t.id}" />
            <span class="slider"></span>
          </label>
        </div>`
        )
        .join("");
      return `<div class="tweak-category">${esc(cat)}</div>${cards}`;
    })
    .join("");

  document.querySelectorAll(".switch input").forEach((el) => {
    el.addEventListener("change", async () => {
      const id = el.dataset.id;
      const t = tweaksById[id];
      try {
        if (el.checked) await api("/api/tweaks/apply", { method: "POST", body: JSON.stringify({ id }) });
        else await api("/api/tweaks/undo", { method: "POST", body: JSON.stringify({ id }) });
      } catch (err) {
        el.checked = !el.checked;
        alert(`${t.name}: ${err.message}`);
      }
    });
  });
}

// ---- Apps ----
let apps = [];
let selected = new Set();

async function loadApps() {
  apps = await api("/api/apps");
  const cats = [...new Set(apps.map((a) => a.category))];
  $("#app-categories").innerHTML =
    `<span class="chip active" data-cat="">All</span>` +
    cats.map((c) => `<span class="chip" data-cat="${esc(c)}">${esc(c)}</span>`).join("");

  document.querySelectorAll(".chip").forEach((c) =>
    c.addEventListener("click", () => {
      document.querySelectorAll(".chip").forEach((x) => x.classList.remove("active"));
      c.classList.add("active");
      renderApps();
    })
  );

  $("#app-search").addEventListener("input", renderApps);
  $("#app-install-selected").addEventListener("click", installSelected);
  renderApps();
}

function renderApps() {
  const q = $("#app-search").value.toLowerCase();
  const cat = document.querySelector(".chip.active")?.dataset.cat || "";
  const list = apps.filter(
    (a) => (!cat || a.category === cat) && (!q || (a.name + " " + a.description + " " + a.id).toLowerCase().includes(q))
  );

  $("#apps-list").innerHTML = list
    .map(
      (a) => `
      <div class="app-card">
        <div class="name">${esc(a.name)}</div>
        <div class="desc">${esc(a.description)}</div>
        <div class="foot">
          <span class="cat">${esc(a.category)}</span>
          <label><input type="checkbox" data-app="${esc(a.id)}" ${selected.has(a.id) ? "checked" : ""} /> select</label>
        </div>
        <button class="small primary" data-install="${esc(a.id)}">Install</button>
      </div>`
    )
    .join("");

  document.querySelectorAll("[data-app]").forEach((cb) =>
    cb.addEventListener("change", () => {
      cb.checked ? selected.add(cb.dataset.app) : selected.delete(cb.dataset.app);
      $("#app-install-selected").disabled = selected.size === 0;
    })
  );
  document.querySelectorAll("[data-install]").forEach((b) =>
    b.addEventListener("click", () => install(b.dataset.install))
  );
}

async function installSelected() {
  for (const id of [...selected]) await install(id);
  selected.clear();
  $("#app-install-selected").disabled = true;
  renderApps();
}

async function install(id) {
  const log = $("#install-log");
  log.classList.remove("hidden");
  log.textContent = `Installing ${id}…\n`;
  try {
    const job = await api("/api/apps/install", { method: "POST", body: JSON.stringify({ id }) });
    pollJob(job.id, log);
  } catch (err) {
    log.textContent += `Error: ${err.message}\n`;
  }
}

async function pollJob(id, log) {
  const job = await api("/api/apps/jobs/" + id);
  log.textContent = `Installing ${job.id}…\n` + job.lines.join("\n");
  if (!job.done) {
    setTimeout(() => pollJob(id, log), 800);
  } else {
    log.textContent += `\n[${job.status}]\n`;
  }
}

// ---- History ----
async function loadHistory() {
  const entries = await api("/api/history");
  $("#history-body").innerHTML = entries
    .map(
      (e) => `
      <tr>
        <td>${new Date(e.timestamp).toLocaleString()}</td>
        <td>${esc(e.operationType)}</td>
        <td>${esc(e.target)}</td>
        <td><span class="tag ${e.success ? "ok" : "fail"}">${e.success ? "ok" : "failed"}</span></td>
        <td>${e.canUndo ? `<button class="small" data-undo="${e.id}">Undo</button>` : ""}</td>
      </tr>`
    )
    .join("");

  document.querySelectorAll("[data-undo]").forEach((b) =>
    b.addEventListener("click", async () => {
      try {
        await api("/api/history/undo", { method: "POST", body: JSON.stringify({ id: b.dataset.undo }) });
        loadHistory();
      } catch (err) {
        alert(err.message);
      }
    })
  );
}

function esc(s) {
  return String(s ?? "").replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c])
  );
}

// Initial load.
loadDashboard();
