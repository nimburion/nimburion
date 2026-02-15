package sse

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	// ErrTooManyConnections indicates max local SSE connections reached.
	ErrTooManyConnections = errors.New("too many sse connections")
	// ErrInvalidChannel indicates an empty or invalid channel.
	ErrInvalidChannel = errors.New("invalid channel")
)

// ManagerConfig configures SSE connection manager.
type ManagerConfig struct {
	InstanceID         string
	MaxConnections     int
	ClientBuffer       int
	ReplayLimit        int
	DropOnBackpressure bool
	HeartbeatInterval  time.Duration
	DefaultRetryMS     int
}

// DefaultManagerConfig returns defaults tuned for API SSE usage.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		InstanceID:         "instance-1",
		MaxConnections:     10000,
		ClientBuffer:       64,
		ReplayLimit:        100,
		DropOnBackpressure: true,
		HeartbeatInterval:  20 * time.Second,
		DefaultRetryMS:     3000,
	}
}

// Manager handles local SSE clients and distributed fan-out via bus.
type Manager struct {
	cfg   ManagerConfig
	store Store
	bus   Bus

	mu             sync.RWMutex
	connections    map[string]*Client
	byChannel      map[string]map[string]*Client
	busSubscribers map[string]Subscription
}

// Client represents a connected SSE subscriber.
type Client struct {
	id        string
	channel   string
	tenantID  string
	subject   string
	lastEvent string
	events    chan Event
	closed    chan struct{}
	closeOnce sync.Once
}

// SubscriptionRequest describes channel + routing context for new connection.
type SubscriptionRequest struct {
	ClientID    string
	Channel     string
	TenantID    string
	Subject     string
	LastEventID string
}

// PublishRequest describes an outgoing SSE event.
type PublishRequest struct {
	Channel  string
	TenantID string
	Subject  string
	Type     string
	Data     []byte
	RetryMS  int
}

// NewManager creates an SSE manager with optional bus/store.
func NewManager(cfg ManagerConfig, store Store, bus Bus) *Manager {
	cfg = normalizeManagerConfig(cfg)
	if store == nil {
		store = NewInMemoryStore(cfg.ReplayLimit)
	}
	return &Manager{
		cfg:            cfg,
		store:          store,
		bus:            bus,
		connections:    make(map[string]*Client),
		byChannel:      make(map[string]map[string]*Client),
		busSubscribers: make(map[string]Subscription),
	}
}

// Subscribe registers a new local SSE client and returns replay events.
func (m *Manager) Subscribe(ctx context.Context, req SubscriptionRequest) (*Client, []Event, error) {
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		return nil, nil, ErrInvalidChannel
	}

	client := &Client{
		id:        chooseClientID(req.ClientID),
		channel:   channel,
		tenantID:  strings.TrimSpace(req.TenantID),
		subject:   strings.TrimSpace(req.Subject),
		lastEvent: strings.TrimSpace(req.LastEventID),
		events:    make(chan Event, m.cfg.ClientBuffer),
		closed:    make(chan struct{}),
	}

	m.mu.Lock()
	if m.cfg.MaxConnections > 0 && len(m.connections) >= m.cfg.MaxConnections {
		m.mu.Unlock()
		return nil, nil, ErrTooManyConnections
	}
	m.connections[client.id] = client
	if m.byChannel[channel] == nil {
		m.byChannel[channel] = make(map[string]*Client)
	}
	m.byChannel[channel][client.id] = client
	needSub := m.bus != nil && m.busSubscribers[channel] == nil
	m.mu.Unlock()

	if needSub {
		if err := m.ensureBusSubscription(ctx, channel); err != nil {
			_ = m.Disconnect(client.id)
			return nil, nil, err
		}
	}

	replay, err := m.store.GetSince(ctx, channel, client.lastEvent, m.cfg.ReplayLimit)
	if err != nil {
		_ = m.Disconnect(client.id)
		return nil, nil, err
	}
	return client, replay, nil
}

// Publish app-level event to replay store and distributed bus/local subscribers.
func (m *Manager) Publish(ctx context.Context, req PublishRequest) (Event, error) {
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		return Event{}, ErrInvalidChannel
	}
	event := Event{
		Channel:   channel,
		TenantID:  strings.TrimSpace(req.TenantID),
		Subject:   strings.TrimSpace(req.Subject),
		Type:      strings.TrimSpace(req.Type),
		Data:      append([]byte(nil), req.Data...),
		RetryMS:   req.RetryMS,
		Timestamp: time.Now().UTC(),
	}
	if event.RetryMS <= 0 {
		event.RetryMS = m.cfg.DefaultRetryMS
	}
	event.normalize(time.Now().UTC())

	if err := m.store.Append(ctx, event); err != nil {
		return Event{}, err
	}

	if m.bus != nil {
		if err := m.bus.Publish(ctx, event); err != nil {
			return Event{}, err
		}
		return event, nil
	}

	m.deliver(event)
	return event, nil
}

