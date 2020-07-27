package publisher

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/nextcaller/sip-capture/collect"
	"github.com/rs/zerolog"
)

const (
	// MQTTQOSOne is a constant representing QOS level 1 when publishing.
	MQTTQOSOne = byte(1)
	// defaultResponseTimeout is how long to wait for the broker to respond to
	// a single MQTT operation.
	defaultResponseTimeout = time.Second * 2

	// keepaliveTimeout is how often to make MQTT Keepalive requests.
	keepaliveTimeout = time.Second * 30

	// disconnectQueisce is how long to wait for the server during disconnects;
	// measured in milliseconds.  see `go doc paho.mqtt.golang.Client.Disconnect`
	disconnectQuiesce = 250
)

var (
	// ErrPublishTimeout should only happen if the broker is unresponsive.
	ErrPublishTimeout = errors.New("mqtt publish timed out")
)

func timeoutFromCtx(ctx context.Context, def time.Duration) time.Duration {
	if dl, ok := ctx.Deadline(); ok {
		return time.Until(dl)
	}
	return def
}

// MQTTPublisher knows how to Publish a collect.Msg to a given topic on its
// connected broker.
type MQTTPublisher struct {
	client mqtt.Client
	opts   MQTTOptions
}

// MQTTOptions controls how the internal mqtt client is created.
type MQTTOptions struct {
	Topic       string
	Telemetry   string
	Broker      string
	ClientID    string
	TLSKeyFile  string
	TLSCertFile string
}

func (m *MQTTPublisher) sendMsg(ctx context.Context, topic string, data []byte) error {
	log := zerolog.Ctx(ctx)
	log.Debug().Bytes("msg", data).Msg("publishing mqtt message")
	token := m.client.Publish(topic, MQTTQOSOne, false, data)

	timeout := timeoutFromCtx(ctx, defaultResponseTimeout)

	// does not handle early ctx cancellation correctly.
	if !token.WaitTimeout(timeout) {
		return ErrPublishTimeout
	}
	if token.Error() != nil {
		return fmt.Errorf("mqtt publish failed: %w", token.Error())
	}
	return nil
}

// Publish encodes a collect.Msg into json and sends it to the broker with
// QoS level 1.
func (m *MQTTPublisher) Publish(ctx context.Context, msg *collect.Msg) error {
	jbytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshaling Msg to json: %w", err)
	}
	return m.sendMsg(ctx, m.opts.Topic, jbytes)
}

// Connect initiates a client MQTT connection to the configured broker.
func (m *MQTTPublisher) Connect(ctx context.Context) error {
	log := zerolog.Ctx(ctx)
	timeout := timeoutFromCtx(ctx, defaultResponseTimeout)
	token := m.client.Connect()
	for {
		if ctx.Err() != nil {
			log.Debug().Msg("context timed out, waiting for mqtt connect")
			return ctx.Err()
		}
		if token.WaitTimeout(timeout) {
			log.Debug().Msg("mqtt connect returned")
			break
		}
	}
	if token.Error() != nil {
		return fmt.Errorf("mqtt connect failed: %w", token.Error())
	}
	return nil
}

// Close disconnects from the broker.
func (m *MQTTPublisher) Close() {
	m.client.Disconnect(disconnectQuiesce)
}

func tlsCfgFromFiles(key, cert string) (*tls.Config, error) {
	certs, err := tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return nil, fmt.Errorf("loading tls keypair: %w", err)
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{certs}}
	return cfg, nil
}

// NewMQTT creates an MQTTPublisher from the given options.
func NewMQTT(o MQTTOptions) *MQTTPublisher {
	if o.ClientID == "" {
		o.ClientID = fmt.Sprintf("sip-capture:%v", time.Now().UnixNano())
	}

	opts := mqtt.NewClientOptions().
		AddBroker(o.Broker).
		SetClientID(o.ClientID).
		SetKeepAlive(keepaliveTimeout)

	if o.TLSKeyFile != "" && o.TLSCertFile != "" {
		cfg, err := tlsCfgFromFiles(o.TLSKeyFile, o.TLSCertFile)
		if err != nil {
			opts.SetTLSConfig(cfg)
		}
	}
	client := mqtt.NewClient(opts)

	return &MQTTPublisher{
		opts:   o,
		client: client,
	}
}
