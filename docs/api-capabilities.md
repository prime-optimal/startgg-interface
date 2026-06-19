# start.gg GraphQL API — Capability Map

Reference notes for the `startgg-interface` Go client. Two sources:

1. **Developer-portal docs** (`developer-portal/docs`) — examples, not exhaustive.
2. **Live schema introspection** against `https://api.start.gg/gql/alpha` — the
   authoritative list. The docs only cover 3 mutations; the schema exposes **20**.

> **Headline:** There is **no `createTournament` / `createEvent` mutation.** A
> tournament and its events must be created in the start.gg web UI first.
> Everything _inside_ an existing event — phases, pools, waves, stations,
> registrations, seeding, results — **is** API-controllable with an admin token.

---

## Authentication

- **Token:** personal access token (PAT) from `start.gg/admin/profile/developer`;
  expires after 1 year. Header: `Authorization: Bearer <token>`.
- **Endpoint:** `POST https://api.start.gg/gql/alpha`, `Content-Type: application/json`,
  body `{"query":"...","variables":{...}}`.
- **Rate limits:** 80 requests / 60s; max 1000 objects (query complexity) per request.
- **Client default:** `CreateClient` serializes requests every 750 ms and retries
  HTTP 429/5xx responses twice. Use `CreateClientWithOptions` to tune transport,
  pacing, or retry behavior.
- **OAuth** (act on behalf of a user): authorization-code grant + refresh tokens.
  - Authorize: `https://start.gg/oauth/authorize?response_type=code&client_id=...&scope=...&redirect_uri=...`
  - Token: `POST api.start.gg/oauth/access_token` · Refresh: `POST api.start.gg/oauth/refresh`

### OAuth scopes (`docs/oauth/scopes.md`)

| Scope                 | Grants                                                  |
| --------------------- | ------------------------------------------------------- |
| `user.identity`       | `currentUser` query + public user fields                |
| `user.email`          | `email` on `currentUser` (requires `user.identity`)     |
| `tournament.manager`  | Seeding + bracket setup for tournaments the user admins |
| `tournament.reporter` | Set reporting for tournaments the user admins           |

**OAuth does not add mutations** — the mutation set is fixed in the schema. Scopes
only gate which existing operations an app may run on a user's behalf.
`tournament.manager` is what authorizes the "bracket setup" subset
(`upsertPhase`, `updatePhaseGroups`, seeding); `tournament.reporter` authorizes
score reporting (`reportBracketSet`).

---

## Mutations — full list (20, from live introspection)

### Bracket / event structure ("setup")

| Mutation            | Args                              | Notes                                                                                      |
| ------------------- | --------------------------------- | ------------------------------------------------------------------------------------------ |
| `upsertPhase`       | `phaseId, eventId, payload`       | **Create or update** a phase. payload = `PhaseUpsertInput{name, groupCount, bracketType}`. |
| `deletePhase`       | `phaseId`                         | Delete a phase.                                                                            |
| `updatePhaseGroups` | `groupConfigs`                    | Update the set of phase groups (pools) in a phase.                                         |
| `upsertWave`        | `waveId, tournamentId, fields`    | Create/update a wave.                                                                      |
| `deleteWave`        | `waveId`                          | Delete a wave.                                                                             |
| `upsertStation`     | `stationId, tournamentId, fields` | Create/update a station.                                                                   |
| `deleteStation`     | `stationId`                       | Delete a station.                                                                          |

`BracketType` enum: `SINGLE_ELIMINATION, DOUBLE_ELIMINATION, ROUND_ROBIN, SWISS,
EXHIBITION, CUSTOM_SCHEDULE, MATCHMAKING, ELIMINATION_ROUNDS, RACE, CIRCUIT`.

### Registration

| Mutation                    | Args                              | Notes                                              |
| --------------------------- | --------------------------------- | -------------------------------------------------- |
| `generateRegistrationToken` | `registration, userId`            | Generate a registration token on behalf of a user. |
| `registerForTournament`     | `registration, registrationToken` | Register for a tournament.                         |

### Seeding

| Mutation                   | Args                            | Notes                                                             |
| -------------------------- | ------------------------------- | ----------------------------------------------------------------- |
| `updatePhaseSeeding`       | `phaseId, seedMapping, options` | Rewrite a phase's seeding (full `[UpdatePhaseSeedInfo]` mapping). |
| `swapSeeds`                | `phaseId, seed1Id, seed2Id`     | Swap two seeds.                                                   |
| `resolveScheduleConflicts` | `tournamentId, options`         | Auto-resolve schedule conflicts; returns changed seeds.           |

