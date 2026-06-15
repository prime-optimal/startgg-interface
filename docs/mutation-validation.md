# Mutation Validation — Live Runtime Results

Runtime validation of the mutation wrappers in `startgg/mutations.go`, run
against a **private admin-owned test tournament** (never a public event):

- Tournament `2xko TEST SOLO` (`tournament/2xko-test-solo`), id **923152**,
  admin = user 3403147 (DK PAT in `fnox`).
- Event `Double Elim`, id **1648096**, 8 entrants, phase `Bracket`
  (**2317814**, `DOUBLE_ELIMINATION`), pool **3353163**.
- Empty sibling event `2XKO Open` (1648050, 0 entrants) used for
  non-destructive phase create/delete.

All writes were reversed; the bracket was left as found (only the user's own
reported set remained completed).

## Results — all 7 wrappers runtime-verified

| Wrapper | Tested via | Schema return | Result |
|---|---|---|---|
| `SwapSeeds` | raw API | `[Seed]` | ✅ works — return type corrected to a slice (was singular `struct{Id}`; see fix #1) |
| `UpdatePhaseSeeding` | Go client | `Phase` | ✅ works — identity remap accepted; nullability concern resolved |
| `UpsertPhase` | Go client | `Phase` | ✅ works (create) — now returns `(UpsertedPhase, error)` so the new `phaseId` is usable |
| `MarkSetCalled` | Go client | `Set` | ✅ works — set state `1 → 6` (called) |
| `ReportSet` | Go client | `[Set]` | ✅ works — set state `→ 3` (completed), winnerId set |
| `DeletePhase` | Go client | `Boolean` | ✅ works — returns `(true, nil)`; cascades to the phase's groups + schedule |
| `ResetSet` | Go client | `Set` | ✅ works — `resetSet(setId, resetDependentSets: true)` restores a reported set to state 1, clears the winner, and undoes the cascade |

### `DeletePhase` / `ResetSet` — Go-wrapper round trips

These two were first proven via raw GraphQL, then confirmed end-to-end through
the Go wrappers with before/after raw read-backs:

- **`DeletePhase`** — created throwaway phases on the empty event 1648050 via the
  improved `UpsertPhase` (using the returned `phase.Id` directly), then deleted
  them. The wrapper returned `(true, nil)` and a read-back confirmed the event was
  back to its single original `Bracket` phase (2317762) — the phases were really
  removed, not just reported deleted.
- **`ResetSet`** — `ReportSet` advanced test set 104220600 to state 3 (winner
  set); `ResetSet(104220600, resetDependentSets: true)` returned it to
  **state 1 with the winner cleared**, verified by a fresh raw read (the set
  carried no cache hint).

## Fixes applied (landed in PR #4)

1. **`SwapSeeds` return type.** The live API returns `[Seed]`, but the wrapper
   declared `SwapSeeds struct { Id graphql.ID }` (singular). shurcooL fails to
   unmarshal a JSON array into a struct → the wrapper errored at runtime.
   Fixed to a slice (`[]struct { Id graphql.ID }`).
2. **`UpsertPhase` now returns its result.** Previously returned only `error`,
   discarding the created `Phase{id,name}`; now returns
   `(UpsertedPhase, error)` so callers can read the new `phaseId`.

## Confirmed correct (no change needed)

- `updatePhaseSeeding`'s shurcooL-inferred `[UpdatePhaseSeedInfo!]!` is accepted
  where the schema declares `[UpdatePhaseSeedInfo]!`; `seedNum`-as-`ID` typing also
  works.
- Return types: `reportBracketSet` (`[Set]`), `markSetCalled` (`Set`),
  `updatePhaseSeeding` (`Phase`), `upsertPhase` (`Phase`), `deletePhase`
  (`Boolean`), `resetSet` (`Set`).

## Known limitation (not yet addressed)

- **`UpsertPhase` is create-only.** It never passes `phaseId`, so despite the
  "upsert" name it cannot update an existing phase. Adding an optional `phaseId`
  would make it a true upsert.

## Set state semantics observed

`Set.state`: **1** = not started · **6** = called · **3** = completed.

## How it was run

Guarded Go integration test `startgg/live_validate_test.go` (skipped unless
`RUN_LIVE=1`), executed via
`fnox exec -- env RUN_LIVE=1 go test -run TestLive -v ./startgg`.
Read-backs were issued as raw GraphQL against `https://api.start.gg/gql/alpha`.
