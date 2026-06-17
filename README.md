# startgg-interface

A Go client for the [start.gg GraphQL API](https://developer.start.gg/docs/intro).
It wraps the `https://api.start.gg/gql/alpha` endpoint using
[`github.com/shurcooL/graphql`](https://github.com/shurcooL/graphql) and exposes
typed functions for reading tournament data (no auth beyond a personal access token)
and managing brackets/seeding/results inside existing tournaments (admin token
required for mutations).

> **Fork note:** This is a continuation of
> [jacobrlewis/startgg-interface](https://github.com/jacobrlewis/startgg-interface)
> (upstream last updated 2023). New read queries and tournament-management mutations
> have been added; see [CHANGELOG.md](CHANGELOG.md) for the full history.
>
> **Scope:** The start.gg API **cannot create tournaments or events** — those must
> be created in the web UI first. Everything _inside_ an existing event — phases,
> pools, seeding, results — **is** API-controllable. See
> [docs/api-capabilities.md](docs/api-capabilities.md) for the full capability map.

---

## Install

```bash
go get jacobrlewis/startgg-interface
```

Or clone and build locally:

```bash
git clone https://github.com/jacobrlewis/startgg-interface
cd startgg-interface
go build ./...
```

Requires **Go 1.21+**.

---

## Authentication

Generate a personal access token at
[start.gg/admin/profile/developer](https://start.gg/admin/profile/developer).
Tokens expire after one year.

The client reads the token from `os.Getenv("api_key")`. Two ways to supply it:

**Option A — `.env` file** (loaded automatically via
[godotenv](https://github.com/joho/godotenv); the file is optional):

```text
api_key=your_token_here
```

**Option B — runtime injection** (nothing written to disk, good for CI / shared
machines):

```bash
fnox exec -- go run .
# or any other env-injection tool, e.g.:
api_key=your_token_here go run .
```

For mutations that write to a tournament (seeding, result reporting) you need a
token with admin rights on that tournament — either a PAT belonging to a tournament
admin account or an OAuth token with the `tournament.manager` /
`tournament.reporter` scope.

---

## CLI

The repo now includes an operator CLI. Output defaults to tables; pass
`--format json` for dashboard, OBS, Discord, or automation consumers.

```bash
go run . tournament status --slug 2xko-test-solo
go run . sets list --phase-group 3353163 --state pending
go run . stations list --tournament 923152
go run . stations assign --set 104220600 --station 1563263
go run . sets call --set 104220600
go run . sets progress --set 104220600
go run . sets report --set 104220600 --winner 23851793
```

Supported set filters:

| Filter        | start.gg state handling                  |
| ------------- | ---------------------------------------- |
| `pending`     | state `1`                                |
| `called`      | state `2` with no `startedAt`            |
| `in-progress` | state `6`, or state `2` with `startedAt` |
| `done`        | state `3`                                |

Mutation commands write to a live bracket and require tournament admin access.

### Local operator server and phone UI

Run a local JSON server and phone-friendly operator UI for browser dashboards,
OBS overlays, Terminus-style venue displays, and score reporting without
exposing the start.gg token to frontend code:

```bash
STARTGG_OPERATOR_TOKEN=venue-pin go run . server --addr 127.0.0.1:8787
```

Open `http://127.0.0.1:8787/` on the same machine. On phones or other LAN
devices, use the host computer's LAN IP when the server is bound to `0.0.0.0`.

Endpoints:

| Endpoint                                          | Data                                               |
| ------------------------------------------------- | -------------------------------------------------- |
| `GET /healthz`                                    | health check                                       |
| `GET /api/tournament/status?slug=2xko-test-solo`  | tournament, events, phases, phase groups, stations |
| `GET /api/sets?phase_group=3353163&state=pending` | filtered sets; `state=all\|pending\|called\|in-progress\|done` |
| `GET /api/stations?tournament=923152`             | tournament stations                                |
| `POST /api/stations/assign`                       | assign a set to a station                          |
| `POST /api/sets/call`                             | mark a set called                                  |
| `POST /api/sets/progress`                         | mark a set in progress                             |
| `POST /api/sets/report`                           | report a set winner                                |

Mutation endpoints require `Authorization: Bearer <operator-token>` or
`X-Operator-Token: <operator-token>`. If no operator token is configured, write
endpoints are disabled.

When an operator taps Call, Start, Assign, or Win in the phone UI, the browser
shows an immediate "sending to server" message, then a "server confirmed" message
after the backend accepts the write. The UI updates optimistically while start.gg
propagates the official state. The server console logs every accepted mutation
with operation, set id, station/winner id, and requester IP.

Upcoming sets appear as soon as at least one entrant is known, with the other
side shown as "Awaiting opponent". That lets operators send players to their
next station immediately after a result is recorded, even if the opponent is
still finishing another match. Call/Start/Report stay hidden until both entrants
are known.

Side note for venue networks: the same server can become the single backend for
multiple devices so phones, laptops, OBS machines, and venue displays do not all
need the start.gg API key. For LAN access, bind to all interfaces:

```bash
go run . server --addr 0.0.0.0:8787
```

Other devices would call `http://<host-lan-ip>:8787/`. Use a short operator
token for score reporting and rotate it between events.

---

## Library Usage

### Create a client

```go
import "jacobrlewis/startgg-interface/startgg"

client := startgg.CreateClient(os.Getenv("api_key"))
```

The default client serializes GraphQL requests at 750 ms intervals (start.gg's
published limit is 80 requests / 60 s) and retries HTTP 429/5xx responses twice.
Use `CreateClientWithOptions` to tune this for tests or other transports.

---

### Queries (read-only, standard PAT)

#### `GetTournamentIdFromSlug`

Resolve a tournament's friendly URL slug to its numeric ID.

```go
id := client.GetTournamentIdFromSlug("genesis-x")
// genesis-x -> tournament id 517161
```

#### `GetEvents`

List all events (id + name) for a tournament.

```go
events := client.GetEvents("genesis-x")
for _, e := range events {
    fmt.Printf("%d  %s\n", e.Id, e.Name)
}
// 985241  Melee Singles
// 985242  Ultimate Singles
// ...
```

#### `GetStandings`

Top-N final standings for an event, ordered by placement.

```go
standings := client.GetStandings(985241, 8) // Genesis X Melee Singles
for _, s := range standings {
    fmt.Printf("%d. %s\n", s.Placement, s.Entrant.Name)
}
// 1. 69% | Cody Schwab
// 2. Red Bull IFM | aMSa
// 3. Zain
// ...
```

#### `GetEntrants`

One page of entrants for an event, plus the total count.

```go
entrants, total := client.GetEntrants(985241, 25)
fmt.Printf("%d total entrants (showing %d)\n", total, len(entrants))
for _, e := range entrants {
    fmt.Println(e.Name)
}
```

#### `GetTop8`

The top-8 bracket sets for an event, filtered so the losing player placed top 8.

```go
sets := client.GetTop8(985241)
for _, s := range sets {
    fmt.Printf("[%s] %s\n", s.FullRoundText, s.DisplayScore)
}
// [Winners Semi-Final] Sirmeris 2 - Monika | Rocks 0
// [Winners Semi-Final] Grab 2 - technospider 0
// ... (see the known limitation below)
```

> **Known limitation:** `GetTop8` reads `event.sets(page:1, perPage:11,
sortType:STANDARD)` and filters `lPlacement < 8`. For large multi-phase events
> this returns early Winners-side sets rather than the true top-8 bracket. A
> correct implementation should target the final phase group directly.

#### `GetPhaseGroupSets`

List bracket-order sets from a phase group, including state, display score,
entrant slots, station, and stream fields. Use this for live monitors, OBS
overlays, call sheets, and operations consoles.

```go
sets, total := client.GetPhaseGroupSets(3353163, 1, 30)
fmt.Printf("%d total sets\n", total)
for _, s := range sets {
    fmt.Printf("%s %s [%d] station %d\n",
        s.Identifier, s.FullRoundText, s.State, s.Station.Number)
}
```

#### `GetTournamentStations`

List configured stations for a tournament. The returned ids are what
`AssignStation` needs.

```go
stations := client.GetTournamentStations(923152)
for _, station := range stations {
    fmt.Printf("%d station %d enabled=%v\n",
        station.Id, station.Number, station.Enabled)
}
```

---

### Mutations (require an admin token on your tournament)

> These functions are **runtime-validated** against a private admin-owned test
> tournament (see [docs/mutation-validation.md](docs/mutation-validation.md)).
> They write to a live bracket — run them only against a tournament you own.

#### `ReportSet` — scope: `tournament.reporter`

Mark the winner (or a DQ) on a bracket set.

```go
err := client.ReportSet(setId, winnerId, false)
```

#### `MarkSetCalled` — scope: `tournament.reporter`

Mark a set as called (players summoned to their station).

```go
err := client.MarkSetCalled(setId)
```

#### `MarkSetInProgress` — scope: `tournament.reporter`

Mark a set as in progress after players arrive at the station.

```go
err := client.MarkSetInProgress(setId)
```

#### `AssignStation` — scope: `tournament.reporter`

Assign a set to a tournament station. If the station is tied to a stream, start.gg
also exposes that relationship through the set/station query fields.

```go
err := client.AssignStation(setId, stationId)
```

#### `AssignStream` — scope: `tournament.reporter`

Assign a set directly to a stream.

```go
err := client.AssignStream(setId, streamId)
```

#### `SwapSeeds` — scope: `tournament.manager`

Swap two seeds within a phase.

```go
err := client.SwapSeeds(phaseId, seed1Id, seed2Id)
```

#### `UpdatePhaseSeeding` — scope: `tournament.manager`

Rewrite the full seeding of a phase by supplying a `seedId → seedNum` mapping.

```go
mapping := []startgg.UpdatePhaseSeedInfo{
    {SeedId: "1001", SeedNum: "1"},
    {SeedId: "1002", SeedNum: "2"},
}
err := client.UpdatePhaseSeeding(phaseId, mapping)
```

#### `UpsertPhase` — scope: `tournament.manager`

Create a phase on an existing event and get the created phase back (its `Id` and
`Name`). `BracketType` values: `SINGLE_ELIMINATION`, `DOUBLE_ELIMINATION`,
`ROUND_ROBIN`, `SWISS`, `RACE` (and others; see
[docs/api-capabilities.md](docs/api-capabilities.md)). Currently create-only.

```go
phase, err := client.UpsertPhase(eventId, startgg.PhaseUpsertInput{
    Name:        "Top 8",
    GroupCount:  1,
    BracketType: "DOUBLE_ELIMINATION",
})
// phase.Id is the newly assigned phaseId
```

#### `DeletePhase` — scope: `tournament.manager`

Delete a phase (cascades to its pools and schedule). Returns whether the deletion
succeeded.

```go
ok, err := client.DeletePhase(phaseId)
```

#### `ResetSet` — scope: `tournament.reporter`

Reset a reported set back to its unplayed state (state 1, winner cleared). Pass
`true` to cascade the reset to sets fed by this one.

```go
err := client.ResetSet(setId, true)
```

---

## Rate limits

| Limit            | Value                   |
| ---------------- | ----------------------- |
| Requests         | 80 / 60 s               |
| Query complexity | 1 000 objects / request |

---

## Capabilities

See [docs/api-capabilities.md](docs/api-capabilities.md) for the full query and
mutation map, OAuth scope details, and the authoritative list of what the API can
and cannot do.

**Summary:**

- **Cannot** create tournaments or events (web UI only).
- **Can** read all public tournament data with a standard PAT.
- **Can** configure and run an existing tournament's bracket (phases, seeding,
  results) with an admin token.

---

## Local test tournament notes

Read-only API checks were run against private tournament slug `2xko-test-solo`
on 2026-06-15:

| Item        | ID / state                                        |
| ----------- | ------------------------------------------------- |
| Tournament  | `923152`                                          |
| Event       | `1648096` (`Double Elim`, 8 entrants)             |
| Phase       | `2317814` (`Bracket`)                             |
| Phase group | `3353163` (`DOUBLE_ELIMINATION`, state `2`)       |
| Stations    | `1563262`-`1563265`, station numbers 1-4, enabled |

Local Go is pinned to `1.26.4` in mise. Verified after reinstalling the mise Go
toolchain:

```bash
mise exec -- go test ./...
```

---

## License

See the [LICENSE](LICENSE) file in this repository.
