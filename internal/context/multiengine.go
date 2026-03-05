package codectx

import (
	"context"
	"os"
)

type MultiEngine struct {
	engines map[string]*Engine
	active  string
}

func NewMultiEngine() *MultiEngine {
	return &MultiEngine{engines: map[string]*Engine{}}
}

func (m *MultiEngine) AddRoot(path string) *Engine {
	if path == "" {
		wd, _ := os.Getwd()
		path = wd
	}
	if e := m.engines[path]; e != nil {
		return e
	}
	e := NewEngine(path)
	_ = e.BuildIndex()
	m.engines[path] = e
	if m.active == "" {
		m.active = path
	}
	return e
}

func (m *MultiEngine) RemoveRoot(path string) {
	delete(m.engines, path)
	if m.active == path {
		m.active = ""
		// pick any remaining as active
		for p := range m.engines {
			m.active = p
			break
		}
	}
}

func (m *MultiEngine) SetActive(path string) *Engine {
	if e := m.engines[path]; e != nil {
		m.active = path
		return e
	}
	return nil
}

func (m *MultiEngine) Active() *Engine {
	if m.active == "" {
		return nil
	}
	return m.engines[m.active]
}

func (m *MultiEngine) Roots() []string {
	var out []string
	for p := range m.engines {
		out = append(out, p)
	}
	return out
}

// Start background indexing/watch for all engines
func (m *MultiEngine) StartBackground(ctx context.Context) {
	for _, e := range m.engines {
		if err := e.StartWatch(ctx); err != nil {
			e.StartPoll(ctx, 30*1000000000)
		}
	}
}