### Running brackets / sets

| Mutation            | Args                              | Notes                                                      |
| ------------------- | --------------------------------- | ---------------------------------------------------------- |
| `reportBracketSet`  | `setId, winnerId, isDQ, gameData` | Report winner / per-game stats. winnerId marks complete.   |
| `updateBracketSet`  | `setId, winnerId, isDQ, gameData` | Update game stats (winner cannot change — use `resetSet`). |
| `resetSet`          | `setId, resetDependentSets`       | Reset a set; can cascade to dependent sets.                |
| `markSetCalled`     | `setId`                           | Mark set called.                                           |
| `markSetInProgress` | `setId`                           | Mark set in progress.                                      |
| `assignStation`     | `setId, stationId`                | Assign a set to a station (and its stream, if any).        |
| `assignStream`      | `setId, streamId`                 | Assign a set to a stream.                                  |
| `updateVodUrl`      | `setId, vodUrl`                   | Attach a VOD URL to a set.                                 |

Only `reportBracketSet`, `updatePhaseSeeding`, and `resolveScheduleConflicts`
are documented in the developer portal; the other 17 are undocumented but live.

---

## Queries — documented (27, from `developer-portal/docs/queries` + `examples`)

Root fields: `event`, `tournament`, `tournaments`, `phase`, `phaseGroup`, `set`,
`player`, `league`, `shop`, `videogames`.

**Tournament / event:** `tournament(slug)`, `events-by-tournament`,
`get-event(slug)`, `count-entrants-by-event` (`numEntrants`, `entrantSizeMin`),
`attendee-counts` (`participants.pageInfo.total`), `attendees-by-sponsor`
(participant search), `entrants-by-tournament`.

**Standings / entrants:** `event-standings` (placement + entrant, paginated),
`event-entrants` (paginated, with `pageInfo.total`).

### Organizer contact data

The live schema exposes organizer-authorized participant contact fields through
the public GraphQL endpoint:

- `Participant.email`
- `Participant.contactInfo` (`name`, `phoneNumber`, and address fields)
- `Participant.user.authorizations` (`type`, `externalId`,
  `externalUsername`, `url`) for linked Discord, Twitch, Twitter, and other
  providers

The attendee UI's **Re-send registration email** action uses this undocumented
web-only operation:

```graphql
mutation SendRegistrationEmail($participantId: ID!) {
  sendRegistrationEmail(participantId: $participantId)
}
```

`sendRegistrationEmail` is absent from the PAT/OAuth schema at
`api.start.gg/gql/alpha`; the start.gg web application sends it through its
browser-session GraphQL endpoint. Treat it as brittle and keep browser-session
credentials in memory only.

**Sets:** `sets-in-event`, `sets-in-phase`, `sets-in-phase-group`,
`sets-by-player`, `sets-by-station`, `set-score`, `set-entrants`,
`set-game-data` (per-game stage/character/scores).

**Phases / seeds:** `phase-groups-in-phase`, `phase-seeds`, `pool-seeds`.

**Discovery:** `tournaments-by-owner` (read-only ownership), `tournaments-by-location`
(country / state / geo-radius), `tournaments-by-videogame` (upcoming, sorted),
`videogame-id-by-name`.

**Streaming / league / shop:** `stream-queue`, `league-standings`,
`league-schedule`, `shop`.

---

## What this means for "tournament pages"

- **Render a page from data — fully supported (read-only PAT).** Pull events,
  standings, entrants, sets, seeds, stream queue and render your own page.
  Implemented here: `GetEvents`, `GetStandings`, `GetEntrants`, `GetTop8`,
  `GetTournamentIdFromSlug`.
- **Configure an existing tournament's structure — supported (admin token /
  `tournament.manager`).** Create/update phases, pools, waves, stations; set
  seeding; register participants; report results. Scaffolded here in
  `startgg/mutations.go` (compile-verified, not run against live data).
- **Create the tournament/event itself — NOT supported.** No such mutation
  exists; use the web UI, then drive everything else via the API.

## Local operator API

`go run . server --addr 127.0.0.1:8787` exposes JSON endpoints and an embedded
phone-friendly operator UI at `/`. The start.gg token stays server-side; browsers
call the local API instead of `https://api.start.gg/gql/alpha` directly.

Write endpoints require `Authorization: Bearer <operator-pin>` or
`X-Operator-Token: <operator-pin>`, configured with `--operator-token`,
`STARTGG_OPERATOR_TOKEN`, or generated and printed automatically at startup.

