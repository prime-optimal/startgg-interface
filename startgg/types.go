package startgg

type Entrant struct {
	Id   int
	Name string
}

type Slot struct {
	Id      string
	Entrant Entrant
}

type Node struct {
	Id            int    `json:"id"`
	LPlacement    int    `json:"lPlacement"`
	FullRoundText string `json:"fullRoundText"`
	DisplayScore  string `json:"displayScore"`
}

type Set struct {
	Id       int
	Nodes    []Node
	PageInfo struct {
		Total int
	}
}

type Event struct {
	Id   int
	Name string `json:"name"`
	Sets Set
}

type Tournament struct {
	Id   int
	Name string
}

// EventInfo is a lightweight event record (id + name) used by GetEvents.
type EventInfo struct {
	Id          int
	Name        string
	Slug        string
	NumEntrants int
}

// Standing is a single placement in an event's standings.
type Standing struct {
	Placement int
	Entrant   Entrant
}

// PageInfo is the pagination metadata returned by start.gg connection fields.
type PageInfo struct {
	Total      int
	TotalPages int
}

// Score is the nested score payload returned on a set slot standing.
type Score struct {
	Value int
}

// StandingStats is the stats payload returned on a set slot standing.
type StandingStats struct {
	Score Score
}

// SlotStanding is the standing payload embedded in a set slot.
type SlotStanding struct {
	Placement int
	Stats     StandingStats
}

// BracketSetSlot is a player/entrant slot inside a bracket set.
type BracketSetSlot struct {
	Id       string
	Entrant  Entrant
	Standing SlotStanding
}

// Stream is a start.gg stream record attached to a station or set.
type Stream struct {
	Id           int
	StreamName   string
	StreamSource string
	Enabled      bool
}

// Station is a start.gg tournament station record.
type Station struct {
	Id            int
	Number        int
	ClusterNumber *int
	Enabled       bool
	CanAutoAssign bool
	QueueDepth    int
	Stream        Stream
}

// BracketSet is a runnable set record suitable for dashboards, station queues,
// OBS overlays, and reporting workflows.
type BracketSet struct {
	Id            int
	Identifier    string
	FullRoundText string
	State         int
	StartedAt     *int
	DisplayScore  string
	Station       Station
	Stream        Stream
	Slots         []BracketSetSlot
}

// PhaseGroupStatus is a lightweight phase-group record for tournament status
// and operator discovery commands.
type PhaseGroupStatus struct {
	Id                int
	DisplayIdentifier string
	State             int
	BracketType       string
}

// PhaseStatus is a lightweight phase record for tournament status.
type PhaseStatus struct {
	Id          int
	Name        string
	PhaseGroups struct {
		Nodes []PhaseGroupStatus
	} `graphql:"phaseGroups(query: {page: 1, perPage: 50})"`
}

// TournamentStatus is the status payload used by CLI/dashboard discovery.
type TournamentStatus struct {
	Id                 int
	Name               string
	Slug               string
	IsRegistrationOpen bool
	Events             []struct {
		Id          int
		Name        string
		Slug        string
		NumEntrants int
		Phases      []PhaseStatus
	}
	Stations struct {
		Nodes []Station
	}
}
