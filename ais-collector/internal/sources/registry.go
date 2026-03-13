package sources

import (
	"fmt"
	"log"
	"sync"
)

// SourceFactory is a function that creates a Source from a Config.
type SourceFactory func(config Config) (Source, error)

// Registry holds registered source factories so new AIS data endpoints
// can be added with a single Register call.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]SourceFactory
}

// NewRegistry creates an empty source registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]SourceFactory),
	}
}

// Register adds a source factory under the given name.
// Call this for each data endpoint you want to support.
func (r *Registry) Register(name string, factory SourceFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[name] = factory
}

// Create builds a Source from the registry using the given name and config.
func (r *Registry) Create(name string, config Config) (Source, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown source type %q; registered types: %v", name, r.ListRegistered())
	}
	return factory(config)
}

// ListRegistered returns all registered source type names.
func (r *Registry) ListRegistered() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	return names
}

// DefaultRegistry is the global registry with all built-in sources pre-registered.
var DefaultRegistry = NewRegistry()

func init() {
	// Register all built-in source types.
	// To add a new data endpoint, create a file implementing Source and add
	// a Register call here (or call DefaultRegistry.Register from your own init).

	DefaultRegistry.Register("digitraffic", func(cfg Config) (Source, error) {
		return NewDigitTrafficSource(cfg), nil
	})

	DefaultRegistry.Register("mock", func(cfg Config) (Source, error) {
		return NewMockSource(cfg), nil
	})

	DefaultRegistry.Register("aishub", func(cfg Config) (Source, error) {
		return NewAISHubSource(cfg), nil
	})

	DefaultRegistry.Register("aisstream", func(cfg Config) (Source, error) {
		return NewAISStreamSource(cfg)
	})
}

// SourceDef describes a source to create from configuration (env vars, config file, etc.).
type SourceDef struct {
	// Type is the registered source type name (e.g. "digitraffic", "aishub").
	Type string
	// Config holds connection parameters for this source instance.
	Config Config
	// Enabled controls whether this source is active.
	Enabled bool
}

// CreateSources builds all enabled Source instances from a slice of definitions
// using the given registry. Sources that fail to create are logged and skipped.
func CreateSources(registry *Registry, defs []SourceDef, logger *log.Logger) []Source {
	var sources []Source
	for _, def := range defs {
		if !def.Enabled {
			logger.Printf("Source %q (%s) is disabled, skipping", def.Config.Name, def.Type)
			continue
		}

		src, err := registry.Create(def.Type, def.Config)
		if err != nil {
			logger.Printf("Failed to create source %q: %v", def.Config.Name, err)
			continue
		}

		logger.Printf("Initialized source: %s (type=%s)", def.Config.Name, def.Type)
		sources = append(sources, src)
	}
	return sources
}
