package sse

import (
	"context"
	"testing"
	"time"
)

func TestManager_PublishSubscribeRoutingAndReplay(t *testing.T) {
	manager := NewManager(DefaultManagerConfig(), NewInMemoryStore(32), nil)
	defer manager.Close()

	clientA, replayA, err := manager.Subscribe(context.Background(), SubscriptionRequest{
		Channel:  "orders",
		TenantID: "t1",
		Subject:  "u1",
	})
	if err != nil {
		t.Fatalf("subscribe A: %v", err)
	}
	if len(replayA) != 0 {
		t.Fatalf("expected empty replay on first subscribe")
	}
	defer manager.Disconnect(clientA.ID())

	clientB, _, err := manager.Subscribe(context.Background(), SubscriptionRequest{
		Channel:  "orders",
		TenantID: "t2",
		Subject:  "u2",
	})
	if err != nil {
		t.Fatalf("subscribe B: %v", err)
	}
	defer manager.Disconnect(clientB.ID())

	first, err := manager.Publish(context.Background(), PublishRequest{
		Channel:  "orders",
		TenantID: "t1",
		Subject:  "u1",
		Type:     "order.created",
		Data:     []byte(`{"id":"o1"}`),
	})
	if err != nil {
		t.Fatalf("publish first: %v", err)
	}

	select {
	case evt := <-clientA.Events():
		if evt.ID != first.ID {
			t.Fatalf("client A got wrong event id: %s", evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting event for client A")
	}

	select {
	case <-clientB.Events():
		t.Fatalf("client B should not receive tenant/subject-mismatched event")
	case <-time.After(50 * time.Millisecond):
	}

	clientReplay, replay, err := manager.Subscribe(context.Background(), SubscriptionRequest{
		Channel:     "orders",
		TenantID:    "t1",
		Subject:     "u1",
		LastEventID: "",
	})
	if err != nil {
		t.Fatalf("subscribe replay: %v", err)
	}
	defer manager.Disconnect(clientReplay.ID())
	if len(replay) == 0 || replay[len(replay)-1].ID != first.ID {
		t.Fatalf("expected replay to include first event")
	}
}

func TestManager_DistributedWithInMemoryBus(t *testing.T) {
	bus := NewInMemoryBus()
	storeA := NewInMemoryStore(16)
	storeB := NewInMemoryStore(16)

	managerA := NewManager(DefaultManagerConfig(), storeA, bus)
	managerB := NewManager(DefaultManagerConfig(), storeB, bus)
	defer managerA.Close()
	defer managerB.Close()

	clientB, _, err := managerB.Subscribe(context.Background(), SubscriptionRequest{
		Channel: "alerts",
	})
	if err != nil {
		t.Fatalf("subscribe B: %v", err)
	}
	defer managerB.Disconnect(clientB.ID())

	published, err := managerA.Publish(context.Background(), PublishRequest{
		Channel: "alerts",
		Type:    "notice",
		Data:    []byte(`"hello"`),
	})
	if err != nil {
		t.Fatalf("publish A: %v", err)
	}

	select {
	case evt := <-clientB.Events():
		if evt.ID != published.ID {
			t.Fatalf("unexpected distributed event id: %s", evt.ID)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting distributed event")
	}
}
