# Mutation Validation — Live Runtime Results

Runtime validation of the scaffolded mutations in `startgg/mutations.go`, run
against a **private admin-owned test tournament** (never a public event):

- Tournament `2xko TEST SOLO` (`tournament/2xko-test-solo`), id **923152**,
  admin = user 3403147 (DK PAT in `fnox`).
- Event `Double Elim`, id **1648096**, 8 entrants, phase `Bracket`
  (**2317814**, `DOUBLE_ELIMINATION`), pool **3353163**.
- Empty sibling event `2XKO Open` (1648050, 0 entrants) used for
  non-destructive phase create/delete.

All writes were reversed; the bracket was left as found (only the user's own
reported set remained completed).

## Results — all 5 wrappers runtime-verified

| Wrapper | Tested via | Schema return | Result |
|---|---|---|---|
| `SwapSeeds` | raw API | `[Seed]` | ✅ works — **wrapper return type is WRONG** (declares singular `struct{Id}`; must be a slice) |
| `UpdatePhaseSeeding` | Go client | `Phase` | ✅ works — identity remap accepted; nullability concern resolved |
| `UpsertPhase` | Go client | `Phase` | ✅ works (create) — see limitations below |
| `MarkSetCalled` | Go client | `Set` | ✅ works — set state `1 → 6` (called) |
| `ReportSet` | Go client | `[Set]` | ✅ works — set state `6 → 3` (completed), winnerId set |

### Bonus — unwrapped mutations validated (candidates to wrap)

| Mutation | Return | Notes |
|---|---|---|
| `deletePhase` | `Boolean` | Returns `true`; cascades to the phase's groups + schedule. |
| `resetSet` | `Set` (single) | `resetSet(setId, resetDependentSets: true)` restores a reported set to state 1 and undoes the cascade to dependent sets. |

## Findings / fixes for the PR

1. **BUG — `SwapSeeds` return type.** The live API returns `[Seed]`, but the
   wrapper declares `SwapSeeds struct { Id graphql.ID }` (singular). shurcooL
   would fail unmarshaling a JSON array into a struct → the wrapper errors at
   runtime. Fix: `SwapSeeds []struct { Id graphql.ID }`.
2. **`UpsertPhase` discards its result.** Returns only `error`; the created
   `Phase{id,name}` is thrown away, so callers can't get the new `phaseId`.
   Should return the `Phase`.
3. **`UpsertPhase` is create-only.** It never passes `phaseId`, so despite the
   "upsert" name it cannot update an existing phase. Add an optional `phaseId`.
4. **Resolved (was an open risk):** `updatePhaseSeeding`'s shurcooL-inferred
   `[UpdatePhaseSeedInfo!]!` is accepted where the schema declares
   `[UpdatePhaseSeedInfo]!`; `seedNum`-as-`ID` typing also works. No change needed.
5. **Confirmed correct return types:** `reportBracketSet` (`[Set]`),
   `markSetCalled` (`Set`), `updatePhaseSeeding` (`Phase`), `upsertPhase` (`Phase`).

## Set state semantics observed

`Set.state`: **1** = not started · **6** = called · **3** = completed.

## How it was run

Guarded Go integration test `startgg/live_validate_test.go` (skipped unless
`RUN_LIVE=1`), executed via
`fnox exec -- env RUN_LIVE=1 go test -run TestLive -v ./startgg`.
Read-backs and the unwrapped mutations (`deletePhase`, `resetSet`) were issued as
raw GraphQL against `https://api.start.gg/gql/alpha`.
