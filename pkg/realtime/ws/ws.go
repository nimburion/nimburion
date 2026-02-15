package ws

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/middleware/cors"
)

const (
	OpText  byte = 0x1
	OpClose byte = 0x8
	OpPing  byte = 0x9
	OpPong  byte = 0xA

	websocketMagicGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
)

type Config struct {
	// Legacy origin allow-list. When OriginPolicy is not set:
	// - empty list means allow all origins (legacy behavior)
	// - non-empty list means exact-match allow-list.
	AllowedOrigins []string
	// OriginPolicy enables advanced origin validation shared with HTTP CORS middleware.
	OriginPolicy     cors.Config
	TopicsQueryParam string
	ReadLimit        int
	WriteTimeout     time.Duration
	MaxTopicCount    int
	MaxTopicLength   int
	// KeepaliveInterval enables automatic keepalive messages. Zero disables keepalive.
	KeepaliveInterval time.Duration
	// RequireAuth requires JWT authentication before upgrade. When true, claims are available in OnConnect.
	RequireAuth bool
}

func DefaultConfig() Config {
	return Config{
		AllowedOrigins:    []string{},
		TopicsQueryParam:  "topics",
		ReadLimit:         4096,
		WriteTimeout:      10 * time.Second,
		MaxTopicCount:     20,
		MaxTopicLength:    64,
		KeepaliveInterval: 0,
		RequireAuth:       false,
	}
}

type Conn struct {
	conn         net.Conn
	rw           *bufio.ReadWriter
	readLimit    int
	writeTimeout time.Duration
	writeMu      sync.Mutex
}

func Upgrade(w http.ResponseWriter, r *http.Request, cfg Config) (*Conn, []string, error) {
	cfg = normalizeConfig(cfg)

	topics, err := parseTopicsFromRequest(r, cfg)
	if err != nil {
		return nil, nil, err
	}
	if err := validateWebSocketHeaders(r, cfg); err != nil {
		return nil, nil, err
	}

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response does not support hijacking")
	}

	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}

	accept := computeWebSocketAccept(strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key")))
	response := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + accept + "\r\n\r\n"
	if _, err := rw.WriteString(response); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}

	return &Conn{
		conn:         conn,
		rw:           rw,
		readLimit:    cfg.ReadLimit,
		writeTimeout: cfg.WriteTimeout,
	}, topics, nil
}

func (c *Conn) Close() error {
	return c.conn.Close()
}

func (c *Conn) WriteJSON(payload interface{}) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.WriteFrame(OpText, raw)
}

func (c *Conn) WriteFrame(opcode byte, payload []byte) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	_ = c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))

	header := make([]byte, 0, 14)
	header = append(header, 0x80|opcode)

	payloadLen := len(payload)
	switch {
	case payloadLen < 126:
		header = append(header, byte(payloadLen))
	case payloadLen <= 65535:
		header = append(header, 126)
		var ext [2]byte
		binary.BigEndian.PutUint16(ext[:], uint16(payloadLen))
		header = append(header, ext[:]...)
	default:
		header = append(header, 127)
		var ext [8]byte
		binary.BigEndian.PutUint64(ext[:], uint64(payloadLen))
		header = append(header, ext[:]...)
	}

	if _, err := c.rw.Write(header); err != nil {
		return err
	}
	if payloadLen > 0 {
		if _, err := c.rw.Write(payload); err != nil {
			return err
		}
	}
	return c.rw.Flush()
}

