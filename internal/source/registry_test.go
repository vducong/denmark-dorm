package source

import (
	"context"
	"testing"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/model"
)

type fakeSource struct{}

func (fakeSource) Descriptor() Descriptor                { return Descriptor{Name: "fake"} }
func (fakeSource) Fetch(context.Context) (string, error) { return "", nil }
func (fakeSource) Parse(string) (*model.Result, error)   { return &model.Result{}, nil }
func (fakeSource) RankOrder(string) (int, bool)          { return 0, false }

func TestRegistry(t *testing.T) {
	Register("fake", func(config.SourceSettings) (Source, error) { return fakeSource{}, nil })

	found := false
	for _, n := range Names() {
		if n == "fake" {
			found = true
		}
	}
	if !found {
		t.Fatalf("Names() = %v, missing fake", Names())
	}

	s, err := New("fake", config.SourceSettings{})
	if err != nil || s == nil {
		t.Fatalf("New(fake) = %v, %v", s, err)
	}

	if _, err := New("nope", config.SourceSettings{}); err == nil {
		t.Error("expected error for unknown source")
	}
}
