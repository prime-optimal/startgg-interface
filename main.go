package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"jacobrlewis/startgg-interface/startgg"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/joho/godotenv"
)

type outputFormat string

const (
	formatTable outputFormat = "table"
	formatJSON  outputFormat = "json"
)

type tournamentStatusOutput struct {
	Id                 int                 `json:"id"`
	Name               string              `json:"name"`
	Slug               string              `json:"slug"`
	IsRegistrationOpen bool                `json:"is_registration_open"`
	Events             []eventStatusOutput `json:"events"`
	Stations           []stationOutput     `json:"stations"`
}

type eventStatusOutput struct {
	Id          int                 `json:"id"`
	Name        string              `json:"name"`
	Slug        string              `json:"slug"`
	NumEntrants int                 `json:"num_entrants"`
	Phases      []phaseStatusOutput `json:"phases"`
}

type phaseStatusOutput struct {
	Id          int                      `json:"id"`
	Name        string                   `json:"name"`
	PhaseGroups []phaseGroupStatusOutput `json:"phase_groups"`
}

type phaseGroupStatusOutput struct {
	Id                int    `json:"id"`
	DisplayIdentifier string `json:"display_identifier"`
	State             int    `json:"state"`
	StateLabel        string `json:"state_label"`
	BracketType       string `json:"bracket_type"`
}

type setOutput struct {
	Id            int       `json:"id"`
	Identifier    string    `json:"identifier"`
	State         int       `json:"state"`
	StateLabel    string    `json:"state_label"`
	StartedAt     *int      `json:"started_at,omitempty"`
	Round         string    `json:"round"`
	DisplayScore  string    `json:"display_score"`
	StationId     int       `json:"station_id,omitempty"`
	StationNumber int       `json:"station_number,omitempty"`
	StreamId      int       `json:"stream_id,omitempty"`
	StreamName    string    `json:"stream_name,omitempty"`
	Entrants      []entrant `json:"entrants"`
}

type entrant struct {
	Id   int    `json:"id,omitempty"`
	Name string `json:"name"`
}

type stationOutput struct {
	Id            int    `json:"id"`
	Number        int    `json:"number"`
	ClusterNumber *int   `json:"cluster_number,omitempty"`
	Enabled       bool   `json:"enabled"`
	CanAutoAssign bool   `json:"can_auto_assign"`
	QueueDepth    int    `json:"queue_depth"`
	StreamId      int    `json:"stream_id,omitempty"`
	StreamName    string `json:"stream_name,omitempty"`
	StreamSource  string `json:"stream_source,omitempty"`
}

