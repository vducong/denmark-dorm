package source

import (
	"fmt"
	"sort"

	"housing-waitlist/internal/config"
)

// Factory builds a configured Source from its settings.
type Factory func(cfg config.SourceSettings) (Source, error)

var registry = map[string]Factory{}

// Register adds a source factory under name; called from each source's init().
func Register(name string, f Factory) {
	if _, dup := registry[name]; dup {
		panic(fmt.Sprintf("source already registered: %q", name))
	}
	registry[name] = f
}

// New constructs the registered source for name with the given settings.
func New(name string, cfg config.SourceSettings) (Source, error) {
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown source %q (known: %v)", name, Names())
	}
	return f(cfg)
}

// Names returns the registered source names, sorted.
func Names() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
