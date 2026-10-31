package shared

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
)

func TestSetNewIsEmpty(t *testing.T) {
	set := NewSet[string, int]()

	if got := set.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
	if got := set.List(); len(got) != 0 {
		t.Fatalf("List() = %v, want empty", got)
	}
	if value, ok := set.Get("missing"); ok || value != 0 {
		t.Fatalf("Get(missing) = (%d, %v), want (0, false)", value, ok)
	}
}

func TestSetGenericInstantiations(t *testing.T) {
	type player struct {
		id uuid.UUID
	}

	players := NewSet[uuid.UUID, *player]()
	first := &player{id: uuid.New()}
	if !players.Add(first.id, first) {
		t.Fatal("Add() = false, want true")
	}
	if got, ok := players.Get(first.id); !ok || got != first {
		t.Fatalf("Get() = (%v, %v), want the stored player", got, ok)
	}

	counts := NewSet[string, int]()
	if !counts.Add("clicks", 3) {
		t.Fatal("Add() = false, want true")
	}
	if got, ok := counts.Get("clicks"); !ok || got != 3 {
		t.Fatalf("Get() = (%d, %v), want (3, true)", got, ok)
	}
}

func TestSetAddIfAbsent(t *testing.T) {
	set := NewSet[string, int]()

	if !set.Add("key", 1) {
		t.Fatal("first Add() = false, want true")
	}
	if set.Add("key", 2) {
		t.Fatal("duplicate Add() = true, want false")
	}
	if got, ok := set.Get("key"); !ok || got != 1 {
		t.Fatalf("Get() = (%d, %v), want the original value 1", got, ok)
	}
	if got := set.Len(); got != 1 {
		t.Fatalf("Len() = %d, want 1", got)
	}
}

func TestSetGetMissingKey(t *testing.T) {
	set := NewSet[string, *int]()
	present := 7
	set.Add("present", &present)

	if value, ok := set.Get("present"); !ok || value != &present {
		t.Fatalf("Get(present) = (%v, %v), want the stored pointer", value, ok)
	}
	if value, ok := set.Get("missing"); ok || value != nil {
		t.Fatalf("Get(missing) = (%v, %v), want (nil, false)", value, ok)
	}
}

func TestSetRemove(t *testing.T) {
	set := NewSet[string, int]()
	set.Add("key", 7)

	if value, ok := set.Remove("key"); !ok || value != 7 {
		t.Fatalf("Remove(key) = (%d, %v), want (7, true)", value, ok)
	}
	if _, ok := set.Get("key"); ok {
		t.Fatal("Get() after Remove() reports the key still present")
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
	if value, ok := set.Remove("key"); ok || value != 0 {
		t.Fatalf("Remove(missing) = (%d, %v), want (0, false)", value, ok)
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after removing a missing key = %d, want 0", got)
	}
}

func TestSetRemoveNonComparableValue(t *testing.T) {
	set := NewSet[string, []int]()
	set.Add("slice", []int{1, 2, 3})

	value, ok := set.Remove("slice")
	if !ok || len(value) != 3 {
		t.Fatalf("Remove(slice) = (%v, %v), want the stored slice", value, ok)
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestSetListMembership(t *testing.T) {
	set := NewSet[int, string]()
	set.Add(1, "a")
	set.Add(2, "b")
	set.Add(3, "c")
	set.Remove(2)

	values := set.List()
	if len(values) != 2 {
		t.Fatalf("List() has %d values, want 2: %v", len(values), values)
	}

	seen := make(map[string]bool, len(values))
	for _, value := range values {
		if seen[value] {
			t.Fatalf("List() contains duplicate value %q", value)
		}
		seen[value] = true
	}
	if !seen["a"] || !seen["c"] {
		t.Fatalf("List() = %v, want a and c", values)
	}
	if seen["b"] {
		t.Fatal("List() contains a value removed from the set")
	}
}

func TestSetListIsSnapshot(t *testing.T) {
	set := NewSet[int, string]()
	set.Add(1, "a")
	set.Add(2, "b")

	values := set.List()
	for i := range values {
		values[i] = "changed"
	}

	listed := set.List()
	seen := make(map[string]bool, len(listed))
	for _, value := range listed {
		seen[value] = true
	}
	if len(listed) != 2 || !seen["a"] || !seen["b"] || seen["changed"] {
		t.Fatalf("List() after changing a previous listing = %v, want a and b", listed)
	}
	if got := set.Len(); got != 2 {
		t.Fatalf("Len() = %d, want 2", got)
	}
}

func TestSetClear(t *testing.T) {
	set := NewSet[string, int]()
	for i := 0; i < 3; i++ {
		set.Add(fmt.Sprintf("key-%d", i), i)
	}

	removed := set.Clear()
	if len(removed) != 3 {
		t.Fatalf("Clear() returned %d values, want 3: %v", len(removed), removed)
	}
	seen := make(map[int]bool, len(removed))
	for _, value := range removed {
		seen[value] = true
	}
	for i := 0; i < 3; i++ {
		if !seen[i] {
			t.Fatalf("Clear() did not return value %d: %v", i, removed)
		}
	}

	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after Clear() = %d, want 0", got)
	}
	if got := set.List(); len(got) != 0 {
		t.Fatalf("List() after Clear() = %v, want empty", got)
	}
	if !set.Add("after-clear", 42) {
		t.Fatal("Add() after Clear() = false, want true")
	}
}

func TestSetClearEmpty(t *testing.T) {
	set := NewSet[string, int]()

	removed := set.Clear()
	if len(removed) != 0 {
		t.Fatalf("Clear() on empty set returned %v, want no values", removed)
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() = %d, want 0", got)
	}
}

func TestSetConcurrentMixedOperations(t *testing.T) {
	set := NewSet[int, int]()

	const (
		workers    = 8
		iterations = 500
		keySpace   = 64
	)

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := (worker*iterations + i) % keySpace
				switch i % 6 {
				case 0:
					set.Add(key, key)
				case 1:
					set.Get(key)
				case 2:
					set.Remove(key)
				case 3:
					set.Len()
				case 4:
					set.List()
				case 5:
					if key%16 == 0 {
						set.Clear()
					}
				}
			}
		}(worker)
	}
	wg.Wait()

	listed := set.List()
	if got, want := set.Len(), len(listed); got != want {
		t.Fatalf("Len() = %d, want %d to match the listing", got, want)
	}
	for _, value := range listed {
		stored, ok := set.Get(value)
		if !ok || stored != value {
			t.Fatalf("Get(%d) = (%d, %v), want the listed value", value, stored, ok)
		}
	}
}
