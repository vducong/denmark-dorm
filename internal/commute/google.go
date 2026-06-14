package commute

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const computeRoutesURL = "https://routes.googleapis.com/directions/v2:computeRoutes"

// HTTPDoer is the slice of *http.Client an HTTP-based router needs. Injecting it
// lets tests stub the network instead of calling the real backend.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// googleRouter is the Google Maps Routes API (computeRoutes v2) adapter — one
// implementation of Router. See router.go for the interface and how to add
// another backend.
type googleRouter struct {
	doer   HTTPDoer
	apiKey string
}

// newGoogleRouter builds the Google adapter. A nil doer uses a default HTTP
// client with a request timeout.
func newGoogleRouter(s Settings, doer HTTPDoer) *googleRouter {
	if doer == nil {
		doer = &http.Client{Timeout: 15 * time.Second}
	}
	return &googleRouter{doer: doer, apiKey: s.APIKey}
}

// Minutes issues one computeRoutes call and returns the trip duration in whole
// minutes. An empty routes array (no route by this mode/time) returns "" with a
// nil error — a real answer worth caching, distinct from a failure.
func (g *googleRouter) Minutes(ctx context.Context, leg Leg) (string, error) {
	body := map[string]any{
		"origin":      map[string]any{"address": leg.Origin},
		"destination": map[string]any{"address": leg.Dest},
		"travelMode":  string(leg.Mode),
	}
	switch leg.When.Kind {
	case WhenArrive:
		body["arrivalTime"] = leg.When.At.UTC().Format(time.RFC3339)
	case WhenDepart:
		body["departureTime"] = leg.When.At.UTC().Format(time.RFC3339)
	}

	buf, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, computeRoutesURL, bytes.NewReader(buf))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", g.apiKey)
	// The field mask is mandatory for computeRoutes; without it the API 400s.
	req.Header.Set("X-Goog-FieldMask", "routes.duration")

	resp, err := g.doer.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("routes api status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out struct {
		Routes []struct {
			Duration string `json:"duration"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return "", fmt.Errorf("decode routes response: %w", err)
	}
	if len(out.Routes) == 0 {
		return "", nil
	}
	return durationToMinutes(out.Routes[0].Duration)
}

// durationToMinutes converts a protobuf duration string ("2040s") to whole
// minutes, rounding to nearest. An empty input maps to "".
func durationToMinutes(d string) (string, error) {
	s := strings.TrimSuffix(strings.TrimSpace(d), "s")
	if s == "" {
		return "", nil
	}
	secs, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return "", fmt.Errorf("parse duration %q: %w", d, err)
	}
	return strconv.Itoa(int(math.Round(secs / 60))), nil
}
