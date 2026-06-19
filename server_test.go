package main

import (
	"context"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"jacobrlewis/startgg-interface/startgg"
)

func TestGenerateOperatorPIN(t *testing.T) {
	pin, err := generateOperatorPIN()
	if err != nil {
		t.Fatalf("generateOperatorPIN() error = %v", err)
	}
	if !regexp.MustCompile(`^[0-9]{6}$`).MatchString(pin) {
		t.Fatalf("generateOperatorPIN() = %q, want six digits", pin)
	}
}

func TestExtractWebSession(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "bare", input: "0123456789abcdef0123456789abcdef", want: "0123456789abcdef0123456789abcdef"},
		{name: "curl", input: `curl https://www.start.gg -b 'other=value; gg_session=session_value_123456; more=value'`, want: "session_value_123456"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := extractWebSession(test.input)
			if err != nil {
				t.Fatalf("extractWebSession() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("extractWebSession() = %q, want %q", got, test.want)
			}
		})
	}
	if _, err := extractWebSession("not a cookie"); err == nil {
		t.Fatal("extractWebSession() accepted invalid input")
	}
}

func TestContactRows(t *testing.T) {
	entrant := startgg.ContactEntrant{Id: 7, Name: "Team Name"}
	participant := startgg.ParticipantContact{Id: 11, GamerTag: "Player"}
	participant.Email = "player@example.com"
	participant.ContactInfo.Name = "Real Name"
	participant.ContactInfo.PhoneNumber = "555-0100"
	participant.User.Authorizations = []startgg.ProfileAuthorization{{
		Type:             "DISCORD",
		ExternalId:       "123",
		ExternalUsername: "player",
	}}
	entrant.Participants = []startgg.ParticipantContact{participant}

	rows := contactRows([]startgg.ContactEntrant{entrant})
	if len(rows) != 1 {
		t.Fatalf("contactRows() returned %d rows, want 1", len(rows))
	}
	if rows[0].ParticipantId != 11 || rows[0].Accounts[0].Type != "DISCORD" {
		t.Fatalf("contactRows() = %#v", rows[0])
	}
}

func TestSendRegistrationEmail(t *testing.T) {
	server := &apiServer{
		webSession: "session_value_123456",
		httpClient: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.String() != "https://www.start.gg/api/-/gql" {
				t.Fatalf("request URL = %s", req.URL)
			}
			if req.Header.Get("Cookie") != "gg_session=session_value_123456" {
				t.Fatalf("Cookie header = %q", req.Header.Get("Cookie"))
			}
			body, err := io.ReadAll(req.Body)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), `"participantId":42`) {
				t.Fatalf("request body = %s", body)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"data":{"sendRegistrationEmail":true}}`)),
				Header:     make(http.Header),
			}, nil
		})},
	}
	if err := server.sendRegistrationEmail(context.Background(), 42); err != nil {
		t.Fatalf("sendRegistrationEmail() error = %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}
