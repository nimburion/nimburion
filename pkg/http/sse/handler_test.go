package sse

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/http/router"
)

func TestHandler_StreamWritesEventAndHeartbeat(t *testing.T) {
	manager := NewManager(ManagerConfig{
		MaxConnections:    100,
		ClientBuffer:      8,
		ReplayLimit:       16,
		HeartbeatInterval: 20 * time.Millisecond,
		DefaultRetryMS:    1500,
	}, NewInMemoryStore(32), nil)
	defer manager.Close()

	handler, err := NewHandler(HandlerConfig{Manager: manager})
	if err != nil {
		t.Fatalf("new handler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/events?channel=orders", nil).WithContext(ctx)
	resp := newFakeSSEWriter()
	c := newFakeContext(req, resp)

	done := make(chan error, 1)
	go func() {
		done <- handler.Stream()(c)
	}()

	time.Sleep(30 * time.Millisecond)
	if _, err := manager.Publish(context.Background(), PublishRequest{
		Channel: "orders",
		Type:    "order.created",
		Data:    []byte(`{"id":"o1"}`),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		body := resp.BodyString()
		if strings.Contains(body, "event: order.created") && strings.Contains(body, `data: {"id":"o1"}`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting streamed event, body=%q", body)
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handler returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("handler did not stop after cancel")
	}
}

func TestHandler_StreamDoesNotLeakUnderlyingErrors(t *testing.T) {
	manager := NewManager(ManagerConfig{
		MaxConnections:    1,
		ClientBuffer:      1,
		ReplayLimit:       1,
		HeartbeatInterval: time.Second,
		DefaultRetryMS:    1000,
	}, NewInMemoryStore(1), nil)
	defer manager.Close()

	t.Run("authorize callback error is hidden", func(t *testing.T) {
		h, err := NewHandler(HandlerConfig{
			Manager: manager,
			AuthorizeSubscribe: func(_ router.Context, _ SubscriptionRequest) error {
				return errors.New("sensitive authorization callback error")
			},
		})
		if err != nil {
			t.Fatalf("new handler: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/events?channel=orders", nil)
		resp := newFakeSSEWriter()
		c := newFakeContext(req, resp)
		if err := h.Stream()(c); err != nil {
			t.Fatalf("stream failed: %v", err)
		}
		if resp.Status() != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.Status())
		}
		body := resp.BodyString()
		if strings.Contains(body, "sensitive authorization callback error") {
			t.Fatalf("response leaked internal error: %s", body)
		}
		if !strings.Contains(body, "subscription not authorized") {
			t.Fatalf("expected fixed message, got: %s", body)
		}
	})

	t.Run("subscription typed errors are canonicalized", func(t *testing.T) {
		h, err := NewHandler(HandlerConfig{Manager: manager})
		if err != nil {
			t.Fatalf("new handler: %v", err)
		}

		streamCtx, cancel := context.WithCancel(context.Background())
		defer cancel()
		firstReq := httptest.NewRequest(http.MethodGet, "/events?channel=orders", nil).WithContext(streamCtx)
		firstResp := newFakeSSEWriter()
		firstCtx := newFakeContext(firstReq, firstResp)
		done := make(chan struct{})
		go func() {
			_ = h.Stream()(firstCtx)
			close(done)
		}()
		time.Sleep(20 * time.Millisecond)

		secondReq := httptest.NewRequest(http.MethodGet, "/events?channel=orders", nil)
		secondResp := newFakeSSEWriter()
		secondCtx := newFakeContext(secondReq, secondResp)
		if err := h.Stream()(secondCtx); err != nil {
			t.Fatalf("stream failed: %v", err)
		}
		body := secondResp.BodyString()
		if !strings.Contains(body, "service.unavailable") {
			t.Fatalf("expected canonicalized app error, got: %s", body)
		}

		cancel()
		<-done
	})
}

type fakeContext struct {
	req  *http.Request
	resp router.ResponseWriter
}

func newFakeContext(req *http.Request, resp router.ResponseWriter) *fakeContext {
	return &fakeContext{req: req, resp: resp}
}

func (c *fakeContext) Request() *http.Request              { return c.req }
func (c *fakeContext) SetRequest(r *http.Request)          { c.req = r }
func (c *fakeContext) Response() router.ResponseWriter     { return c.resp }
func (c *fakeContext) SetResponse(w router.ResponseWriter) { c.resp = w }
func (c *fakeContext) Param(_ string) string               { return "" }
func (c *fakeContext) Query(key string) string             { return c.req.URL.Query().Get(key) }
func (c *fakeContext) Bind(_ interface{}) error            { return nil }
func (c *fakeContext) JSON(code int, v interface{}) error {
	c.resp.WriteHeader(code)
	return json.NewEncoder(c.resp).Encode(v)
}
func (c *fakeContext) String(code int, s string) error {
	c.resp.WriteHeader(code)
	_, err := c.resp.Write([]byte(s))
	return err
}

func (c *fakeContext) Get(_ string) interface{}    { return nil }
func (c *fakeContext) Set(_ string, _ interface{}) {}

type fakeSSEWriter struct {
	header  http.Header
	status  int
	written bool
	mu      sync.Mutex
	body    strings.Builder
}

func newFakeSSEWriter() *fakeSSEWriter {
	return &fakeSSEWriter{
		header: make(http.Header),
	}
}

func (w *fakeSSEWriter) Header() http.Header {
	return w.header
}

func (w *fakeSSEWriter) WriteHeader(code int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.written {
		return
	}
	w.written = true
	w.status = code
}

func (w *fakeSSEWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.written {
		w.written = true
		w.status = http.StatusOK
	}
	return w.body.Write(b)
}

func (w *fakeSSEWriter) Status() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func (w *fakeSSEWriter) Written() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.written
}

func (w *fakeSSEWriter) Flush() {}

func (w *fakeSSEWriter) BodyString() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.body.String()
}
