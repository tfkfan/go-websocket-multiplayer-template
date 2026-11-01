package server

import (
	"testing"
	"time"
)

func TestPendingQueueEnqueueOrder(t *testing.T) {
	queue := newPendingQueue()
	queue.Enqueue(queuedPending{pending: newFakePending("first")})
	queue.Enqueue(queuedPending{pending: newFakePending("second")})
	queue.Enqueue(queuedPending{pending: newFakePending("third")})

	if queue.Len() != 3 {
		t.Fatalf("length = %d, want 3", queue.Len())
	}

	batch := queue.TakeBatch(3)
	if len(batch) != 3 {
		t.Fatalf("batch size = %d, want 3", len(batch))
	}
	for index, key := range []string{"first", "second", "third"} {
		if batch[index].pending.Key() != key {
			t.Fatalf("batch order = %s at %d, want %s", batch[index].pending.Key(), index, key)
		}
	}
	if queue.Len() != 0 {
		t.Fatalf("length after batch = %d, want 0", queue.Len())
	}
}

func TestPendingQueueRemoveAnywhere(t *testing.T) {
	queue := newPendingQueue()
	queue.Enqueue(queuedPending{pending: newFakePending("first")})
	queue.Enqueue(queuedPending{pending: newFakePending("second")})
	queue.Enqueue(queuedPending{pending: newFakePending("third")})

	if !queue.Remove("second") {
		t.Fatal("removing the middle entry returned false")
	}
	if queue.Len() != 2 {
		t.Fatalf("length after middle removal = %d, want 2", queue.Len())
	}
	batch := queue.TakeBatch(2)
	if batch[0].pending.Key() != "first" || batch[1].pending.Key() != "third" {
		t.Fatalf("order after middle removal = %s, %s", batch[0].pending.Key(), batch[1].pending.Key())
	}

	queue.Enqueue(queuedPending{pending: newFakePending("fourth")})
	if !queue.Remove("fourth") {
		t.Fatal("removing the last entry returned false")
	}
	if queue.Remove("missing") {
		t.Fatal("removing a missing entry returned true")
	}
	if queue.Len() != 0 {
		t.Fatalf("length = %d, want 0", queue.Len())
	}
}

func TestPendingQueueTakeBatchNeedsCapacity(t *testing.T) {
	queue := newPendingQueue()
	queue.Enqueue(queuedPending{pending: newFakePending("first")})

	if batch := queue.TakeBatch(2); batch != nil {
		t.Fatalf("batch = %v, want nil below capacity", batch)
	}
	if queue.Len() != 1 {
		t.Fatalf("length = %d, want 1", queue.Len())
	}
	if batch := queue.TakeBatch(0); batch != nil {
		t.Fatalf("batch = %v, want nil for non-positive capacity", batch)
	}
}

func TestPendingQueuePrunesClosedEntries(t *testing.T) {
	queue := newPendingQueue()
	first := newFakePending("first")
	closed := newFakePending("closed")
	third := newFakePending("third")
	queue.Enqueue(queuedPending{pending: first})
	queue.Enqueue(queuedPending{pending: closed})
	queue.Enqueue(queuedPending{pending: third})

	closed.Close()

	batch := queue.TakeBatch(2)
	if len(batch) != 2 {
		t.Fatalf("batch size = %d, want 2 after pruning", len(batch))
	}
	if batch[0].pending.Key() != "first" || batch[1].pending.Key() != "third" {
		t.Fatalf("batch = %s, %s, want first, third", batch[0].pending.Key(), batch[1].pending.Key())
	}
}

func TestPendingQueueRequeueOrder(t *testing.T) {
	queue := newPendingQueue()
	queue.Enqueue(queuedPending{pending: newFakePending("second")})
	queue.Enqueue(queuedPending{pending: newFakePending("third")})
	queue.Requeue([]queuedPending{{pending: newFakePending("first")}})
	queue.Enqueue(queuedPending{pending: newFakePending("fourth")})

	batch := queue.TakeBatch(4)
	if len(batch) != 4 {
		t.Fatalf("batch size = %d, want 4", len(batch))
	}
	for index, key := range []string{"first", "second", "third", "fourth"} {
		if batch[index].pending.Key() != key {
			t.Fatalf("batch = %s at %d, want %s", batch[index].pending.Key(), index, key)
		}
	}
}

func TestPendingQueueSignalsWaiters(t *testing.T) {
	queue := newPendingQueue()

	queue.Enqueue(queuedPending{pending: newFakePending("first")})
	select {
	case <-queue.Wait():
	case <-time.After(time.Second):
		t.Fatal("enqueue did not signal waiters")
	}

	queue.Enqueue(queuedPending{pending: newFakePending("second")})
	select {
	case <-queue.Wait():
	case <-time.After(time.Second):
		t.Fatal("second enqueue did not signal waiters")
	}

	queue.Remove("first")
	select {
	case <-queue.Wait():
	case <-time.After(time.Second):
		t.Fatal("remove did not signal waiters")
	}

	queue.TakeBatch(1)
	select {
	case <-queue.Wait():
	case <-time.After(time.Second):
		t.Fatal("batch take did not signal waiters")
	}
}

func TestPendingQueueRequeueWaitsForNewArrival(t *testing.T) {
	queue := newPendingQueue()
	queue.Enqueue(queuedPending{pending: newFakePending("first")})
	queue.Enqueue(queuedPending{pending: newFakePending("second")})

	batch := queue.TakeBatch(2)
	if len(batch) != 2 {
		t.Fatalf("batch size = %d, want 2", len(batch))
	}
	queue.Requeue(batch)

	if again := queue.TakeBatch(2); again != nil {
		t.Fatalf("batch = %v, want nil while paused", again)
	}
	if queue.Len() != 2 {
		t.Fatalf("length = %d, want 2", queue.Len())
	}

	queue.Enqueue(queuedPending{pending: newFakePending("third")})
	retried := queue.TakeBatch(2)
	if len(retried) != 2 {
		t.Fatalf("batch size = %d, want 2 after a new arrival", len(retried))
	}
	if retried[0].pending.Key() != "first" || retried[1].pending.Key() != "second" {
		t.Fatalf("retried batch = %s, %s", retried[0].pending.Key(), retried[1].pending.Key())
	}
}
