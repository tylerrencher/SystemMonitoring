package alerts

import "sync"

// ActiveAlert is an alert whose condition is currently true.
type ActiveAlert struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

// State is shared between the Engine and the API server.
// The engine writes on each tick; the dashboard handler reads it.
type State struct {
	mu     sync.RWMutex
	active []ActiveAlert
}

func NewState() *State {
	return &State{active: []ActiveAlert{}}
}

func (s *State) set(active []ActiveAlert) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = active
}

// Active returns a snapshot of currently active alerts.
func (s *State) Active() []ActiveAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ActiveAlert, len(s.active))
	copy(out, s.active)
	return out
}