type mutationOutput struct {
	Ok        bool   `json:"ok"`
	Operation string `json:"operation"`
	SetId     int    `json:"set_id,omitempty"`
	StationId int    `json:"station_id,omitempty"`
	WinnerId  int    `json:"winner_id,omitempty"`
	IsDQ      bool   `json:"is_dq,omitempty"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("start.gg API error: %v", r)
		}
	}()

	if len(args) == 0 {
		printUsage(out)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printUsage(out)
		return nil
	case "tournament":
		return runTournament(args[1:], out)
	case "sets":
		return runSets(args[1:], out)
	case "stations":
		return runStations(args[1:], out)
	case "server":
		return runServer(args[1:], out)
	default:
		return fmt.Errorf("unknown command %q\n\nRun `startgg-interface help` for usage.", args[0])
	}
}

func runTournament(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing tournament command: expected `status`")
	}
	switch args[0] {
	case "status":
		fs := flag.NewFlagSet("tournament status", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		slug := fs.String("slug", "", "tournament slug")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *slug == "" {
			return errors.New("missing --slug")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		status := client.GetTournamentStatus(*slug)
		payload := tournamentStatusFromAPI(status)
		return writeTournamentStatus(out, parseFormat(*format), payload)
	default:
		return fmt.Errorf("unknown tournament command %q", args[0])
	}
}

func runSets(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing sets command: expected `list`, `call`, `progress`, or `report`")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("sets list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		phaseGroup := fs.Int("phase-group", 0, "phase group id")
		state := fs.String("state", "all", "all, pending, called, in-progress, done")
		page := fs.Int("page", 1, "page number")
		perPage := fs.Int("per-page", 50, "sets per page")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *phaseGroup == 0 {
			return errors.New("missing --phase-group")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		sets, _ := client.GetPhaseGroupSets(*phaseGroup, *page, *perPage)
		rows, err := filterSetRows(sets, *state)
		if err != nil {
			return err
		}
		return writeSets(out, parseFormat(*format), rows)
	case "call":
		fs := flag.NewFlagSet("sets call", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		setId := fs.Int("set", 0, "set id")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *setId == 0 {
			return errors.New("missing --set")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.MarkSetCalled(*setId); err != nil {
			return err
		}
		return writeMutation(out, parseFormat(*format), mutationOutput{Ok: true, Operation: "sets call", SetId: *setId})
	case "progress":
		fs := flag.NewFlagSet("sets progress", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		setId := fs.Int("set", 0, "set id")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *setId == 0 {
			return errors.New("missing --set")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.MarkSetInProgress(*setId); err != nil {
			return err
		}
		return writeMutation(out, parseFormat(*format), mutationOutput{Ok: true, Operation: "sets progress", SetId: *setId})
	case "report":
		fs := flag.NewFlagSet("sets report", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		setId := fs.Int("set", 0, "set id")
		winnerId := fs.Int("winner", 0, "winner entrant id")
		isDQ := fs.Bool("dq", false, "report as DQ")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *setId == 0 {
			return errors.New("missing --set")
		}
		if *winnerId == 0 {
			return errors.New("missing --winner")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.ReportSet(*setId, *winnerId, *isDQ); err != nil {
			return err
		}
		return writeMutation(out, parseFormat(*format), mutationOutput{Ok: true, Operation: "sets report", SetId: *setId, WinnerId: *winnerId, IsDQ: *isDQ})
	default:
		return fmt.Errorf("unknown sets command %q", args[0])
	}
}

func runStations(args []string, out io.Writer) error {
	if len(args) == 0 {
		return errors.New("missing stations command: expected `list` or `assign`")
	}
	switch args[0] {
	case "list":
		fs := flag.NewFlagSet("stations list", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		tournament := fs.Int("tournament", 0, "tournament id")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *tournament == 0 {
			return errors.New("missing --tournament")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		stations := client.GetTournamentStations(*tournament)
		return writeStations(out, parseFormat(*format), stationRows(stations))
	case "assign":
		fs := flag.NewFlagSet("stations assign", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		setId := fs.Int("set", 0, "set id")
		stationId := fs.Int("station", 0, "station id")
		format := fs.String("format", string(formatTable), "table or json")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if *setId == 0 {
			return errors.New("missing --set")
		}
		if *stationId == 0 {
			return errors.New("missing --station")
		}
		client, err := newClient()
		if err != nil {
			return err
		}
		if err := client.AssignStation(*setId, *stationId); err != nil {
			return err
		}
		return writeMutation(out, parseFormat(*format), mutationOutput{Ok: true, Operation: "stations assign", SetId: *setId, StationId: *stationId})
	default:
		return fmt.Errorf("unknown stations command %q", args[0])
	}
}

func newClient() (startgg.SGGClient, error) {
	_ = godotenv.Load()
	token := firstNonEmpty(os.Getenv("api_key"), os.Getenv("STARTGG_API_KEY"), os.Getenv("START_GG_TOKEN"), os.Getenv("STARTGG_TOKEN"))
	if token == "" {
		return startgg.SGGClient{}, errors.New("api key not set: provide api_key, STARTGG_API_KEY, START_GG_TOKEN, or STARTGG_TOKEN")
	}
	return startgg.CreateClient(token), nil
}

func tournamentStatusFromAPI(status startgg.TournamentStatus) tournamentStatusOutput {
	out := tournamentStatusOutput{
		Id:                 status.Id,
		Name:               status.Name,
		Slug:               status.Slug,
		IsRegistrationOpen: status.IsRegistrationOpen,
		Stations:           stationRows(status.Stations.Nodes),
	}
	for _, event := range status.Events {
		eventOut := eventStatusOutput{
			Id:          event.Id,
			Name:        event.Name,
			Slug:        event.Slug,
			NumEntrants: event.NumEntrants,
		}
		for _, phase := range event.Phases {
			phaseOut := phaseStatusOutput{Id: phase.Id, Name: phase.Name}
			for _, group := range phase.PhaseGroups.Nodes {
				phaseOut.PhaseGroups = append(phaseOut.PhaseGroups, phaseGroupStatusOutput{
					Id:                group.Id,
					DisplayIdentifier: group.DisplayIdentifier,
					State:             group.State,
					StateLabel:        phaseGroupStateLabel(group.State),
					BracketType:       group.BracketType,
				})
			}
			eventOut.Phases = append(eventOut.Phases, phaseOut)
		}
		out.Events = append(out.Events, eventOut)
	}
	return out
}

func filterSetRows(sets []startgg.BracketSet, state string) ([]setOutput, error) {
	normalized := strings.ToLower(strings.TrimSpace(state))
	if normalized == "" {
		normalized = "all"
	}
	if !validSetStateFilter(normalized) {
		return nil, fmt.Errorf("invalid --state %q: expected all, pending, called, in-progress, or done", state)
	}

	rows := make([]setOutput, 0, len(sets))
	for _, set := range sets {
		row := setRow(set)
		if normalized == "all" || row.StateLabel == normalized {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func setRow(set startgg.BracketSet) setOutput {
	row := setOutput{
		Id:            set.Id,
		Identifier:    set.Identifier,
		State:         set.State,
		StateLabel:    setStateLabel(set.State, set.StartedAt),
		StartedAt:     set.StartedAt,
		Round:         set.FullRoundText,
		DisplayScore:  set.DisplayScore,
		StationId:     set.Station.Id,
		StationNumber: set.Station.Number,
		StreamId:      set.Stream.Id,
		StreamName:    set.Stream.StreamName,
	}
	if row.StreamId == 0 && set.Station.Stream.Id != 0 {
		row.StreamId = set.Station.Stream.Id
		row.StreamName = set.Station.Stream.StreamName
	}
	for _, slot := range set.Slots {
		if slot.Entrant.Id == 0 && slot.Entrant.Name == "" {
			continue
		}
		row.Entrants = append(row.Entrants, entrant{Id: slot.Entrant.Id, Name: slot.Entrant.Name})
	}
	return row
}

func stationRows(stations []startgg.Station) []stationOutput {
	rows := make([]stationOutput, 0, len(stations))
	for _, station := range stations {
		rows = append(rows, stationOutput{
			Id:            station.Id,
			Number:        station.Number,
			ClusterNumber: station.ClusterNumber,
			Enabled:       station.Enabled,
			CanAutoAssign: station.CanAutoAssign,
			QueueDepth:    station.QueueDepth,
			StreamId:      station.Stream.Id,
			StreamName:    station.Stream.StreamName,
			StreamSource:  station.Stream.StreamSource,
		})
	}
	return rows
}

func writeTournamentStatus(out io.Writer, format outputFormat, status tournamentStatusOutput) error {
	if format == formatJSON {
		return writeJSON(out, status)
	}
	fmt.Fprintf(out, "Tournament\t%d\t%s\tregistration_open=%v\n", status.Id, status.Name, status.IsRegistrationOpen)
	fmt.Fprintln(out, "\nEvents")
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tENTRANTS\tNAME\tSLUG")
	for _, event := range status.Events {
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\n", event.Id, event.NumEntrants, event.Name, event.Slug)
	}
	tw.Flush()

	fmt.Fprintln(out, "\nPhase groups")
	tw = tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "EVENT\tPHASE\tGROUP\tSTATE\tBRACKET")
	for _, event := range status.Events {
		for _, phase := range event.Phases {
			for _, group := range phase.PhaseGroups {
				fmt.Fprintf(tw, "%s\t%d %s\t%d\t%s\t%s\n", event.Name, phase.Id, phase.Name, group.Id, group.StateLabel, group.BracketType)
			}
		}
	}
	tw.Flush()

	fmt.Fprintln(out, "\nStations")
	return writeStationsTable(out, status.Stations)
}

func writeSets(out io.Writer, format outputFormat, sets []setOutput) error {
	if format == formatJSON {
		return writeJSON(out, sets)
	}
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSET\tSTATE\tSTATION\tROUND\tENTRANTS\tSCORE")
	for _, set := range sets {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%s\n",
			set.Id,
			valueOrDash(set.Identifier),
			set.StateLabel,
			stationLabel(set.StationNumber, set.StationId),
			valueOrDash(set.Round),
			entrantNames(set.Entrants),
			valueOrDash(set.DisplayScore),
		)
	}
	return tw.Flush()
}

func writeStations(out io.Writer, format outputFormat, stations []stationOutput) error {
	if format == formatJSON {
		return writeJSON(out, stations)
	}
	return writeStationsTable(out, stations)
}

func writeStationsTable(out io.Writer, stations []stationOutput) error {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tNUMBER\tENABLED\tAUTO\tQUEUE\tSTREAM")
	for _, station := range stations {
		fmt.Fprintf(tw, "%d\t%d\t%v\t%v\t%d\t%s\n",
			station.Id,
			station.Number,
			station.Enabled,
			station.CanAutoAssign,
			station.QueueDepth,
			valueOrDash(station.StreamName),
		)
	}
	return tw.Flush()
}

func writeMutation(out io.Writer, format outputFormat, mutation mutationOutput) error {
	if format == formatJSON {
		return writeJSON(out, mutation)
	}
	fmt.Fprintf(out, "ok\t%s", mutation.Operation)
	if mutation.SetId != 0 {
		fmt.Fprintf(out, "\tset=%d", mutation.SetId)
	}
	if mutation.StationId != 0 {
		fmt.Fprintf(out, "\tstation=%d", mutation.StationId)
	}
	if mutation.WinnerId != 0 {
		fmt.Fprintf(out, "\twinner=%d", mutation.WinnerId)
	}
	if mutation.IsDQ {
		fmt.Fprint(out, "\tdq=true")
	}
	fmt.Fprintln(out)
	return nil
}

func writeJSON(out io.Writer, payload any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

func parseFormat(value string) outputFormat {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "json":
		return formatJSON
	default:
		return formatTable
	}
}

func setStateLabel(state int, startedAt *int) string {
	switch state {
	case 1:
		return "pending"
	case 2:
		if startedAt != nil && *startedAt != 0 {
			return "in-progress"
		}
		return "called"
	case 3:
		return "done"
	case 6:
		return "in-progress"
	default:
		return fmt.Sprintf("state-%d", state)
	}
}

func phaseGroupStateLabel(state int) string {
	switch state {
	case 1:
		return "pending"
	case 2:
		return "active"
	case 3:
		return "done"
	default:
		return fmt.Sprintf("state-%d", state)
	}
}

func validSetStateFilter(state string) bool {
	switch state {
	case "all", "pending", "called", "in-progress", "done":
		return true
	default:
		return false
	}
}

func entrantNames(entrants []entrant) string {
	if len(entrants) == 0 {
		return "-"
	}
	names := make([]string, 0, len(entrants))
	for _, entrant := range entrants {
		names = append(names, entrant.Name)
	}
	return strings.Join(names, " vs ")
}

func stationLabel(number int, id int) string {
	if id == 0 && number == 0 {
		return "-"
	}
	if number == 0 {
		return fmt.Sprintf("%d", id)
	}
	return fmt.Sprintf("%d (%d)", number, id)
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func printUsage(out io.Writer) {
	fmt.Fprint(out, `startgg-interface

Usage:
  startgg-interface tournament status --slug <slug> [--format table|json]
  startgg-interface sets list --phase-group <id> [--state all|pending|called|in-progress|done] [--format table|json]
  startgg-interface stations list --tournament <id> [--format table|json]
  startgg-interface stations assign --set <id> --station <id> [--format table|json]
  startgg-interface sets call --set <id> [--format table|json]
  startgg-interface sets progress --set <id> [--format table|json]
  startgg-interface sets report --set <id> --winner <entrant-id> [--dq] [--format table|json]
  startgg-interface server [--addr 127.0.0.1:8787] [--operator-token <token>] [--allow-origin *]

Authentication:
  Set api_key, STARTGG_API_KEY, START_GG_TOKEN, or STARTGG_TOKEN.
  A local .env file with api_key=... is loaded when present.
  Server write endpoints require --operator-token or STARTGG_OPERATOR_TOKEN.
`)
}
