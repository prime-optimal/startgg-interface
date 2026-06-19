const els = {
  serverUrl: document.querySelector("#serverUrl"),
  operatorToken: document.querySelector("#operatorToken"),
  tournamentId: document.querySelector("#tournamentId"),
  eventId: document.querySelector("#eventId"),
  phaseGroupId: document.querySelector("#phaseGroupId"),
  connectionStatus: document.querySelector("#connectionStatus"),
  connectButton: document.querySelector("#connectButton"),
  saveButton: document.querySelector("#saveButton"),
  refreshButton: document.querySelector("#refreshButton"),
  matches: document.querySelector("#matches"),
  message: document.querySelector("#message"),
  template: document.querySelector("#matchTemplate"),
  tabs: [...document.querySelectorAll(".tab")],
  sessionSetup: document.querySelector("#sessionSetup"),
  sessionInput: document.querySelector("#sessionInput"),
  configureSessionButton: document.querySelector("#configureSessionButton"),
  sessionStatus: document.querySelector("#sessionStatus"),
};

const defaults = {
  serverUrl: window.location.origin,
  tournamentId: "922256",
  eventId: "1648050",
  phaseGroupId: "3353103",
  view: "current",
};

const state = {
  stations: [],
  sets: [],
  contacts: [],
  contactsLoadedForEvent: null,
  view: localStorage.getItem("startgg_ops_view") || defaults.view,
  refreshTimer: null,
  optimisticPatches: new Map(),
  actionInFlight: false,
};

function loadSettings() {
  els.serverUrl.value = localStorage.getItem("startgg_ops_server") || defaults.serverUrl;
  els.operatorToken.value = localStorage.getItem("startgg_ops_token") || "";
  els.tournamentId.value = localStorage.getItem("startgg_ops_tournament") || defaults.tournamentId;
  els.eventId.value = localStorage.getItem("startgg_ops_event") || defaults.eventId;
  els.phaseGroupId.value = localStorage.getItem("startgg_ops_phase_group") || defaults.phaseGroupId;
  setView(state.view);
}

