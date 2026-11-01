package snapshot

import (
	"context"
	"testing"
)

func TestOutboxDrainsInNumericOrder(t *testing.T) {
	outbox := NewOutbox(nil)
	outbox.Publish(BroadcastType, []byte("broadcast"))
	outbox.Publish(CustomType, []byte("custom"))
	outbox.Publish(SystemType, []byte("system"))
	outbox.Publish(PrivateType, []byte("private"))

	want := []string{"system", "private", "broadcast", "custom"}
	for _, value := range want {
		got := outbox.Next()
		if string(got) != value {
			t.Fatalf("Next() = %q, want %q", got, value)
		}
	}
	if got := outbox.Next(); got != nil {
		t.Fatalf("Next() after drain = %q, want nil", got)
	}
}

func TestOutboxKeepsLatestPerType(t *testing.T) {
	outbox := NewOutbox(nil)
	outbox.Publish(PrivateType, []byte("first"))
	outbox.Publish(PrivateType, []byte("second"))

	if got := string(outbox.Next()); got != "second" {
		t.Fatalf("Next() = %q, want the latest private payload", got)
	}
	if got := outbox.Next(); got != nil {
		t.Fatalf("Next() = %q, want nil after the single coalesced slot", got)
	}
}

func TestOutboxDropsUnknownType(t *testing.T) {
	outbox := NewOutbox(nil)

	if outbox.Publish(TypeCount, []byte("invalid")) {
		t.Fatal("Publish above TypeCount returned true")
	}
	if got := outbox.Next(); got != nil {
		t.Fatalf("Next() = %q, want nil after a dropped publish", got)
	}
}

func TestOutboxDropsPublishAfterClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	outbox := NewOutbox(ctx.Done())

	if !outbox.Publish(SystemType, []byte("before")) {
		t.Fatal("Publish before close returned false")
	}
	cancel()
	if outbox.Publish(SystemType, []byte("after")) {
		t.Fatal("Publish after close returned true")
	}
	if got := string(outbox.Next()); got != "before" {
		t.Fatalf("Next() = %q, want the payload published before close", got)
	}
}

func TestOutboxWakeIsCoalesced(t *testing.T) {
	outbox := NewOutbox(nil)
	outbox.Publish(SystemType, []byte("one"))
	outbox.Publish(SystemType, []byte("two"))

	select {
	case <-outbox.Signal:
	default:
		t.Fatal("publish did not wake the outbox")
	}
	select {
	case <-outbox.Signal:
		t.Fatal("wake signal was queued more than once")
	default:
	}
}