| Endpoint                                  | Backing operation                        |
| ----------------------------------------- | ---------------------------------------- |
| `GET /healthz`                            | local health check                       |
| `GET /api/tournament/status?slug=...`     | `GetTournamentStatus`                    |
| `GET /api/sets?phase_group=...&state=...` | `GetPhaseGroupSets` + local state filter |
| `GET /api/stations?tournament=...`        | `GetTournamentStations`                  |
| `GET /api/contacts?event=...`             | `GetEventContacts`                       |
| `POST /api/stations/assign`               | `AssignStation`                          |
| `POST /api/sets/call`                     | `MarkSetCalled`                          |
| `POST /api/sets/progress`                 | `MarkSetInProgress`                      |
| `POST /api/sets/report`                   | `ReportSet`                              |
| `POST /api/sets/reset`                    | `ResetSet` with dependent-set cascade    |
| `POST /api/contacts/resend-registration`  | web-only `sendRegistrationEmail`         |
| `GET/POST /api/session/startgg`           | host-only in-memory web session setup    |

The embedded phone UI intentionally shows pending/upcoming sets as soon as at
least one entrant has resolved. The unresolved side is displayed as "Awaiting
opponent" so operators can assign the known player to their next station early;
Call/Start/Report controls remain hidden until both entrants are available.

---

## Implemented in this client

| Function                  | Kind     | Operation                                                 | Auth                          |
| ------------------------- | -------- | --------------------------------------------------------- | ----------------------------- |
| `GetTournamentIdFromSlug` | query    | `tournament(slug){id}`                                    | PAT                           |
| `GetTournamentStatus`     | query    | `tournament(slug){events, phases, phaseGroups, stations}` | PAT/admin-visible tournament  |
| `GetEvents`               | query    | `tournament(slug){events}`                                | PAT                           |
| `GetStandings`            | query    | `event.standings`                                         | PAT                           |
| `GetEntrants`             | query    | `event.entrants`                                          | PAT                           |
| `GetEventContacts`        | query    | participant contact info + linked authorizations          | admin PAT                     |
| `GetTop8`                 | query    | `event.sets(sortType: STANDARD)`                          | PAT                           |
| `GetPhaseGroupSets`       | query    | `phaseGroup.sets(sortType: STANDARD)`                     | PAT                           |
| `GetTournamentStations`   | query    | `tournament.stations`                                     | PAT/admin-visible tournament  |
| `ReportSet`               | mutation | `reportBracketSet`                                        | admin / `tournament.reporter` |
| `MarkSetCalled`           | mutation | `markSetCalled`                                           | admin / `tournament.reporter` |
| `MarkSetInProgress`       | mutation | `markSetInProgress`                                       | admin / `tournament.reporter` |
| `AssignStation`           | mutation | `assignStation`                                           | admin / `tournament.reporter` |
| `AssignStream`            | mutation | `assignStream`                                            | admin / `tournament.reporter` |
| `SwapSeeds`               | mutation | `swapSeeds`                                               | admin / `tournament.manager`  |
| `UpdatePhaseSeeding`      | mutation | `updatePhaseSeeding`                                      | admin / `tournament.manager`  |
| `UpsertPhase`             | mutation | `upsertPhase`                                             | admin / `tournament.manager`  |
| `DeletePhase`             | mutation | `deletePhase`                                             | admin / `tournament.manager`  |
| `ResetSet`                | mutation | `resetSet`                                                | admin / `tournament.reporter` |

## Live test notes

On 2026-06-15, read-only GraphQL checks against private tournament slug
`2xko-test-solo` verified the useful dashboard/run-bracket shapes:

- `tournament(slug)` resolves tournament `923152`.
- Event `1648096` has phase `2317814` and phase group `3353163`.
- `phaseGroup(id: 3353163).sets` returns set state, display score, slots,
  station, and stream fields.
- `tournament(id: 923152).stations.nodes` returns stations `1563262`-`1563265`
  with station numbers 1-4.

These reads validate the shape behind `GetPhaseGroupSets` and
`GetTournamentStations`. Mutations remain intentionally unexercised unless a
specific bracket-running action is desired.

### Known limitation

`GetTop8` reads `event.sets(page:1, perPage:11, sortType: STANDARD)` and filters
`lPlacement < 8`. For a large multi-phase event this returns the first 11 sets in
STANDARD order (often early Winners-side sets), **not** the actual top-8 bracket.
A correct implementation should target the final/top-8 phase group rather than the
event-wide set list.
