const els = {
  serverUrl: document.querySelector("#serverUrl"),
  operatorToken: document.querySelector("#operatorToken"),
  tournamentId: document.querySelector("#tournamentId"),
  phaseGroupId: document.querySelector("#phaseGroupId"),
  connectionStatus: document.querySelector("#connectionStatus"),
  connectButton: document.querySelector("#connectButton"),
  saveButton: document.querySelector("#saveButton"),
  refreshButton: document.querySelector("#refreshButton"),
  matches: document.querySelector("#matches"),
  message: document.querySelector("#message"),
  template: document.querySelector("#matchTemplate"),
  tabs: [...document.querySelectorAll(".tab")],
};

const defaults = {
  serverUrl: window.location.origin,
  tournamentId: "923152",
  phaseGroupId: "3353163",
  view: "current",
};

const state = {
  stations: [],
  sets: [],
  view: localStorage.getItem("startgg_ops_view") || defaults.view,
  refreshTimer: null,
  optimisticPatches: new Map(),
  actionInFlight: false,
};

function loadSettings() {
  els.serverUrl.value = localStorage.getItem("startgg_ops_server") || defaults.serverUrl;
  els.operatorToken.value = localStorage.getItem("startgg_ops_token") || "";
  els.tournamentId.value = localStorage.getItem("startgg_ops_tournament") || defaults.tournamentId;
  els.phaseGroupId.value = localStorage.getItem("startgg_ops_phase_group") || defaults.phaseGroupId;
  setView(state.view);
}

function saveSettings() {
  localStorage.setItem("startgg_ops_server", cleanBaseURL());
  localStorage.setItem("startgg_ops_token", els.operatorToken.value.trim());
  localStorage.setItem("startgg_ops_tournament", els.tournamentId.value.trim());
  localStorage.setItem("startgg_ops_phase_group", els.phaseGroupId.value.trim());
  localStorage.setItem("startgg_ops_view", state.view);
}

function cleanBaseURL() {
  return els.serverUrl.value.trim().replace(/\/+$/, "") || window.location.origin;
}

async function api(path, options = {}) {
  const token = els.operatorToken.value.trim();
  const headers = {
    Accept: "application/json",
    ...options.headers,
  };
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (token) headers.Authorization = `Bearer ${token}`;

  const response = await fetch(`${cleanBaseURL()}${path}`, {
    ...options,
    headers,
  });
  const text = await response.text();
  const payload = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(payload?.error || `HTTP ${response.status}`);
  }
  return payload;
}

async function connect(options = {}) {
  const silent = options?.silent === true;
  saveSettings();
  if (!silent) {
    clearMessage();
    setBusy(true);
  }
  try {
    await api("/healthz");
    const tournamentId = requiredNumber(els.tournamentId.value, "Tournament");
    const phaseGroupId = requiredNumber(els.phaseGroupId.value, "Phase group");
    const [stations, sets] = await Promise.all([
      api(`/api/stations?tournament=${tournamentId}`),
      api(`/api/sets?phase_group=${phaseGroupId}&state=all&per_page=50`),
    ]);
    state.stations = stations || [];
    state.sets = applyOptimisticPatches(sets || []);
    els.connectionStatus.textContent = `Connected · ${state.sets.length} sets · ${state.stations.length} stations`;
    renderMatches();
  } catch (error) {
    els.connectionStatus.textContent = "Disconnected";
    showMessage(error.message, true);
  } finally {
    if (!silent) {
      setBusy(false);
    }
  }
}

function requiredNumber(value, label) {
  const trimmed = value.trim();
  if (!trimmed || Number(trimmed) <= 0) throw new Error(`${label} required`);
  return encodeURIComponent(trimmed);
}

function renderMatches() {
  els.matches.textContent = "";
  const matches = filteredSets();
  if (matches.length === 0) {
    showMessage(state.view === "current" ? "No current matches" : "No upcoming matches", false);
    return;
  }
  clearMessage();
  for (const set of matches) {
    els.matches.appendChild(renderMatch(set));
  }
}

function filteredSets() {
  if (state.view === "current") {
    return state.sets.filter(
      (set) => ["called", "in-progress"].includes(set.state_label) && hasTwoEntrants(set),
    );
  }
  return state.sets.filter((set) => set.state_label === "pending" && hasAnyEntrant(set));
}

