package pubsub

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestProperty_InMemoryBusPreservesOrderPerSubscriber(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 80
	properties := gopter.NewProperties(params)

	properties.Property("single subscriber receives messages in publish order", prop.ForAll(
		func(values []uint8) bool {
			if len(values) == 0 {
				return true
			}
			bus := NewInMemoryBus(InMemoryConfig{SubscriberBuffer: len(values) + 1})
			defer bus.Close()

			sub, err := bus.Subscribe("topic")
			if err != nil {
				t.Logf("subscribe error: %v", err)
				return false
			}

			for _, value := range values {
				if err := bus.Publish(context.Background(), Message{Topic: "topic", Payload: []byte{value}}); err != nil {
					t.Logf("publish error: %v", err)
					return false
				}
			}

			received := make([]uint8, 0, len(values))
			for i := 0; i < len(values); i++ {
				select {
				case msg := <-sub.Receive():
					received = append(received, msg.Payload[0])
				case <-time.After(time.Second):
					t.Log("timed out while waiting for message")
					return false
				}
			}

			if !reflect.DeepEqual(received, values) {
				t.Logf("received=%v want=%v", received, values)
				return false
			}
			return true
		},
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}

func TestProperty_InMemoryBusSubscriberIndependence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 50
	properties := gopter.NewProperties(params)

	properties.Property("slow subscriber drops do not block fast subscriber", prop.ForAll(
		func(count uint8) bool {
			messageCount := int(count%20) + 10
			bus := NewInMemoryBus(InMemoryConfig{SubscriberBuffer: 1})
			defer bus.Close()

			slow, err := bus.Subscribe("topic")
			if err != nil {
				t.Logf("subscribe slow error: %v", err)
				return false
			}
			fast, err := bus.Subscribe("topic")
			if err != nil {
				t.Logf("subscribe fast error: %v", err)
				return false
			}

			start := time.Now()
			for i := 0; i < messageCount; i++ {
				if err := bus.Publish(context.Background(), Message{Topic: "topic", Payload: []byte(fmt.Sprintf("%d", i))}); err != nil {
					t.Logf("publish error: %v", err)
					return false
				}
			}
			if time.Since(start) > 200*time.Millisecond {
				t.Logf("publish loop was unexpectedly slow: %v", time.Since(start))
				return false
			}

			receivedByFast := 0
			drainDeadline := time.After(200 * time.Millisecond)
			for {
				select {
				case <-fast.Receive():
					receivedByFast++
				case <-drainDeadline:
					if receivedByFast == 0 {
						t.Log("expected fast subscriber to receive at least one message")
						return false
					}
					select {
					case <-slow.Receive():
					default:
					}
					return true
				}
			}
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}

func TestProperty_InMemoryBusCloseDoesNotLeakGoroutines(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	properties := gopter.NewProperties(params)

	properties.Property("close releases subscribers without goroutine growth", prop.ForAll(
		func(n uint8) bool {
			subCount := int(n%30) + 1
			before := runtime.NumGoroutine()
			bus := NewInMemoryBus(InMemoryConfig{SubscriberBuffer: 2})
			for i := 0; i < subCount; i++ {
				sub, err := bus.Subscribe(Topic(fmt.Sprintf("topic-%d", i%3)))
				if err != nil {
					t.Logf("subscribe error: %v", err)
					return false
				}
				_ = sub
			}

			if err := bus.Close(); err != nil {
				t.Logf("close error: %v", err)
				return false
			}
			time.Sleep(10 * time.Millisecond)
			after := runtime.NumGoroutine()
			return after <= before+3
		},
		gen.UInt8(),
	))

	properties.TestingRun(t)
}
