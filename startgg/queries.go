package startgg

import (
	"context"
	"strconv"

	"github.com/shurcooL/graphql"
)

// These read queries assemble the data you'd render onto a "tournament page":
// the list of events, final standings, and the entrant list. They require only
// a standard personal access token (no admin rights).

// GetEvents returns the events (id + name) for a tournament given its slug.
func (c SGGClient) GetEvents(slug string) []EventInfo {
	var query struct {
		Tournament struct {
			Events []EventInfo
		} `graphql:"tournament(slug: $slug)"`
	}
	variables := map[string]any{
		"slug": graphql.String(slug),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.Tournament.Events
}

// GetTournamentStatus returns tournament, event, phase-group, and station
// discovery data for operator dashboards and CLI status output.
func (c SGGClient) GetTournamentStatus(slug string) TournamentStatus {
	var query struct {
		Tournament TournamentStatus `graphql:"tournament(slug: $slug)"`
	}
	variables := map[string]any{
		"slug": graphql.String(slug),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.Tournament
}

// GetStandings returns the top `n` placements for an event, ordered by placement.
func (c SGGClient) GetStandings(eventId int, n int) []Standing {
	var query struct {
		Event struct {
			Standings struct {
				Nodes []Standing
			} `graphql:"standings(query: {page: $page, perPage: $perPage})"`
		} `graphql:"event(id: $eventId)"`
	}
	variables := map[string]any{
		"eventId": graphql.ID(strconv.Itoa(eventId)),
		"page":    graphql.Int(1),
		"perPage": graphql.Int(n),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.Event.Standings.Nodes
}

// GetEntrants returns one page of an event's entrants along with the total count.
func (c SGGClient) GetEntrants(eventId int, perPage int) ([]Entrant, int) {
	var query struct {
		Event struct {
			Entrants struct {
				PageInfo struct {
					Total int
				}
				Nodes []Entrant
			} `graphql:"entrants(query: {page: $page, perPage: $perPage})"`
		} `graphql:"event(id: $eventId)"`
	}
	variables := map[string]any{
		"eventId": graphql.ID(strconv.Itoa(eventId)),
		"page":    graphql.Int(1),
		"perPage": graphql.Int(perPage),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.Event.Entrants.Nodes, query.Event.Entrants.PageInfo.Total
}

// GetPhaseGroupSets returns one page of sets from a phase group, ordered in
// bracket order. This is the useful primitive for monitors, OBS overlays, and
// operations consoles because it includes state, player slots, station, stream,
// and score display fields.
func (c SGGClient) GetPhaseGroupSets(phaseGroupId int, page int, perPage int) ([]BracketSet, int) {
	var query struct {
		PhaseGroup struct {
			Sets struct {
				PageInfo PageInfo
				Nodes    []BracketSet
			} `graphql:"sets(page: $page, perPage: $perPage, sortType: STANDARD)"`
		} `graphql:"phaseGroup(id: $phaseGroupId)"`
	}
	variables := map[string]any{
		"phaseGroupId": graphql.ID(strconv.Itoa(phaseGroupId)),
		"page":         graphql.Int(page),
		"perPage":      graphql.Int(perPage),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.PhaseGroup.Sets.Nodes, query.PhaseGroup.Sets.PageInfo.Total
}

// GetTournamentStations returns stations configured on a tournament. Station
// ids are required when assigning sets to setups or stream stations.
func (c SGGClient) GetTournamentStations(tournamentId int) []Station {
	var query struct {
		Tournament struct {
			Stations struct {
				Nodes []Station
			}
		} `graphql:"tournament(id: $tournamentId)"`
	}
	variables := map[string]any{
		"tournamentId": graphql.ID(strconv.Itoa(tournamentId)),
	}

	if err := c.Client.Query(context.Background(), &query, variables); err != nil {
		panic(err)
	}

	return query.Tournament.Stations.Nodes
}
