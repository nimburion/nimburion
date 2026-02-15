package sse

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// AuthorizeSubscribeFunc validates subscribe request using app auth context.
type AuthorizeSubscribeFunc func(c router.Context, req SubscriptionRequest) error

// HandlerConfig configures router-agnostic SSE HTTP handler.
type HandlerConfig struct {
	Manager *Manager
	// Query key for channel name, default "channel".
	ChannelQueryParam string
	// Query key for optional tenant routing, default "tenant".
	TenantQueryParam string
	// Query key for optional subject routing, default "subject".
	SubjectQueryParam string
	// Query key for explicit Last-Event-ID fallback, default "last_event_id".
	LastEventIDQueryParam string

	AuthorizeSubscribe AuthorizeSubscribeFunc
}

// Handler exposes SSE endpoint adapter for framework routers.
type Handler struct {
	cfg HandlerConfig
}

// NewHandler creates an SSE HTTP handler.
func NewHandler(cfg HandlerConfig) (*Handler, error) {
	if cfg.Manager == nil {
		return nil, errors.New("sse manager is required")
	}
	if strings.TrimSpace(cfg.ChannelQueryParam) == "" {
		cfg.ChannelQueryParam = "channel"
	}
	if strings.TrimSpace(cfg.TenantQueryParam) == "" {
		cfg.TenantQueryParam = "tenant"
	}
	if strings.TrimSpace(cfg.SubjectQueryParam) == "" {
		cfg.SubjectQueryParam = "subject"
	}
	if strings.TrimSpace(cfg.LastEventIDQueryParam) == "" {
		cfg.LastEventIDQueryParam = "last_event_id"
	}
	return &Handler{cfg: cfg}, nil
}

// Stream returns a router-agnostic SSE endpoint.
func (h *Handler) Stream() router.HandlerFunc {
	return func(c router.Context) error {
		req := parseSubscriptionRequest(c, h.cfg)
		if req.Channel == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing channel"})
		}

		if h.cfg.AuthorizeSubscribe != nil {
			if err := h.cfg.AuthorizeSubscribe(c, req); err != nil {
				return c.JSON(http.StatusForbidden, map[string]string{"error": err.Error()})
			}
		}

		flusher, ok := c.Response().(http.Flusher)
		if !ok {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "response writer does not support streaming"})
		}

		client, replay, err := h.cfg.Manager.Subscribe(c.Request().Context(), req)
		if err != nil {
			if errors.Is(err, ErrTooManyConnections) {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": err.Error()})
			}
			if errors.Is(err, ErrInvalidChannel) {
				return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "subscribe failed"})
		}
		defer func() { _ = h.cfg.Manager.Disconnect(client.ID()) }()

		header := c.Response().Header()
		header.Set("Content-Type", "text/event-stream")
		header.Set("Cache-Control", "no-cache, no-transform")
		header.Set("Connection", "keep-alive")
		header.Set("X-Accel-Buffering", "no")

		c.Response().WriteHeader(http.StatusOK)
		flusher.Flush()

		for _, evt := range replay {
			if err := writeEvent(c.Response(), evt); err != nil {
				return nil
			}
			flusher.Flush()
		}

		if err := writeComment(c.Response(), "connected"); err == nil {
			flusher.Flush()
		}

		ticker := time.NewTicker(h.cfg.Manager.cfg.HeartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-c.Request().Context().Done():
				return nil
			case <-client.Closed():
				return nil
			case <-ticker.C:
				if err := writeComment(c.Response(), "heartbeat"); err != nil {
					return nil
				}
				flusher.Flush()
			case evt, ok := <-client.Events():
				if !ok {
					return nil
				}
				if err := writeEvent(c.Response(), evt); err != nil {
					return nil
				}
				flusher.Flush()
			}
		}
	}
}

func parseSubscriptionRequest(c router.Context, cfg HandlerConfig) SubscriptionRequest {
	r := c.Request()
	lastID := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if lastID == "" {
		lastID = strings.TrimSpace(c.Query(cfg.LastEventIDQueryParam))
	}
	return SubscriptionRequest{
		ClientID:    strings.TrimSpace(r.Header.Get("X-Client-ID")),
		Channel:     strings.TrimSpace(c.Query(cfg.ChannelQueryParam)),
		TenantID:    strings.TrimSpace(c.Query(cfg.TenantQueryParam)),
		Subject:     strings.TrimSpace(c.Query(cfg.SubjectQueryParam)),
		LastEventID: lastID,
	}
}

func writeComment(w http.ResponseWriter, value string) error {
	_, err := w.Write([]byte(": " + value + "\n\n"))
	return err
}

func writeEvent(w http.ResponseWriter, event Event) error {
	var buffer bytes.Buffer
	if event.ID != "" {
		buffer.WriteString("id: ")
		buffer.WriteString(event.ID)
		buffer.WriteByte('\n')
	}
	if event.Type != "" {
		buffer.WriteString("event: ")
		buffer.WriteString(event.Type)
		buffer.WriteByte('\n')
	}
	if event.RetryMS > 0 {
		buffer.WriteString("retry: ")
		buffer.WriteString(strconv.Itoa(event.RetryMS))
		buffer.WriteByte('\n')
	}
	data := event.Data
	if len(data) == 0 {
		data = []byte("{}")
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		buffer.WriteString("data: ")
		buffer.WriteString(line)
		buffer.WriteByte('\n')
	}
	buffer.WriteByte('\n')

	_, err := fmt.Fprint(w, buffer.String())
	return err
}
