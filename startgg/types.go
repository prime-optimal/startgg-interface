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
	Id   int
	Name string
}

// Standing is a single placement in an event's standings.
type Standing struct {
	Placement int
	Entrant   Entrant
}
