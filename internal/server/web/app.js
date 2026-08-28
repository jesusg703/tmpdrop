"use strict";

(() => {
  const $ = (sel) => document.querySelector(sel);
  const el = (tag, cls, text) => {
    const n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  };

  const state = { pollTimers: {}, converting: new Set() };

  const fmtBytes = (n) => {
    if (!Number.isFinite(n)) return "-";
    const units = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
    return `${n.toFixed(n >= 10 || i === 0 ? 0 : 1)} ${units[i]}`;
  };

  const fmtWhen = (iso) => {
    if (!iso) return "never";
    const d = new Date(iso);
    const diff = d.getTime() - Date.now();
    if (diff < 0) return "expired";
    if (diff < 24 * 3600 * 1000) return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
    return d.toLocaleDateString([], { day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });
  };

  const stripExt = (name) => {
    const i = name.lastIndexOf(".");
    return i > 0 ? name.slice(0, i) : name;
  };

  const shareURL = (f) => new URL(f.download_url || f.url, location.href).href;

  // navigator.clipboard only exists in a secure context, so it is missing on
  // plain http://<ip>:8080 — which is exactly how most people will run this.
  // execCommand is the fallback there; if even that is gone the caller still
  // shows the link in a field the reader can select.
  const copyText = async (text) => {
    try {
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(text);
        return true;
      }
    } catch (_) { /* fall through */ }

    const ta = el("textarea");
    ta.value = text;
    ta.setAttribute("readonly", "");
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    try {
      ta.select();
      ta.setSelectionRange(0, text.length);
      return document.execCommand("copy");
    } catch (_) {
      return false;
    } finally {
      ta.remove();
    }
  };

  const showShare = (f) => {
    const box = $("#share");
    const field = $("#share-link");
    if (!box || !field) return;
    field.value = shareURL(f);
    box.classList.remove("hidden");
  };

  const copyLink = async (f) => {
    const url = shareURL(f);
    if (await copyText(url)) {
      toast("Link copied");
    } else {
      toast("Could not copy — select the link above", true);
    }
    showShare(f);
  };

  let toastTimer = null;
  const toast = (msg, isError = false) => {
    const t = $("#toast");
    t.textContent = msg;
    t.classList.toggle("error", isError);
    t.classList.add("show");
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => t.classList.remove("show"), 3600);
  };

  const api = async (url, opts = {}) => {
    const res = await fetch(url, opts);
    if (!res.ok) {
      let msg = `HTTP ${res.status}`;
      try { const j = await res.json(); if (j.error) msg = j.error; } catch (_) {}
      throw new Error(msg);
    }
    const ct = res.headers.get("content-type") || "";
    return ct.includes("application/json") ? res.json() : res.text();
  };

  async function loadFiles() {
    try {
      const data = await api("/api/files");
      renderUsage(data);
      renderList(data.files);
    } catch (err) {
      renderList([]);
      toast(`Could not load files: ${err.message}`, true);
    }
  }

  function renderUsage(data) {
    const text = `${data.count} file${data.count === 1 ? "" : "s"} · ` +
      `<strong>${fmtBytes(data.used)}</strong> of ${fmtBytes(data.max_storage)}`;
    $("#usage").innerHTML = text;
  }

  function renderList(files) {
    const box = $("#file-list");
    box.innerHTML = "";
    $("#empty").classList.toggle("hidden", files.length > 0);

    if (!files.length) return;

    const table = el("table");
    const thead = el("thead");
    const headRow = el("tr");
    for (const t of ["Name", "Size", "Expires", "Downloads", "Actions"]) {
      headRow.appendChild(el("th", null, t));
    }
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = el("tbody");
    for (const f of files) {
      const tr = el("tr");

      const nameCell = el("td");
      const nameLink = el("a", "fname", stripExt(f.name));
      nameLink.href = f.url;
      nameLink.title = "Open";
      nameLink.addEventListener("click", (e) => {
        if (f.protected) { e.preventDefault(); openFile(f); }
      });
      nameCell.appendChild(nameLink);
      tr.appendChild(nameCell);

      tr.appendChild(el("td", "mono", fmtBytes(f.size)));

      const expCell = el("td", null, fmtWhen(f.expires_at));
      if (f.expires_at && !f.expired) expCell.className = "expiring";
      if (f.expired) expCell.className = "expired";
      tr.appendChild(expCell);

      tr.appendChild(el("td", null, f.max_downloads ? `${f.downloads}/${f.max_downloads}` : String(f.downloads)));

      tr.appendChild(buildActions(f));
      tbody.appendChild(tr);
    }
    table.appendChild(tbody);
    box.appendChild(table);
  }

  function buildActions(f) {
    const cell = el("td");
    const actions = el("div", "actions");

    if (f.formats && f.formats.length) {
      if (f.formats.length > 1) {
        actions.appendChild(buildDropdown("Download", f.formats.map((fmt) => ({
          label: `.${fmt.ext}`,
          run: () => downloadFormat(fmt),
        }))));
      } else {
        const fmt = f.formats[0];
        const dl = el("button", null, "Download");
        dl.addEventListener("click", () => downloadFormat(fmt));
        actions.appendChild(dl);
      }
    }

    if (f.convert_available && f.targets && f.targets.length) {
      if (state.converting.has(f.id)) {
        const busy = el("button", null, "Converting…");
        busy.disabled = true;
        actions.appendChild(busy);
      } else {
        const groups = {};
        for (const t of f.targets) {
          const c = t.converter || "convert";
          (groups[c] = groups[c] || []).push(t);
        }
        const opts = [];
        for (const [conv, targets] of Object.entries(groups)) {
          for (const t of targets) {
            opts.push({
              label: `.${t.ext}`,
              run: () => startConvert(f, t.ext, conv),
            });
          }
        }
        actions.appendChild(buildDropdown("Convert", opts));
      }
    }

    const copy = el("button", null, "Copy");
    copy.title = "Copy the share link";
    copy.addEventListener("click", () => copyLink(f));
    actions.appendChild(copy);

    const del = el("button", "danger", "Delete");
    del.addEventListener("click", async () => {
      if (!confirm(`Delete "${f.name}"?`)) return;
      try {
        await api(`/api/files/${f.id}`, { method: "DELETE" });
        toast("Deleted");
        loadFiles();
      } catch (err) {
        toast(`Delete failed: ${err.message}`, true);
      }
    });
    actions.appendChild(del);

    cell.appendChild(actions);
    return cell;
  }

  function buildDropdown(label, options) {
    const wrap = el("div", "dd");
    const btn = el("button", null, `${label} ▾`);
    const menu = el("div", "dd-menu");
    menu.appendChild(el("p", "head", label));
    for (const opt of options) {
      const b = el("button", null, opt.label);
      b.addEventListener("click", () => {
        closeDropdowns();
        opt.run();
      });
      menu.appendChild(b);
    }
    wrap.appendChild(btn);
    wrap.appendChild(menu);
    btn.addEventListener("click", (e) => {
      e.stopPropagation();
      wrap.classList.toggle("open");
    });
    return wrap;
  }

  function closeDropdowns() {
    document.querySelectorAll(".dd.open").forEach((n) => n.classList.remove("open"));
  }
  document.addEventListener("click", closeDropdowns);

  let pwResolve = null;
  function askPassword(name) {
    return new Promise((resolve) => {
      $("#pw-file-name").textContent = name;
      $("#pw-input").value = "";
      $("#password-modal").classList.remove("hidden");
      pwResolve = resolve;
      setTimeout(() => $("#pw-input").focus(), 50);
    });
  }
  $("#pw-ok").addEventListener("click", () => {
    const v = $("#pw-input").value;
    $("#password-modal").classList.add("hidden");
    if (pwResolve) { const r = pwResolve; pwResolve = null; r(v); }
  });
  $("#pw-cancel").addEventListener("click", () => {
    $("#password-modal").classList.add("hidden");
    if (pwResolve) { const r = pwResolve; pwResolve = null; r(null); }
  });
  $("#password-modal").addEventListener("keydown", (e) => {
    if (e.key === "Enter") $("#pw-ok").click();
    if (e.key === "Escape") $("#pw-cancel").click();
  });

  async function downloadFile(target, password) {
    const headers = {};
    if (password) headers["X-File-Password"] = password;
    const res = await fetch(target.url, { headers });
    if (res.status === 401 || res.status === 403) {
      throw new Error(res.status === 401 ? "password required" : "incorrect password");
    }
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const blob = await res.blob();
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = target.name;
    document.body.appendChild(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 4000);
  }

  async function downloadFormat(fmt) {
    if (!fmt.protected) {
      const a = document.createElement("a");
      a.href = fmt.url;
      a.download = "";
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      setTimeout(() => loadFiles(), 1500);
      return;
    }
    const pw = await askPassword(fmt.name);
    if (pw == null) return;
    try {
      await downloadFile(fmt, pw);
      setTimeout(() => loadFiles(), 1500);
    } catch (err) {
      toast(err.message, true);
    }
  }

  async function openFile(f) {
    const pw = await askPassword(f.name);
    if (pw == null) return;
    try {
      const res = await fetch(f.url, { headers: { "X-File-Password": pw } });
      if (res.status === 401 || res.status === 403) { toast("Incorrect password", true); return; }
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const blob = await res.blob();
      const url = URL.createObjectURL(blob);
      window.open(url, "_blank");
      setTimeout(() => URL.revokeObjectURL(url), 30000);
    } catch (err) {
      toast(`Could not open: ${err.message}`, true);
    }
  }

  async function startConvert(f, target, converter) {
    try {
      await api(`/api/files/${f.id}/convert`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ target, converter }),
      });
      state.converting.add(f.id);
      toast(`Converting to .${target}…`);
      loadFiles();
      pollConvert(f.id, target);
    } catch (err) {
      toast(`Convert failed: ${err.message}`, true);
    }
  }

  function pollConvert(id, target) {
    clearTimeout(state.pollTimers[id]);
    const tick = async () => {
      try {
        const j = await api(`/api/convert/${id}`);
        if (j.status === "done") {
          state.converting.delete(id);
          toast(`Converted to .${target} — it is now in the Download menu`);
          loadFiles();
          return;
        }
        if (j.status === "error") {
          state.converting.delete(id);
          loadFiles();
          toast(`Conversion failed: ${j.message}`, true);
          return;
        }
        state.pollTimers[id] = setTimeout(tick, 2000);
      } catch (err) {
        state.converting.delete(id);
        loadFiles();
        toast(`Convert status error: ${err.message}`, true);
      }
    };
    tick();
  }

  function setupUpload() {
    const dz = $("#dropzone");
    const input = $("#file");
    const hint = $("#drop-hint");
    const selected = $("#selected-file");

    const showSelected = (file) => {
      if (file) {
        selected.textContent = `✓ ${file.name}`;
        selected.classList.remove("hidden");
        hint.classList.add("hidden");
        dz.classList.add("has-file");
      } else {
        selected.textContent = "";
        selected.classList.add("hidden");
        hint.classList.remove("hidden");
        dz.classList.remove("has-file");
      }
    };
    input.addEventListener("change", () => showSelected(input.files[0]));

    $("#share-copy").addEventListener("click", async () => {
      const field = $("#share-link");
      if (await copyText(field.value)) {
        toast("Link copied");
      } else {
        field.select();
        toast("Could not copy — the link is selected", true);
      }
    });

    dz.addEventListener("dragover", (e) => { e.preventDefault(); dz.classList.add("drag"); });
    dz.addEventListener("dragleave", () => dz.classList.remove("drag"));
    dz.addEventListener("drop", (e) => {
      e.preventDefault();
      dz.classList.remove("drag");
      if (e.dataTransfer.files.length) {
        input.files = e.dataTransfer.files;
        showSelected(e.dataTransfer.files[0]);
      }
    });

    $("#upload-form").addEventListener("submit", (e) => {
      e.preventDefault();
      const file = input.files[0];
      if (!file) return;

      const fd = new FormData();
      fd.append("ttl", $("#ttl").value);
      fd.append("password", $("#password").value);
      fd.append("max_downloads", $("#max-downloads").value);
      fd.append("file", file);

      const btn = $("#upload-btn");
      const progress = $("#progress");
      const bar = $("#progress-bar");
      btn.disabled = true;
      btn.textContent = "Uploading…";
      bar.style.width = "0%";
      progress.classList.remove("hidden");

      const done = () => {
        btn.disabled = false;
        btn.textContent = "Upload";
        progress.classList.add("hidden");
      };

      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/api/upload");
      xhr.upload.addEventListener("progress", (ev) => {
        if (!ev.lengthComputable) return;
        bar.style.width = `${Math.round((ev.loaded / ev.total) * 100)}%`;
      });
      xhr.addEventListener("load", () => {
        done();
        let body = {};
        try { body = JSON.parse(xhr.responseText); } catch (_) {}
        if (xhr.status >= 200 && xhr.status < 300) {
          toast(`Uploaded: ${body.name || file.name}`);
          input.value = "";
          $("#password").value = "";
          showSelected(null);
          if (body.download_url || body.url) showShare(body);
          loadFiles();
        } else {
          toast(`Upload failed: ${body.error || `HTTP ${xhr.status}`}`, true);
        }
      });
      xhr.addEventListener("error", () => {
        done();
        toast("Upload failed: network error", true);
      });
      xhr.addEventListener("abort", done);
      xhr.send(fd);
    });
  }

  setupUpload();
  loadFiles();
})();
