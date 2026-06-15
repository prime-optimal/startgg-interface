# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/) and
this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Fixed

- **`SwapSeeds` return type** — the live API returns `[Seed]` (a list), but the
  struct declared a singular field; shurcooL failed to unmarshal an array into a
  struct at runtime. Changed to a slice (`[]struct{ Id graphql.ID }`).
- **`UpsertPhase` now returns the resulting Phase** — previously returned only
  `error`, discarding the assigned `phaseId`. Now returns `(UpsertedPhase, error)`
  so callers can read the new phase's `Id` and `Name`.

### Added

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

- **`GetEvents`** — list all events (id + name) for a tournament by slug.
- **`GetStandings`** — top-N final standings for an event, ordered by placement.
- **`GetEntrants`** — one page of event entrants plus total count.
- **`EventInfo`** and **`Standing`** types to support the new queries.
- **Tournament-management mutations** (compile-verified; not run against live data):
  - `ReportSet` — report winner / DQ on a bracket set (`tournament.reporter`).
  - `MarkSetCalled` — mark a set as called (`tournament.reporter`).
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
