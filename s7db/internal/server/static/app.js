(() => {
  const groupsEl = document.getElementById("groups");
  const tpl = document.getElementById("tag-card-template");
  const plcHostEl = document.getElementById("plc-host");
  const plcStateEl = document.getElementById("plc-state");
  const hbStateEl = document.getElementById("hb-state");
  const hbAgeEl = document.getElementById("hb-age");

  const tagEls = new Map();
  const tagDefs = new Map();
  let ws;
  let reconnectMs = 500;

  async function init() {
    const schemaRes = await fetch("/api/v1/schema");
    const schemaPayload = await schemaRes.json();
    render(schemaPayload.tags || []);
    plcHostEl.textContent = `PLC: ${location.hostname}`;
    connect();
    setInterval(refreshHealth, 1000);
  }

  async function refreshHealth() {
    try {
      const r = await fetch("/api/v1/health");
      const h = await r.json();
      plcStateEl.className = `lamp ${h.plc ? "run" : "fault"}`;
      plcStateEl.textContent = h.plc ? "CONNECTED" : "DOWN";
      hbStateEl.className = `lamp ${h.heartbeat?.fault ? "fault" : h.heartbeat?.running ? "run" : "stop"}`;
      hbStateEl.textContent = "HB";
      hbAgeEl.textContent = h.heartbeat?.last_beat ? `last beat: ${h.heartbeat.last_beat}` : "last beat: -";
    } catch (_) {}
  }

  function connect() {
    // Client-side reconnect with exponential backoff (500ms..5s).
    const proto = location.protocol === "https:" ? "wss" : "ws";
    ws = new WebSocket(`${proto}://${location.host}/ws`);
    ws.onopen = () => {
      reconnectMs = 500;
      ws.send(JSON.stringify({ op: "sub", tags: ["*"] }));
    };
    ws.onclose = () => {
      setTimeout(connect, reconnectMs);
      reconnectMs = Math.min(5000, reconnectMs * 2);
    };
    ws.onmessage = (ev) => {
      const msg = JSON.parse(ev.data);
      if (msg.op === "snap" && Array.isArray(msg.tags)) {
        msg.tags.forEach(updateTag);
      } else if (msg.op === "upd") {
        updateTag(msg);
      } else if (msg.op === "ack") {
        if (msg.ok) {
          updateTag({ name: msg.name, raw: msg.raw, value: msg.value });
        } else {
          console.error("write rejected:", msg.err || "unknown");
        }
      } else if (msg.op === "hb") {
        hbStateEl.className = `lamp ${msg.fault ? "fault" : msg.running ? "run" : "stop"}`;
        hbAgeEl.textContent = msg.last_beat ? `last beat: ${msg.last_beat}` : "last beat: -";
      }
    };
  }

  function render(tags) {
    const byGroup = {};
    for (const tag of tags) {
      if ((tag.ui?.widget || "").toLowerCase() === "hide") {
        continue;
      }
      const group = tag.ui?.group || "General";
      byGroup[group] = byGroup[group] || [];
      byGroup[group].push(tag);
      tagDefs.set(tag.name, tag);
    }
    groupsEl.innerHTML = "";
    Object.entries(byGroup).forEach(([groupName, groupTags]) => {
      const plate = document.createElement("section");
      plate.className = "group";
      plate.innerHTML = `<h2>${groupName.toUpperCase()}</h2><div class="group-grid"></div>`;
      const grid = plate.querySelector(".group-grid");
      groupTags.forEach((tag) => grid.appendChild(renderTagCard(tag)));
      groupsEl.appendChild(plate);
    });
  }

  function renderTagCard(tag) {
    const node = tpl.content.firstElementChild.cloneNode(true);
    const title = node.querySelector(".tag-title");
    const widget = node.querySelector(".widget");
    const addrText = node.querySelector(".addr-text");
    const copyBtn = node.querySelector(".copy-btn");
    title.textContent = `${tag.name} (${tag.type})`;
    addrText.textContent = tag.addr || "-";
    copyBtn.addEventListener("click", async () => {
      const value = tag.addr || "";
      if (!value) return;
      try {
        if (navigator.clipboard?.writeText) {
          await navigator.clipboard.writeText(value);
        } else {
          const ta = document.createElement("textarea");
          ta.value = value;
          document.body.appendChild(ta);
          ta.select();
          document.execCommand("copy");
          ta.remove();
        }
        copyBtn.textContent = "✓";
        setTimeout(() => {
          copyBtn.textContent = "⧉";
        }, 700);
      } catch (_) {}
    });
    const valueLine = node.querySelector(".value-line");
    const rawLine = node.querySelector(".raw-line");
    valueLine.textContent = "value: -";
    rawLine.textContent = "raw: -";
    const mode = (tag.ui?.mode || "toggle").toLowerCase();
    const widgetType = (tag.ui?.widget || "number").toLowerCase();

    if (widgetType === "status") {
      const lamp = document.createElement("div");
      lamp.className = "status-lamp";
      widget.appendChild(lamp);
      node._lamp = lamp;
    } else if (widgetType === "button") {
      const btn = document.createElement("button");
      btn.className = "push";
      btn.textContent = "CMD";
      btn.addEventListener("click", () => {
        if (mode === "toggle") {
          sendSet(tag.name, !(tag._lastValue === true));
        } else if (mode === "set-true") {
          sendSet(tag.name, true);
        } else if (mode === "set-false") {
          sendSet(tag.name, false);
        }
      });
      btn.addEventListener("pointerdown", () => {
        if (mode === "momentary") sendSet(tag.name, true);
      });
      const up = () => {
        if (mode === "momentary") sendSet(tag.name, false);
      };
      btn.addEventListener("pointerup", up);
      btn.addEventListener("pointerleave", up);
      widget.appendChild(btn);
      node._btn = btn;
    } else if (widgetType === "gauge") {
      const gauge = document.createElement("div");
      gauge.className = "gauge";
      gauge.innerHTML = `<div class="gauge-fill" style="height:0%"></div>`;
      widget.appendChild(gauge);
      node._gaugeFill = gauge.querySelector(".gauge-fill");
    } else if (widgetType === "knob") {
      const wrap = document.createElement("div");
      wrap.className = "knob-wrap";
      wrap.innerHTML = `<svg class="dial" viewBox="0 0 80 80"><circle cx="40" cy="40" r="36"></circle><line x1="40" y1="40" x2="40" y2="14"></line></svg>`;
      const numeric = document.createElement("input");
      numeric.type = "number";
      if (tag.ui?.min != null) numeric.min = String(tag.ui.min);
      if (tag.ui?.max != null) numeric.max = String(tag.ui.max);
      numeric.step = tag.ui?.step != null ? String(tag.ui.step) : "1";
      numeric.addEventListener("change", () => sendSet(tag.name, Number(numeric.value)));
      const slider = document.createElement("input");
      slider.type = "range";
      slider.min = tag.ui?.min != null ? String(tag.ui.min) : "0";
      slider.max = tag.ui?.max != null ? String(tag.ui.max) : "100";
      slider.step = tag.ui?.step != null ? String(tag.ui.step) : "1";
      slider.addEventListener("input", () => {
        numeric.value = slider.value;
        sendSet(tag.name, Number(slider.value));
        rotateDial(wrap.querySelector("line"), slider);
      });
      const col = document.createElement("div");
      col.style.width = "100%";
      col.appendChild(numeric);
      col.appendChild(slider);
      wrap.appendChild(col);
      widget.appendChild(wrap);
      node._knobInput = numeric;
      node._knobRange = slider;
      node._dialLine = wrap.querySelector("line");
    } else if (widgetType === "text") {
      const txt = document.createElement("input");
      txt.type = "text";
      let pending = null;
      const sendNow = () => {
        if (pending) {
          clearTimeout(pending);
          pending = null;
        }
        sendSet(tag.name, txt.value);
      };
      txt.addEventListener("input", () => {
        if (pending) clearTimeout(pending);
        pending = setTimeout(() => {
          pending = null;
          sendSet(tag.name, txt.value);
        }, 300);
      });
      txt.addEventListener("change", sendNow);
      txt.addEventListener("keydown", (ev) => {
        if (ev.key === "Enter") sendNow();
      });
      widget.appendChild(txt);
      node._text = txt;
    } else {
      const n = document.createElement("input");
      n.type = "number";
      if (tag.ui?.min != null) n.min = String(tag.ui.min);
      if (tag.ui?.max != null) n.max = String(tag.ui.max);
      n.step = tag.ui?.step != null ? String(tag.ui.step) : "1";
      n.addEventListener("change", () => sendSet(tag.name, Number(n.value)));
      widget.appendChild(n);
      node._num = n;
    }

    tagEls.set(tag.name, { node, valueLine, rawLine, tag });
    return node;
  }

  function rotateDial(line, range) {
    const min = Number(range.min || 0);
    const max = Number(range.max || 100);
    const v = Number(range.value || 0);
    const ratio = max === min ? 0 : (v - min) / (max - min);
    const angle = -130 + ratio * 260;
    const rad = (angle * Math.PI) / 180;
    const x = 40 + Math.cos(rad) * 26;
    const y = 40 + Math.sin(rad) * 26;
    line.setAttribute("x2", x.toFixed(1));
    line.setAttribute("y2", y.toFixed(1));
  }

  function updateTag(msg) {
    const entry = tagEls.get(msg.name);
    if (!entry) return;
    const { node, valueLine, rawLine, tag } = entry;
    tag._lastValue = msg.value;
    valueLine.textContent = `value: ${msg.value ?? "-"}`;
    rawLine.textContent = `raw: ${msg.raw ?? "-"}`;
    if (node._lamp) node._lamp.className = `status-lamp ${msg.value ? "on" : ""}`;
    if (node._btn) node._btn.className = `push ${msg.value ? "on" : ""}`;
    if (node._num) node._num.value = msg.value ?? "";
    if (node._text && document.activeElement !== node._text) node._text.value = msg.value ?? "";
    if (node._knobInput) node._knobInput.value = msg.value ?? "";
    if (node._knobRange) {
      node._knobRange.value = msg.value ?? 0;
      rotateDial(node._dialLine, node._knobRange);
    }
    if (node._gaugeFill) {
      const min = Number(tag.ui?.min ?? 0);
      const max = Number(tag.ui?.max ?? 100);
      const val = Number(msg.value ?? 0);
      const pct = Math.max(0, Math.min(100, ((val - min) / (max - min || 1)) * 100));
      node._gaugeFill.style.height = `${pct}%`;
    }
  }

  function sendSet(name, value) {
    if (!ws || ws.readyState !== WebSocket.OPEN) return;
    const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
    ws.send(JSON.stringify({ op: "set", id, name, value }));
  }

  init();
})();