func (c *Conn) ReadFrame() (byte, []byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(c.rw, header[:]); err != nil {
		return 0, nil, err
	}

	opcode := header[0] & 0x0F
	masked := (header[1] & 0x80) != 0
	payloadLen := int(header[1] & 0x7F)

	switch payloadLen {
	case 126:
		var ext [2]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return 0, nil, err
		}
		payloadLen = int(binary.BigEndian.Uint16(ext[:]))
	case 127:
		var ext [8]byte
		if _, err := io.ReadFull(c.rw, ext[:]); err != nil {
			return 0, nil, err
		}
		size := binary.BigEndian.Uint64(ext[:])
		if size > uint64(c.readLimit) {
			return 0, nil, fmt.Errorf("websocket frame too large")
		}
		payloadLen = int(size)
	}

	if payloadLen > c.readLimit {
		return 0, nil, fmt.Errorf("websocket frame too large")
	}

	var mask [4]byte
	if masked {
		if _, err := io.ReadFull(c.rw, mask[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(c.rw, payload); err != nil {
			return 0, nil, err
		}
	}

	if masked {
		for idx := 0; idx < payloadLen; idx++ {
			payload[idx] ^= mask[idx%4]
		}
	}

	return opcode, payload, nil
}

func ParseTopics(raw string, maxCount, maxLen int) ([]string, error) {
	values := parseList(raw)
	if len(values) == 0 {
		return []string{}, nil
	}
	if len(values) > maxCount {
		return nil, fmt.Errorf("too many topics requested (max %d)", maxCount)
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if len(value) > maxLen {
			return nil, fmt.Errorf("topic %q exceeds max length %d", value, maxLen)
		}
		if !isValidTopic(value) {
			return nil, fmt.Errorf("invalid topic %q", value)
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func parseTopicsFromRequest(r *http.Request, cfg Config) ([]string, error) {
	return ParseTopics(r.URL.Query().Get(cfg.TopicsQueryParam), cfg.MaxTopicCount, cfg.MaxTopicLength)
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if strings.TrimSpace(cfg.TopicsQueryParam) == "" {
		cfg.TopicsQueryParam = defaults.TopicsQueryParam
	}
	if cfg.ReadLimit <= 0 {
		cfg.ReadLimit = defaults.ReadLimit
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaults.WriteTimeout
	}
	if cfg.MaxTopicCount <= 0 {
		cfg.MaxTopicCount = defaults.MaxTopicCount
	}
	if cfg.MaxTopicLength <= 0 {
		cfg.MaxTopicLength = defaults.MaxTopicLength
	}
	return cfg
}

func validateWebSocketHeaders(r *http.Request, cfg Config) error {
	if !headerHasToken(r.Header, "Connection", "upgrade") || !headerHasToken(r.Header, "Upgrade", "websocket") {
		return fmt.Errorf("websocket upgrade headers missing")
	}
	if strings.TrimSpace(r.Header.Get("Sec-WebSocket-Version")) != "13" {
		return fmt.Errorf("unsupported websocket version")
	}
	if strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key")) == "" {
		return fmt.Errorf("missing websocket key")
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin != "" && !isAllowedOrigin(origin, cfg) {
		return fmt.Errorf("websocket origin %q is not allowed", origin)
	}
	return nil
}

func isAllowedOrigin(origin string, cfg Config) bool {
	policy := wsOriginPolicy(cfg)
	return policy.AllowsOrigin(origin)
}

func wsOriginPolicy(cfg Config) cors.Config {
	if hasOriginPolicyConfig(cfg.OriginPolicy) {
		return cfg.OriginPolicy
	}

	// Legacy fallback: empty AllowedOrigins means allow all.
	if len(cfg.AllowedOrigins) == 0 {
		return cors.Config{
			AllowAllOrigins: true,
		}
	}
	return cors.Config{
		AllowOrigins: cfg.AllowedOrigins,
	}
}

func hasOriginPolicyConfig(cfg cors.Config) bool {
	if cfg.AllowAllOrigins ||
		len(cfg.AllowOrigins) > 0 ||
		cfg.AllowOriginFunc != nil ||
		cfg.AllowOriginWithContextFunc != nil ||
		cfg.AllowWildcard ||
		cfg.AllowBrowserExtensions ||
		len(cfg.CustomSchemas) > 0 ||
		cfg.AllowWebSockets ||
		cfg.AllowFiles {
		return true
	}
	return false
}

func headerHasToken(headers http.Header, key, expected string) bool {
	for _, value := range headers.Values(key) {
		for _, token := range strings.Split(value, ",") {
			if strings.EqualFold(strings.TrimSpace(token), expected) {
				return true
			}
		}
	}
	return false
}

func computeWebSocketAccept(secKey string) string {
	sum := sha1.Sum([]byte(secKey + websocketMagicGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func parseList(raw string) []string {
	separators := strings.NewReplacer(",", " ", ";", " ", "\n", " ", "\t", " ")
	normalized := separators.Replace(raw)
	values := strings.Fields(normalized)
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func isValidTopic(topic string) bool {
	for _, ch := range topic {
		isLetter := (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
		isDigit := ch >= '0' && ch <= '9'
		isAllowedSymbol := ch == '-' || ch == '_' || ch == '.'
		if !isLetter && !isDigit && !isAllowedSymbol {
			return false
		}
	}
	return true
}