function renderMatch(set) {
  const node = els.template.content.firstElementChild.cloneNode(true);
  const entrants = realEntrants(set);
  const isReady = entrants.length >= 2;
  node.classList.toggle("is-current", ["called", "in-progress"].includes(set.state_label));
  node.classList.toggle("is-in-progress", set.state_label === "in-progress");
  node.classList.toggle("is-called", set.state_label === "called");
  node.classList.toggle("is-waiting", set.state_label === "pending" && !isReady);

  const title = node.querySelector(".round");
  title.textContent = "";
  const code = document.createElement("span");
  code.className = "match-code";
  code.textContent = set.identifier || String(set.id);
  title.appendChild(code);
  title.append(document.createTextNode(set.round || "Match"));

  node.querySelector(".meta").textContent =
    `Set ${set.identifier || set.id} · Station ${set.station_number || "-"}`;
  const stateBadge = node.querySelector(".state");
  stateBadge.textContent = set.state_label;
  stateBadge.classList.toggle("done", set.state_label === "done");
  stateBadge.classList.toggle("in-progress", set.state_label === "in-progress");
  stateBadge.classList.toggle("called", set.state_label === "called");

  const players = node.querySelector(".players");
  players.textContent = "";
  for (const entrant of entrants) {
    const row = document.createElement("div");
    row.className = "player-row";
    row.textContent = entrant.name || "Unknown";
    players.appendChild(row);
  }
  while (entrants.length > 0 && players.children.length < 2) {
    const row = document.createElement("div");
    row.className = "player-row is-placeholder";
    row.textContent = "Awaiting opponent";
    players.appendChild(row);
  }

  const stationSelect = node.querySelector(".station-select");
  stationSelect.appendChild(new Option("Unassigned", ""));
  for (const station of state.stations) {
    stationSelect.appendChild(new Option(`Station ${station.number}`, station.id));
  }
  if (set.station_id) stationSelect.value = String(set.station_id);

  node
    .querySelector(".assign-button")
    .addEventListener("click", () => assignStation(set, stationSelect));
  const callButton = node.querySelector(".call-button");
  const progressButton = node.querySelector(".progress-button");
  callButton.addEventListener("click", () => callSet(set));
  progressButton.addEventListener("click", () => progressSet(set));

  const reportButton = document.createElement("button");
  reportButton.className = "report-button";
  reportButton.type = "button";
  reportButton.textContent = "Report result";
  const reportChoices = document.createElement("div");
  reportChoices.className = "report-choices";
  reportChoices.hidden = true;
  const reportConfirm = document.createElement("div");
  reportConfirm.className = "report-confirm";
  reportConfirm.hidden = true;
  for (const entrant of entrants) {
    const button = document.createElement("button");
    button.className = "winner-button";
    button.type = "button";
    button.innerHTML = `<span>${escapeHTML(entrant.name || "Unknown")}</span><span>Report win</span>`;
    button.addEventListener("click", () => prepareReport(set, entrant, reportConfirm));
    reportChoices.appendChild(button);
  }
  reportButton.addEventListener("click", () => {
    reportChoices.hidden = !reportChoices.hidden;
    reportConfirm.hidden = true;
  });

  const controls = node.querySelector(".match-controls");
  controls.appendChild(reportButton);
  controls.appendChild(reportChoices);
  controls.appendChild(reportConfirm);

  if (state.view === "upcoming") {
    reportButton.hidden = true;
    reportChoices.hidden = true;
    reportConfirm.hidden = true;
  }
  if (!isReady) {
    callButton.hidden = true;
    progressButton.hidden = true;
  }
  if (set.state_label === "called") {
    callButton.hidden = true;
  }
  if (set.state_label === "in-progress") {
    callButton.hidden = true;
    progressButton.hidden = true;
  }
  return node;
}

function realEntrants(set) {
  return (set.entrants || []).filter((entrant) => entrant.id && entrant.name);
}

function hasAnyEntrant(set) {
  return realEntrants(set).length >= 1;
}

function hasTwoEntrants(set) {
  return realEntrants(set).length >= 2;
}

function prepareReport(set, entrant, confirmBox) {
  if (!entrant.id) {
    showMessage("Cannot report a result without an entrant id", true);
    return;
  }
  confirmBox.hidden = false;
  confirmBox.textContent = "";

  const text = document.createElement("div");
  text.className = "confirm-text";
  text.textContent = `Report ${entrant.name} as winner of set ${set.identifier || set.id}?`;

  const actions = document.createElement("div");
  actions.className = "confirm-actions";

  const confirmButton = document.createElement("button");
  confirmButton.className = "confirm-button";
  confirmButton.type = "button";
  confirmButton.textContent = "Confirm result";
  confirmButton.addEventListener("click", () => reportWinner(set, entrant));

  const cancelButton = document.createElement("button");
  cancelButton.className = "cancel-button";
  cancelButton.type = "button";
  cancelButton.textContent = "Cancel";
  cancelButton.addEventListener("click", () => {
    confirmBox.hidden = true;
  });

  actions.append(confirmButton, cancelButton);
  confirmBox.append(text, actions);
}

