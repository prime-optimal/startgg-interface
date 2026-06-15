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
