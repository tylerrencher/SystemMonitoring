package alerts

import (
	"sync"
	"testing"
)

func TestNewState_Empty(t *testing.T) {
	s := NewState()
	if got := s.Active(); len(got) != 0 {
		t.Errorf("want empty slice, got %v", got)
	}
}

func TestState_SetAndGet(t *testing.T) {
	s := NewState()
	s.set([]ActiveAlert{
		{Key: "battery_critical", Name: "Battery Critically Low"},
		{Key: "house_overload", Name: "House Power Overload"},
	})

	got := s.Active()
	if len(got) != 2 {
		t.Fatalf("got %d alerts, want 2", len(got))
	}
	if got[0].Key != "battery_critical" {
		t.Errorf("got[0].Key = %q, want battery_critical", got[0].Key)
	}
	if got[1].Name != "House Power Overload" {
		t.Errorf("got[1].Name = %q, want House Power Overload", got[1].Name)
	}
}

func TestState_ActiveReturnsCopy(t *testing.T) {
	s := NewState()
	s.set([]ActiveAlert{{Key: "battery_critical", Name: "Battery Critically Low"}})

	got := s.Active()
	got[0].Key = "mutated"

	got2 := s.Active()
	if got2[0].Key == "mutated" {
		t.Error("Active() returned a reference to internal state; want a copy")
	}
}

func TestState_SetOverwrites(t *testing.T) {
	s := NewState()
	s.set([]ActiveAlert{{Key: "a", Name: "A"}, {Key: "b", Name: "B"}})
	s.set([]ActiveAlert{{Key: "c", Name: "C"}})

	got := s.Active()
	if len(got) != 1 || got[0].Key != "c" {
		t.Errorf("expected [{c C}], got %v", got)
	}
}

func TestState_SetEmpty(t *testing.T) {
	s := NewState()
	s.set([]ActiveAlert{{Key: "x", Name: "X"}})
	s.set([]ActiveAlert{})

	if got := s.Active(); len(got) != 0 {
		t.Errorf("want empty after set([]), got %v", got)
	}
}

func TestState_ConcurrentAccess(t *testing.T) {
	s := NewState()
	var wg sync.WaitGroup

	for range 50 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			s.set([]ActiveAlert{{Key: "k", Name: "N"}})
		}()
		go func() {
			defer wg.Done()
			_ = s.Active()
		}()
	}
	wg.Wait()
}
