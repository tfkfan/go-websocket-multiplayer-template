package shared

import "container/heap"

type Heap[T any] struct {
	data heapData[T]
}

type heapData[T any] struct {
	items []T
	less  func(a, b T) bool
}

func NewHeap[T any](less func(a, b T) bool) *Heap[T] {
	if less == nil {
		panic("shared: nil less function")
	}
	h := &Heap[T]{data: heapData[T]{less: less}}
	heap.Init(&h.data)
	return h
}

func (h *Heap[T]) Len() int {
	return h.data.Len()
}

func (h *Heap[T]) Push(v T) {
	heap.Push(&h.data, v)
}

func (h *Heap[T]) Pop() (T, bool) {
	if h.data.Len() == 0 {
		var zero T
		return zero, false
	}
	return heap.Pop(&h.data).(T), true
}

func (h *Heap[T]) Peek() (T, bool) {
	if h.data.Len() == 0 {
		var zero T
		return zero, false
	}
	return h.data.items[0], true
}

func (hd *heapData[T]) Len() int {
	return len(hd.items)
}

func (hd *heapData[T]) Less(i, j int) bool {
	return hd.less(hd.items[i], hd.items[j])
}

func (hd *heapData[T]) Swap(i, j int) {
	hd.items[i], hd.items[j] = hd.items[j], hd.items[i]
}

func (hd *heapData[T]) Push(x any) {
	hd.items = append(hd.items, x.(T))
}

func (hd *heapData[T]) Pop() any {
	n := len(hd.items)
	v := hd.items[n-1]
	var zero T
	hd.items[n-1] = zero
	hd.items = hd.items[:n-1]
	return v
}
