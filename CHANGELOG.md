# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed

- **Preview bracket IDs** — unfinalized start.gg sets such as
  `preview_3353103_1_1` now render without integer-unmarshal failures, and write
  controls stay disabled until start.gg assigns reportable numeric IDs.
- **`SwapSeeds` return type** — the live API returns `[Seed]` (a list), but the
  struct declared a singular field; shurcooL failed to unmarshal an array into a
  struct at runtime. Changed to a slice (`[]struct{ Id graphql.ID }`).
- **`UpsertPhase` now returns the resulting Phase** — previously returned only
  `error`, discarding the assigned `phaseId`. Now returns `(UpsertedPhase, error)`
  so callers can read the new phase's `Id` and `Name`.

### Added

- **PIN-protected participant contact directory** — the phone UI now lists
  organizer-visible email, phone, and linked Discord/Twitch/Twitter accounts,
  with search plus `mailto:` / `tel:` actions.
- **Registration-email resend bridge** — host-only setup accepts a start.gg
  `gg_session` value or Copy-as-cURL request, retains only the session value in
  memory, and enables confirmed `sendRegistrationEmail(participantId)` actions
  for bracket runners without sharing the browser session.
- **Automatic operator PINs** — server startup generates and prints a six-digit
  PIN unless one is supplied by flag or environment.
- **Completed-match result reversion** — completed sets can be reset from the
  phone UI with an explicit dependent-match warning.
- **`GetEventContacts`** — returns participant contact information and linked
  profile authorizations for an event.
- **`DeletePhase`** — wraps `deletePhase(phaseId)`; returns `(bool, error)`.
  Scope: `tournament.manager`.
- **`ResetSet`** — wraps `resetSet(setId, resetDependentSets)`; cascades to
  dependent sets when `resetDependentSets` is true. Scope: `tournament.reporter`.
- **`docs/mutation-validation.md`** — runtime validation report from live tests
  against a private admin-owned test tournament; documents confirmed return types
  for all seven mutation wrappers.

### Changed

- **`DeletePhase` / `ResetSet` runtime-validated through the Go wrappers** — both
  were confirmed end-to-end against the test tournament with before/after raw
  read-backs (`DeletePhase` create→delete round trip; `ResetSet` report→reset
  back to state 1), not just compile-checked.
- **Docs reflect runtime validation** — `README.md` and `docs/api-capabilities.md`
  no longer describe the mutations as "compile-verified, not run against live
  data"; the README also documents `DeletePhase`/`ResetSet` and the updated
  `UpsertPhase` `(UpsertedPhase, error)` signature.

---

## [0.1.1] - 2026-06-15

### Added

- **Operator CLI** with table/JSON output:
  - `tournament status`
  - `sets list`
  - `stations list`
  - `stations assign`
  - `sets call`
  - `sets progress`
  - `sets report`
- **Local dashboard API server**:
  - `server --addr 127.0.0.1:8787`
  - embedded phone-friendly operator UI at `/`
  - `GET /healthz`
  - `GET /api/tournament/status`
  - `GET /api/sets`
  - `GET /api/stations`
  - authenticated `POST /api/stations/assign`
  - authenticated `POST /api/sets/call`
  - authenticated `POST /api/sets/progress`
  - authenticated `POST /api/sets/report`
  - current/called/in-progress match cards use green active-state styling to
    mirror start.gg bracket colors.
  - upcoming match cards appear once at least one entrant is known, so operators
    can assign/pre-stage players before the opponent is available.
- **Rate-limited/retrying HTTP client** using start.gg's 80 requests / 60 s
  limit, with retries for HTTP 429/5xx responses.
- **`GetEvents`** — list all events (id + name) for a tournament by slug.
- **`GetTournamentStatus`** — tournament/event/phase-group/station discovery
  payload for operator dashboards and CLI status output.
- **`GetStandings`** — top-N final standings for an event, ordered by placement.
- **`GetEntrants`** — one page of event entrants plus total count.
- **`GetPhaseGroupSets`** — bracket-order set list with state, slots, station,
  stream, and score fields for dashboards/OBS/call sheets.
- **`GetTournamentStations`** — station ids and queue metadata for set assignment.
- **`EventInfo`**, **`Standing`**, **`BracketSet`**, **`Station`**, and related
  types to support the new queries.
- **Tournament-management mutations** (compile-verified; not run against live data):
  - `ReportSet` — report winner / DQ on a bracket set (`tournament.reporter`).
  - `MarkSetCalled` — mark a set as called (`tournament.reporter`).
  - `MarkSetInProgress` — mark a set as in progress (`tournament.reporter`).
  - `AssignStation` — assign a set to a station (`tournament.reporter`).
  - `AssignStream` — assign a set to a stream (`tournament.reporter`).
  - `SwapSeeds` — swap two seeds within a phase (`tournament.manager`).
  - `UpdatePhaseSeeding` — rewrite a phase's full seed mapping (`tournament.manager`).
  - `UpsertPhase` — create or update a phase on an existing event (`tournament.manager`).
- **`docs/api-capabilities.md`** — authoritative capability map derived from live
  schema introspection (20 mutations, 27+ documented queries, OAuth scope table,
  rate limits).
- **`CHANGELOG.md`** — this file.
- **Expanded README** — install instructions, authentication options, per-function
  usage examples with realistic output, rate-limit table, capabilities summary.

### Fixed

- **`GetTop8` invalid event ID** — `main.go` called `GetTop8` with a stale
  hardcoded event ID (`727876`) that no longer resolves, so the API returned an
  error and the call panicked; replaced with a valid event ID.
- **`.env` is now optional** — `godotenv.Load()` errors are silently ignored so
  `api_key` can be injected at runtime (e.g. `fnox exec -- go run .`) without
  requiring a `.env` file on disk.

---

## [0.1.0] - 2023-10-29

### Added

- Initial client: `CreateClient`, `GetTournamentIdFromSlug`, `GetTop8`.
- Migrated from a verbose hand-rolled GraphQL HTTP client to
  `github.com/shurcooL/graphql` for typed, struct-driven query execution.

---

[0.1.1]: https://github.com/jacobrlewis/startgg-interface/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/jacobrlewis/startgg-interface/releases/tag/v0.1.0
