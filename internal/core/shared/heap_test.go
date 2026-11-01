package shared

import "testing"

func TestHeapEmpty(t *testing.T) {
	h := NewHeap(func(a, b int) bool {
		return a < b
	})
	if h.Len() != 0 {
		t.Fatalf("len = %d, want 0", h.Len())
	}
	if v, ok := h.Peek(); ok {
		t.Fatalf("Peek on an empty heap returned %v", v)
	}
	if v, ok := h.Pop(); ok {
		t.Fatalf("Pop on an empty heap returned %v", v)
	}
}

func TestHeapOrder(t *testing.T) {
	h := NewHeap(func(a, b int) bool {
		return a < b
	})

	values := []int{5, 1, 9, 3, 7, 3, 0, 8, 2, 6, 4, 100, -5, 42}
	for _, v := range values {
		h.Push(v)
	}
	if h.Len() != len(values) {
		t.Fatalf("len = %d, want %d", h.Len(), len(values))
	}

	var prev int
	first := true
	for h.Len() > 0 {
		v, ok := h.Pop()
		if !ok {
			t.Fatal("Pop failed while Len > 0")
		}
		if !first && v < prev {
			t.Fatalf("popped %d after %d", v, prev)
		}
		prev = v
		first = false
	}
	if _, ok := h.Pop(); ok {
		t.Fatal("Pop on an exhausted heap succeeded")
	}
}

func TestHeapFIFOTies(t *testing.T) {
	type item struct {
		key int
		seq int
	}
	h := NewHeap(func(a, b item) bool {
		if a.key == b.key {
			return a.seq < b.seq
		}
		return a.key < b.key
	})

	for i := range 100 {
		h.Push(item{key: 1, seq: i})
	}
	for i := range 100 {
		got, ok := h.Pop()
		if !ok {
			t.Fatalf("missing item %d", i)
		}
		if got.seq != i {
			t.Fatalf("tie order: got seq %d, want %d", got.seq, i)
		}
	}
}
