package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
)

// Config carries the parameters needed to construct a Client. All
// fields except BrokerURL and ClientID are optional.
type Config struct {
	BrokerURL     string
	ClientID      string
	Username      string
	Password      string
	TLSSkipVerify bool
}

// Handler is the per-message callback registered with Subscribe.
type Handler func(topic string, payload []byte)

// Client is the subset of paho.Client the Z2M driver uses. The thin
// wrapper exists so tests can hold a concrete type and the driver
// doesn't grow a transitive paho dependency through every layer.
type Client struct {
	cfg Config

	mu           sync.Mutex
	c            paho.Client
	handlers     map[string]Handler
	onConnect    func()
	onDisconnect func(error)
}

// New constructs a Client. BrokerURL and ClientID are required.
func New(cfg Config) (*Client, error) {
	if cfg.BrokerURL == "" {
		return nil, errors.New("mqtt: BrokerURL required")
	}
	if cfg.ClientID == "" {
		return nil, errors.New("mqtt: ClientID required")
	}
	return &Client{
		cfg:      cfg,
		handlers: make(map[string]Handler),
	}, nil
}

// OnConnect registers a callback that fires on every successful
// (re)connect. Use this to re-assert subscriptions after broker churn.
func (c *Client) OnConnect(cb func()) {
	c.mu.Lock()
	c.onConnect = cb
	c.mu.Unlock()
}

// OnDisconnect registers a callback that fires when the connection
// drops. paho's auto-reconnect runs in the background; this is purely
// informational.
func (c *Client) OnDisconnect(cb func(error)) {
	c.mu.Lock()
	c.onDisconnect = cb
	c.mu.Unlock()
}

// Connect dials the broker and blocks until the first connect
// succeeds or ctx is cancelled.
func (c *Client) Connect(ctx context.Context) error {
	opts := paho.NewClientOptions().
		AddBroker(c.cfg.BrokerURL).
		SetClientID(c.cfg.ClientID).
		SetAutoReconnect(true).
		SetCleanSession(false).
		SetConnectRetry(true).
		SetConnectRetryInterval(2 * time.Second).
		SetMaxReconnectInterval(30 * time.Second).
		SetKeepAlive(60 * time.Second).
		SetPingTimeout(10 * time.Second)

	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
	}
	if c.cfg.Password != "" {
		opts.SetPassword(c.cfg.Password)
	}
	if c.cfg.TLSSkipVerify {
		opts.SetTLSConfig(&tls.Config{InsecureSkipVerify: true}) // #nosec G402 — opt-in via config
	}

	opts.SetOnConnectHandler(func(_ paho.Client) {
		c.mu.Lock()
		cb := c.onConnect
		// Re-assert subscriptions on every connect so reconnects
		// don't silently lose them.
		for topic, h := range c.handlers {
			topic, h := topic, h
			c.c.Subscribe(topic, 0, func(_ paho.Client, msg paho.Message) {
				h(msg.Topic(), msg.Payload())
			})
		}
		c.mu.Unlock()
		if cb != nil {
			cb()
		}
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		c.mu.Lock()
		cb := c.onDisconnect
		c.mu.Unlock()
		if cb != nil {
			cb(err)
		}
	})

	c.mu.Lock()
	c.c = paho.NewClient(opts)
	c.mu.Unlock()

	tok := c.c.Connect()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-tokenDone(tok):
	}
	if err := tok.Error(); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}
	return nil
}

// Subscribe registers h for topic. Idempotent across reconnects: the
// OnConnect handler re-applies every entry in c.handlers.
func (c *Client) Subscribe(topic string, h Handler) error {
	c.mu.Lock()
	c.handlers[topic] = h
	cli := c.c
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil // re-applied on next OnConnect
	}
	tok := cli.Subscribe(topic, 0, func(_ paho.Client, msg paho.Message) {
		h(msg.Topic(), msg.Payload())
	})
	tok.Wait()
	return tok.Error()
}

// Unsubscribe drops the handler for topic and tells the broker.
func (c *Client) Unsubscribe(topic string) error {
	c.mu.Lock()
	delete(c.handlers, topic)
	cli := c.c
	c.mu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil
	}
	tok := cli.Unsubscribe(topic)
	tok.Wait()
	return tok.Error()
}

// Publish writes payload to topic. retained controls whether the
// broker stores it for future subscribers (Z2M's bridge/devices
// uses retained; /set commands do not).
func (c *Client) Publish(topic string, payload []byte, retained bool) error {
	c.mu.Lock()
	cli := c.c
	c.mu.Unlock()
	if cli == nil {
		return errors.New("mqtt: not connected")
	}
	tok := cli.Publish(topic, 0, retained, payload)
	tok.Wait()
	return tok.Error()
}

// Close disconnects cleanly. Idempotent.
func (c *Client) Close() {
	c.mu.Lock()
	cli := c.c
	c.c = nil
	c.mu.Unlock()
	if cli != nil && cli.IsConnected() {
		cli.Disconnect(250)
	}
}

func tokenDone(t paho.Token) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		t.Wait()
		close(ch)
	}()
	return ch
}
