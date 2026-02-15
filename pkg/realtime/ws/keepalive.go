package ws

import (
	"context"
	"time"
)

// StartKeepalive starts a goroutine that sends periodic keepalive messages.
// Returns a cancel function to stop the keepalive loop.
func (c *Conn) StartKeepalive(ctx context.Context, interval time.Duration, messageFunc func() interface{}) context.CancelFunc {
	if interval <= 0 {
		return func() {}
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var msg interface{}
				if messageFunc != nil {
					msg = messageFunc()
				} else {
					msg = map[string]string{
						"type":      "keepalive",
						"timestamp": time.Now().UTC().Format(time.RFC3339),
					}
				}
				if err := c.WriteJSON(msg); err != nil {
					return
				}
			}
		}
	}()

	return cancel
}

// HandlePingPong starts a goroutine that automatically responds to ping frames.
// Returns a channel that closes when the connection is closed or an error occurs.
func (c *Conn) HandlePingPong(ctx context.Context) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				opcode, payload, err := c.ReadFrame()
				if err != nil {
					return
				}
				switch opcode {
				case OpClose:
					_ = c.WriteFrame(OpClose, nil)
					return
				case OpPing:
					if len(payload) > c.readLimit {
						return
					}
					_ = c.WriteFrame(OpPong, payload)
				}
			}
		}
	}()

	return done
}
