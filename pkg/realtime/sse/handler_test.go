package sse

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
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
func (c *fakeContext) Param(name string) string            { return "" }
func (c *fakeContext) Query(name string) string            { return c.req.URL.Query().Get(name) }
func (c *fakeContext) Bind(v interface{}) error            { return nil }
func (c *fakeContext) JSON(code int, v interface{}) error  { c.resp.WriteHeader(code); return nil }
func (c *fakeContext) String(code int, s string) error {
	c.resp.WriteHeader(code)
	_, err := c.resp.Write([]byte(s))
	return err
}
func (c *fakeContext) Get(key string) interface{}        { return nil }
func (c *fakeContext) Set(key string, value interface{}) {}

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
