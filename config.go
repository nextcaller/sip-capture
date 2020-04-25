package main

import (
	"flag"
	"os"

	"github.com/nextcaller/sip-capture/publisher"
)

type config struct {
	LogLevel    string
	Interface   string
	BPFFilter   string
	SIPFilter   string
	MetricsAddr string
	MQTT        publisher.MQTTOptions
}

func defEnvStr(k, dval string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return dval
}

func (c *config) Load(args []string) error {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.StringVar(&c.LogLevel, "log-level", defEnvStr("LOG_LEVEL", "info"), "logging level (debug, info, error)")
	fs.StringVar(&c.Interface, "interface", defEnvStr("INTERFACE", "lo"), "Interface for pcap to capture from")
	fs.StringVar(&c.BPFFilter, "bpf-filter", defEnvStr("BPF_FILTER", "udp and port 5060"), "pcap BPF packet selection filter")
	fs.StringVar(&c.SIPFilter, "sip-filter", defEnvStr("SIP_FILTER", ""), "SIP selection filter")
	fs.StringVar(&c.MetricsAddr, "metric-filter", defEnvStr("METRICS_ADDR", ""), "IP:Port to bind for /metrics endpoint")

	fs.StringVar(&c.MQTT.Broker, "broker", defEnvStr("BROKER", "tcp://localhost:1883"), "MQTT broker")
	fs.StringVar(&c.MQTT.ClientID, "client-id", defEnvStr("CLIENT_ID", ""), "MQTT Client ID")
	fs.StringVar(&c.MQTT.Topic, "topic", defEnvStr("TOPIC", ""), "MQTT publishing topic for SIP data")
	fs.StringVar(&c.MQTT.Telemetry, "telemetry-topic", defEnvStr("TELEMETRY_TOPIC", ""), "MQTT publishing topic for telemetry")
	fs.StringVar(&c.MQTT.TLSKeyFile, "key-file", defEnvStr("KEY_FILE", ""), "MQTT TLS key file (pem)")
	fs.StringVar(&c.MQTT.TLSCertFile, "cert-file", defEnvStr("CERT_FILE", ""), "MQTT TLS cert file (pem)")

	return fs.Parse(args[1:])
}