async function reportWinner(set, entrant) {
  if (!entrant.id) {
    showMessage("Cannot report a result without an entrant id", true);
    return;
  }
  await postSetAction(
    "/api/sets/report",
    { set_id: set.id, winner_id: entrant.id },
    {
      sending: `Report tapped: ${entrant.name} wins set ${set.identifier || set.id}. Sending to server...`,
      confirmed: `Server confirmed result: ${entrant.name} wins set ${set.identifier || set.id}. Waiting for start.gg refresh...`,
    },
    () =>
      updateSet(set.id, {
        state: 3,
        state_label: "done",
        display_score: `Reported winner: ${entrant.name}`,
      }),
  );
}

async function assignStation(set, stationSelect) {
  const stationId = Number(stationSelect.value);
  if (!stationId) {
    showMessage("Select a station", true);
    return;
  }
  const station = state.stations.find((candidate) => candidate.id === stationId);
  await postSetAction(
    "/api/stations/assign",
    { set_id: set.id, station_id: stationId },
    {
      sending: `Assign tapped: set ${set.identifier || set.id} to station ${station?.number || stationId}. Sending to server...`,
      confirmed: `Server confirmed station ${station?.number || stationId} for set ${set.identifier || set.id}.`,
    },
    () =>
      updateSet(set.id, {
        station_id: stationId,
        station_number: station?.number || 0,
      }),
  );
}

async function callSet(set) {
  await postSetAction(
    "/api/sets/call",
    { set_id: set.id },
    {
      sending: `Call tapped: set ${set.identifier || set.id}. Sending to server...`,
      confirmed: `Server confirmed call for set ${set.identifier || set.id}.`,
    },
    () =>
      updateSet(set.id, {
        state: 2,
        state_label: "called",
      }),
  );
}

async function progressSet(set) {
  await postSetAction(
    "/api/sets/progress",
    { set_id: set.id },
    {
      sending: `Start tapped: set ${set.identifier || set.id}. Sending to server...`,
      confirmed: `Server confirmed start for set ${set.identifier || set.id}.`,
    },
    () =>
      updateSet(set.id, {
        state: 6,
        state_label: "in-progress",
      }),
  );
}

async function postSetAction(path, body, messages, optimisticUpdate) {
  if (state.actionInFlight) {
    showMessage("Another action is still sending. Wait for server confirmation.", true);
    return;
  }
  saveSettings();
  state.actionInFlight = true;
  showMessage(messages.sending, false);
  try {
    const response = await api(path, {
      method: "POST",
      body: JSON.stringify(body),
    });
    console.log("Server confirmed mutation", response);
    if (optimisticUpdate) optimisticUpdate();
    renderMatches();
    showMessage(messages.confirmed, false);
    scheduleRefresh(7000);
  } catch (error) {
    console.error("Mutation failed", error);
    showMessage(error.message, true);
  } finally {
    state.actionInFlight = false;
  }
}

function updateSet(setId, patch, ttlMs = 20000) {
  state.optimisticPatches.set(setId, {
    patch,
    expiresAt: Date.now() + ttlMs,
  });
  state.sets = state.sets.map((set) => (set.id === setId ? { ...set, ...patch } : set));
}

function applyOptimisticPatches(sets) {
  const now = Date.now();
  for (const [setId, entry] of state.optimisticPatches.entries()) {
    if (entry.expiresAt <= now) {
      state.optimisticPatches.delete(setId);
    }
  }
  return sets.map((set) => {
    const entry = state.optimisticPatches.get(set.id);
    return entry ? { ...set, ...entry.patch } : set;
  });
}

function scheduleRefresh(delayMs = 7000) {
  window.clearTimeout(state.refreshTimer);
  state.refreshTimer = window.setTimeout(() => {
    connect({ silent: true });
  }, delayMs);
}

function setView(view) {
  state.view = view;
  for (const tab of els.tabs) {
    tab.classList.toggle("active", tab.dataset.view === view);
  }
  renderMatches();
}

function showMessage(text, isError) {
  els.message.hidden = false;
  els.message.textContent = text;
  els.message.classList.toggle("error", isError);
}

function clearMessage() {
  els.message.hidden = true;
  els.message.textContent = "";
  els.message.classList.remove("error");
}

function setBusy(isBusy) {
  for (const button of document.querySelectorAll("button")) {
    button.disabled = isBusy;
  }
}

function escapeHTML(value) {
  return String(value).replace(
    /[&<>"']/g,
    (char) =>
      ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
      })[char],
  );
}

els.connectButton.addEventListener("click", connect);
els.refreshButton.addEventListener("click", connect);
els.saveButton.addEventListener("click", () => {
  saveSettings();
  showMessage("Saved", false);
});

for (const tab of els.tabs) {
  tab.addEventListener("click", () => {
    setView(tab.dataset.view);
    saveSettings();
  });
}

loadSettings();
connect();