// Disconnect removes and closes one local connection.
func (m *Manager) Disconnect(clientID string) error {
	m.mu.Lock()
	client := m.connections[clientID]
	if client == nil {
		m.mu.Unlock()
		return nil
	}

	delete(m.connections, clientID)
	channelClients := m.byChannel[client.channel]
	delete(channelClients, clientID)
	if len(channelClients) == 0 {
		delete(m.byChannel, client.channel)
		if sub := m.busSubscribers[client.channel]; sub != nil {
			_ = sub.Close()
			delete(m.busSubscribers, client.channel)
		}
	}
	m.mu.Unlock()

	client.close()
	return nil
}

// Close closes manager, bus and store.
func (m *Manager) Close() error {
	m.mu.Lock()
	for channel, sub := range m.busSubscribers {
		_ = sub.Close()
		delete(m.busSubscribers, channel)
	}
	for id, c := range m.connections {
		delete(m.connections, id)
		c.close()
	}
	m.byChannel = make(map[string]map[string]*Client)
	m.mu.Unlock()

	if m.bus != nil {
		_ = m.bus.Close()
	}
	if m.store != nil {
		_ = m.store.Close()
	}
	return nil
}

func (m *Manager) ensureBusSubscription(ctx context.Context, channel string) error {
	m.mu.RLock()
	existing := m.busSubscribers[channel]
	m.mu.RUnlock()
	if existing != nil {
		return nil
	}

	sub, err := m.bus.Subscribe(ctx, channel, func(event Event) {
		m.deliver(event)
	})
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.busSubscribers[channel] == nil {
		m.busSubscribers[channel] = sub
		return nil
	}
	_ = sub.Close()
	return nil
}

func (m *Manager) deliver(event Event) {
	m.mu.RLock()
	clients := m.byChannel[event.Channel]
	snapshot := make([]*Client, 0, len(clients))
	for _, c := range clients {
		snapshot = append(snapshot, c)
	}
	m.mu.RUnlock()

	for _, c := range snapshot {
		if !matchesRouting(c, event) {
			continue
		}
		if !m.enqueue(c, event) {
			if m.cfg.DropOnBackpressure {
				_ = m.Disconnect(c.id)
			}
		}
	}
}

func (m *Manager) enqueue(c *Client, event Event) bool {
	select {
	case <-c.closed:
		return false
	default:
	}

	select {
	case c.events <- event:
		return true
	default:
		return false
	}
}

func matchesRouting(c *Client, event Event) bool {
	if c.tenantID != "" && event.TenantID != "" && c.tenantID != event.TenantID {
		return false
	}
	if c.subject != "" && event.Subject != "" && c.subject != event.Subject {
		return false
	}
	return true
}

func normalizeManagerConfig(cfg ManagerConfig) ManagerConfig {
	def := DefaultManagerConfig()
	if strings.TrimSpace(cfg.InstanceID) == "" {
		cfg.InstanceID = def.InstanceID
	}
	if cfg.MaxConnections <= 0 {
		cfg.MaxConnections = def.MaxConnections
	}
	if cfg.ClientBuffer <= 0 {
		cfg.ClientBuffer = def.ClientBuffer
	}
	if cfg.ReplayLimit <= 0 {
		cfg.ReplayLimit = def.ReplayLimit
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = def.HeartbeatInterval
	}
	if cfg.DefaultRetryMS <= 0 {
		cfg.DefaultRetryMS = def.DefaultRetryMS
	}
	return cfg
}

func chooseClientID(value string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return nextEventID(time.Now().UTC())
}

// ID returns subscriber id.
func (c *Client) ID() string { return c.id }

// Channel returns subscribed channel.
func (c *Client) Channel() string { return c.channel }

// Events returns receive-only event stream for this client.
func (c *Client) Events() <-chan Event { return c.events }

// Closed returns a channel closed when client is disconnected.
func (c *Client) Closed() <-chan struct{} { return c.closed }

func (c *Client) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		close(c.events)
	})
}
