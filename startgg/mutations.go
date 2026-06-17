package startgg

import (
	"context"
	"strconv"

	"github.com/shurcooL/graphql"
)

// =====================
// Mutations (writes)
// =====================
//
// Every mutation below writes to a LIVE tournament and requires a personal
// access token (or OAuth token) belonging to a user who ADMINS that tournament:
//   - reporting/seeding require the `tournament.reporter` / `tournament.manager`
//     OAuth scope (or an admin PAT).
// There is no `createTournament` / `createEvent` mutation in the schema: the
// tournament and its events must already exist (created via the start.gg web
// UI). Everything *inside* an event — phases, pools, seeding, results — is
// API-controllable through the functions here.
//
// These have NOT been executed against live data during exploration; run them
// only against a test tournament you own.

// id is a small helper to convert an int id into the graphql.ID scalar.
func id(v int) graphql.ID {
	return graphql.ID(strconv.Itoa(v))
}

// ReportSet reports the winner of a head-to-head bracket set (reportBracketSet).
// Passing winnerId marks the set complete. Scope: tournament.reporter.
func (c SGGClient) ReportSet(setId int, winnerId int, isDQ bool) error {
	var mutation struct {
		ReportBracketSet []struct {
			Id graphql.ID
		} `graphql:"reportBracketSet(setId: $setId, winnerId: $winnerId, isDQ: $isDQ)"`
	}
	variables := map[string]any{
		"setId":    id(setId),
		"winnerId": id(winnerId),
		"isDQ":     graphql.Boolean(isDQ),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// SwapSeeds swaps two seeds within a phase (swapSeeds). Scope: tournament.manager.
// The API returns [Seed] (a list); the inner field is a slice accordingly.
func (c SGGClient) SwapSeeds(phaseId int, seed1Id int, seed2Id int) error {
	var mutation struct {
		SwapSeeds []struct {
			Id graphql.ID
		} `graphql:"swapSeeds(phaseId: $phaseId, seed1Id: $seed1Id, seed2Id: $seed2Id)"`
	}
	variables := map[string]any{
		"phaseId": id(phaseId),
		"seed1Id": id(seed1Id),
		"seed2Id": id(seed2Id),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// MarkSetCalled marks a set as called (markSetCalled). Scope: tournament.reporter.
func (c SGGClient) MarkSetCalled(setId int) error {
	var mutation struct {
		MarkSetCalled struct {
			Id graphql.ID
		} `graphql:"markSetCalled(setId: $setId)"`
	}
	variables := map[string]any{
		"setId": id(setId),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// MarkSetInProgress marks a called/pending set as in progress
// (markSetInProgress). Scope: tournament.reporter.
func (c SGGClient) MarkSetInProgress(setId int) error {
	var mutation struct {
		MarkSetInProgress struct {
			Id graphql.ID
		} `graphql:"markSetInProgress(setId: $setId)"`
	}
	variables := map[string]any{
		"setId": id(setId),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// AssignStation assigns a set to a tournament station (assignStation). Use
// GetTournamentStations to resolve station ids. Scope: tournament.reporter.
func (c SGGClient) AssignStation(setId int, stationId int) error {
	var mutation struct {
		AssignStation struct {
			Id graphql.ID
		} `graphql:"assignStation(setId: $setId, stationId: $stationId)"`
	}
	variables := map[string]any{
		"setId":     id(setId),
		"stationId": id(stationId),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// AssignStream assigns a set directly to a stream (assignStream). Scope:
// tournament.reporter.
func (c SGGClient) AssignStream(setId int, streamId int) error {
	var mutation struct {
		AssignStream struct {
			Id graphql.ID
		} `graphql:"assignStream(setId: $setId, streamId: $streamId)"`
	}
	variables := map[string]any{
		"setId":    id(setId),
		"streamId": id(streamId),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// PhaseUpsertInput is the payload for UpsertPhase. BracketType is one of the
// schema enum values, e.g. DOUBLE_ELIMINATION, SINGLE_ELIMINATION, ROUND_ROBIN,
// SWISS, RACE. Fields are sent only when set (omitempty).
type PhaseUpsertInput struct {
	Name        graphql.String `json:"name,omitempty"`
	GroupCount  graphql.Int    `json:"groupCount,omitempty"`
	BracketType graphql.String `json:"bracketType,omitempty"`
}

// UpsertedPhase is the phase returned by UpsertPhase.
type UpsertedPhase struct {
	Id   graphql.ID
	Name graphql.String
}

// UpsertPhase creates (phaseId = 0) or updates a phase on an existing event
// (upsertPhase). This is the closest the API gets to "setting up" a bracket.
// Returns the resulting Phase so callers can read the assigned phaseId.
// Scope: tournament.manager.
func (c SGGClient) UpsertPhase(eventId int, payload PhaseUpsertInput) (UpsertedPhase, error) {
	var mutation struct {
		UpsertPhase struct {
			Id   graphql.ID
			Name graphql.String
		} `graphql:"upsertPhase(eventId: $eventId, payload: $payload)"`
	}
	variables := map[string]any{
		"eventId": id(eventId),
		"payload": payload,
	}
	if err := c.Client.Mutate(context.Background(), &mutation, variables); err != nil {
		return UpsertedPhase{}, err
	}
	return UpsertedPhase{
		Id:   mutation.UpsertPhase.Id,
		Name: mutation.UpsertPhase.Name,
	}, nil
}

// UpdatePhaseSeedInfo is one entry in the seedMapping passed to UpdatePhaseSeeding.
type UpdatePhaseSeedInfo struct {
	SeedId  graphql.ID `json:"seedId"`
	SeedNum graphql.ID `json:"seedNum"`
}

// UpdatePhaseSeeding rewrites the seeding of a phase (updatePhaseSeeding) by
// submitting a full seedId -> seedNum mapping. Scope: tournament.manager.
func (c SGGClient) UpdatePhaseSeeding(phaseId int, seedMapping []UpdatePhaseSeedInfo) error {
	var mutation struct {
		UpdatePhaseSeeding struct {
			Id graphql.ID
		} `graphql:"updatePhaseSeeding(phaseId: $phaseId, seedMapping: $seedMapping)"`
	}
	variables := map[string]any{
		"phaseId":     id(phaseId),
		"seedMapping": seedMapping,
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}

// DeletePhase deletes a phase and its pools (deletePhase); returns whether the
// deletion succeeded. Scope: tournament.manager.
func (c SGGClient) DeletePhase(phaseId int) (bool, error) {
	var mutation struct {
		DeletePhase graphql.Boolean `graphql:"deletePhase(phaseId: $phaseId)"`
	}
	variables := map[string]any{
		"phaseId": id(phaseId),
	}
	if err := c.Client.Mutate(context.Background(), &mutation, variables); err != nil {
		return false, err
	}
	return bool(mutation.DeletePhase), nil
}

// ResetSet resets a set to its unplayed state (resetSet). When
// resetDependentSets is true, the reset cascades to sets fed by this one.
// Scope: tournament.reporter.
func (c SGGClient) ResetSet(setId int, resetDependentSets bool) error {
	var mutation struct {
		ResetSet struct {
			Id    graphql.ID
			State graphql.Int
		} `graphql:"resetSet(setId: $setId, resetDependentSets: $resetDependentSets)"`
	}
	variables := map[string]any{
		"setId":              id(setId),
		"resetDependentSets": graphql.Boolean(resetDependentSets),
	}
	return c.Client.Mutate(context.Background(), &mutation, variables)
}