function saveSettings() {
  localStorage.setItem("startgg_ops_server", cleanBaseURL());
  localStorage.setItem("startgg_ops_token", els.operatorToken.value.trim());
  localStorage.setItem("startgg_ops_tournament", els.tournamentId.value.trim());
  localStorage.setItem("startgg_ops_event", els.eventId.value.trim());
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
    requiredNumber(els.eventId.value, "Event");
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
  if (state.view === "contacts") {
    renderContacts();
    return;
  }
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

async function loadContacts() {
  const eventId = requiredNumber(els.eventId.value, "Event");
  if (state.contactsLoadedForEvent === eventId) {
    renderMatches();
    return;
  }
  clearMessage();
  setBusy(true);
  try {
    state.contacts = (await api(`/api/contacts?event=${eventId}`)) || [];
    state.contactsLoadedForEvent = eventId;
    renderMatches();
  } catch (error) {
    showMessage(error.message, true);
  } finally {
    setBusy(false);
  }
}

function renderContacts() {
  els.matches.textContent = "";
  if (state.contactsLoadedForEvent === null) {
    showMessage("Loading contacts...", false);
    return;
  }
  clearMessage();

  const search = document.createElement("input");
  search.className = "contact-search";
  search.type = "search";
  search.placeholder = "Search name, tag, email, or account";
  search.setAttribute("aria-label", "Search contacts");
  const list = document.createElement("div");
  list.className = "contact-list";
  const draw = () => {
    const needle = search.value.trim().toLowerCase();
    list.textContent = "";
    const contacts = state.contacts.filter((contact) => contactSearchText(contact).includes(needle));
    for (const contact of contacts) list.appendChild(renderContact(contact));
    if (contacts.length === 0) {
      const empty = document.createElement("p");
      empty.className = "empty-state";
      empty.textContent = "No matching contacts";
      list.appendChild(empty);
    }
  };
  search.addEventListener("input", draw);
  els.matches.append(search, list);
  draw();
}

function contactSearchText(contact) {
  return [
    contact.gamer_tag,
    contact.name,
    contact.entrant_name,
    contact.email,
    contact.phone,
    ...(contact.accounts || []).flatMap((account) => [account.type, account.username, account.id]),
  ]
    .filter(Boolean)
    .join(" ")
    .toLowerCase();
}

function renderContact(contact) {
  const card = document.createElement("article");
  card.className = "contact-card";

  const heading = document.createElement("div");
  heading.className = "contact-heading";
  const title = document.createElement("div");
  const tag = document.createElement("h3");
  tag.textContent = contact.gamer_tag || contact.entrant_name || "Unknown player";
  const identity = document.createElement("p");
  identity.textContent = [contact.name, contact.entrant_name !== contact.gamer_tag ? contact.entrant_name : ""]
    .filter(Boolean)
    .join(" · ");
  title.append(tag, identity);
  const participant = document.createElement("span");
  participant.className = "participant-id";
  participant.textContent = `#${contact.participant_id}`;
  heading.append(title, participant);

  const actions = document.createElement("div");
  actions.className = "contact-actions";
  if (contact.email) actions.appendChild(contactLink(`mailto:${contact.email}`, "Email", contact.email));
  if (contact.phone) actions.appendChild(contactLink(`tel:${contact.phone}`, "Call / text", contact.phone));
  for (const account of contact.accounts || []) {
    actions.appendChild(linkedAccountAction(account));
  }

  const resend = document.createElement("button");
  resend.type = "button";
  resend.className = "resend-button";
  resend.textContent = "Re-send registration email";
  const confirm = document.createElement("div");
  confirm.className = "resend-confirm";
  confirm.hidden = true;
  resend.addEventListener("click", () => prepareRegistrationResend(contact, confirm));

  card.append(heading, actions, resend, confirm);
  return card;
}

function contactLink(href, label, value) {
  const link = document.createElement("a");
  link.className = "contact-action";
  link.href = href;
  const strong = document.createElement("strong");
  strong.textContent = label;
  const detail = document.createElement("span");
  detail.textContent = value;
  link.append(strong, detail);
  return link;
}

function linkedAccountAction(account) {
  const label = formatProvider(account.type);
  const value = account.username || account.id || "Connected";
  if (safeExternalURL(account.url)) return contactLink(account.url, label, value);
  const button = document.createElement("button");
  button.type = "button";
  button.className = "contact-action copy-action";
  const strong = document.createElement("strong");
  strong.textContent = label;
  const detail = document.createElement("span");
  detail.textContent = value;
  button.append(strong, detail);
  button.addEventListener("click", async () => {
    try {
      await navigator.clipboard.writeText(value);
      showMessage(`${label} ID copied`, false);
    } catch {
      showMessage(`${label}: ${value}`, false);
    }
  });
  return button;
}

function safeExternalURL(value) {
  if (!value) return false;
  try {
    return ["https:", "http:"].includes(new URL(value).protocol);
  } catch {
    return false;
  }
}

function formatProvider(value) {
  const names = { DISCORD: "Discord", TWITCH: "Twitch", TWITTER: "X / Twitter" };
  return names[value] || String(value || "Account").toLowerCase().replace(/^./, (char) => char.toUpperCase());
}

function prepareRegistrationResend(contact, confirmBox) {
  confirmBox.hidden = false;
  confirmBox.textContent = "";
  const text = document.createElement("div");
  text.className = "confirm-text";
  text.textContent = `Send start.gg's registration email again to ${contact.gamer_tag || contact.name}?`;
  const actions = document.createElement("div");
  actions.className = "confirm-actions";
  const send = document.createElement("button");
  send.className = "confirm-button";
  send.type = "button";
  send.textContent = "Send email";
  send.addEventListener("click", () => resendRegistrationEmail(contact));
  const cancel = document.createElement("button");
  cancel.className = "cancel-button";
  cancel.type = "button";
  cancel.textContent = "Cancel";
  cancel.addEventListener("click", () => {
    confirmBox.hidden = true;
  });
  actions.append(send, cancel);
  confirmBox.append(text, actions);
}

async function resendRegistrationEmail(contact) {
  await postSetAction(
    "/api/contacts/resend-registration",
    { participant_id: contact.participant_id },
    {
      sending: `Requesting registration email for ${contact.gamer_tag || contact.name}...`,
      confirmed: `start.gg confirmed the registration email for ${contact.gamer_tag || contact.name}.`,
    },
  );
}

async function configureWebSession() {
  const value = els.sessionInput.value.trim();
  if (!value) {
    showMessage("Paste a gg_session value or Copy as cURL request", true);
    return;
  }
  setBusy(true);
  try {
    await api("/api/session/startgg", { method: "POST", body: JSON.stringify({ curl: value }) });
    els.sessionInput.value = "";
    els.sessionStatus.textContent = "Configured for this server run";
    showMessage("Registration-email resends enabled", false);
  } catch (error) {
    showMessage(error.message, true);
  } finally {
    setBusy(false);
  }
}

async function refreshWebSessionStatus() {
  try {
    const result = await api("/api/session/startgg");
    els.sessionStatus.textContent = result.configured ? "Configured" : "Not configured";
  } catch (error) {
    els.sessionStatus.textContent = error.message;
  }
}

function filteredSets() {
  if (state.view === "current") {
    return state.sets.filter(
      (set) => ["called", "in-progress"].includes(set.state_label) && hasTwoEntrants(set),
    );
  }
  if (state.view === "completed") {
    return state.sets.filter((set) => set.state_label === "done");
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

  if (!set.id) {
    controls.textContent = "";
    const preview = document.createElement("p");
    preview.className = "preview-note";
    preview.textContent = "Preview match — controls unlock when start.gg finalizes the bracket.";
    controls.appendChild(preview);
    return node;
  }

  if (state.view === "completed") {
    controls.textContent = "";
    const resetButton = document.createElement("button");
    resetButton.className = "reset-button";
    resetButton.type = "button";
    resetButton.textContent = "Revert result";
    const resetConfirm = document.createElement("div");
    resetConfirm.className = "reset-confirm";
    resetConfirm.hidden = true;
    resetButton.addEventListener("click", () => prepareReset(set, resetConfirm));
    controls.append(resetButton, resetConfirm);
    return node;
  }

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

function prepareReset(set, confirmBox) {
  confirmBox.hidden = false;
  confirmBox.textContent = "";

  const text = document.createElement("div");
  text.className = "confirm-text";
  text.textContent = `Revert set ${set.identifier || set.id}? Every later match that depends on this result will also be reset.`;

  const actions = document.createElement("div");
  actions.className = "confirm-actions";
  const confirmButton = document.createElement("button");
  confirmButton.className = "confirm-button";
  confirmButton.type = "button";
  confirmButton.textContent = "Revert result";
  confirmButton.addEventListener("click", () => resetSet(set));
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

async function resetSet(set) {
  await postSetAction(
    "/api/sets/reset",
    { set_id: set.id },
    {
      sending: `Revert tapped: set ${set.identifier || set.id} and dependent matches. Sending to server...`,
      confirmed: `Server confirmed revert for set ${set.identifier || set.id}. Refreshing bracket dependencies...`,
    },
    () =>
      updateSet(set.id, {
        state: 1,
        state_label: "pending",
        display_score: "",
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
  if (view === "contacts") loadContacts();
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
els.configureSessionButton.addEventListener("click", configureWebSession);

for (const tab of els.tabs) {
  tab.addEventListener("click", () => {
    setView(tab.dataset.view);
    saveSettings();
  });
}

loadSettings();
if (["localhost", "127.0.0.1", "::1"].includes(window.location.hostname)) {
  els.sessionSetup.hidden = false;
  refreshWebSessionStatus();
}
connect();
