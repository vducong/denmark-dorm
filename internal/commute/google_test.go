package commute

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// fakeDoer records each request (and its body) and returns a programmable
// response, so tests exercise the Google adapter without touching the network.
type fakeDoer struct {
	bodies   []string
	requests []*http.Request
	respond  func(*http.Request) (*http.Response, error)
}

func (f *fakeDoer) Do(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	f.bodies = append(f.bodies, body)
	f.requests = append(f.requests, req)
	if f.respond != nil {
		return f.respond(req)
	}
	return jsonResp(http.StatusOK, `{"routes":[{"duration":"2040s"}]}`), nil
}

func jsonResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestNewRouter(t *testing.T) {
	if _, err := NewRouter("google", Settings{APIKey: "k"}); err != nil {
		t.Errorf("google provider: %v", err)
	}
	if _, err := NewRouter("", Settings{APIKey: "k"}); err != nil {
		t.Errorf("default provider: %v", err)
	}
	if _, err := NewRouter("bogus", Settings{}); err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestDurationToMinutes(t *testing.T) {
	cases := map[string]string{"2040s": "34", "90s": "2", "0s": "0", "": ""}
	for in, want := range cases {
		got, err := durationToMinutes(in)
		if err != nil {
			t.Fatalf("durationToMinutes(%q) err = %v", in, err)
		}
		if got != want {
			t.Errorf("durationToMinutes(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGoogleRouter_transitSendsArrivalAndHeaders(t *testing.T) {
	f := &fakeDoer{}
	g := newGoogleRouter(Settings{APIKey: "secret"}, f)
	got, err := g.Minutes(context.Background(), Leg{Origin: "A", Dest: "B", Mode: ModeTransit,
		When: When{Kind: WhenArrive, At: time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "34" {
		t.Errorf("minutes = %q, want 34", got)
	}
	req := f.requests[0]
	if req.Header.Get("X-Goog-Api-Key") != "secret" {
		t.Errorf("api key header = %q", req.Header.Get("X-Goog-Api-Key"))
	}
	if req.Header.Get("X-Goog-FieldMask") != "routes.duration" {
		t.Errorf("field mask = %q", req.Header.Get("X-Goog-FieldMask"))
	}
	if !strings.Contains(f.bodies[0], `"arrivalTime"`) {
		t.Errorf("body missing arrivalTime: %s", f.bodies[0])
	}
	if strings.Contains(f.bodies[0], `"departureTime"`) {
		t.Errorf("morning body should not carry departureTime: %s", f.bodies[0])
	}
	if !strings.Contains(f.bodies[0], `"travelMode":"TRANSIT"`) {
		t.Errorf("body travelMode missing: %s", f.bodies[0])
	}
}

func TestGoogleRouter_walkOmitsTime(t *testing.T) {
	f := &fakeDoer{}
	g := newGoogleRouter(Settings{APIKey: "k"}, f)
	if _, err := g.Minutes(context.Background(), Leg{Origin: "A", Dest: "B", Mode: ModeWalk}); err != nil {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(f.bodies[0], "Time") {
		t.Errorf("walk body should omit time fields: %s", f.bodies[0])
	}
}

func TestGoogleRouter_noRouteBlank(t *testing.T) {
	f := &fakeDoer{respond: func(*http.Request) (*http.Response, error) {
		return jsonResp(http.StatusOK, `{"routes":[]}`), nil
	}}
	g := newGoogleRouter(Settings{APIKey: "k"}, f)
	got, err := g.Minutes(context.Background(), Leg{Origin: "A", Dest: "B", Mode: ModeTransit,
		When: When{Kind: WhenDepart, At: time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)}})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if got != "" {
		t.Errorf("no-route minutes = %q, want empty", got)
	}
}

func TestGoogleRouter_httpErrorReturnsError(t *testing.T) {
	f := &fakeDoer{respond: func(*http.Request) (*http.Response, error) {
		return jsonResp(http.StatusForbidden, "denied"), nil
	}}
	g := newGoogleRouter(Settings{APIKey: "k"}, f)
	if _, err := g.Minutes(context.Background(), Leg{Origin: "A", Dest: "B", Mode: ModeWalk}); err == nil {
		t.Fatal("expected error on non-200 status")
	}
}
