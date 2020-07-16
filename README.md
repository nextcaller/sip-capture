# sip-capture agent

## What is it and why would I want it?

`sip-capture` agent acts like an IoT sensor for your SIP call signalling.  By
deploying it where it can monitor SIP-based network traffic it will capture
specific SIP signalling (such as all initial INVITE requests) and forward it as
"sensor data" to be consumed by other applications.  The resulting data can be
used to monitor statistics about your VoIP traffic, stored for call flow
debugging, used for billing, or connected to more advanced applications (such
as NextCaller's Vericall product) to do real-time detection of call spoofing or
analysis of fraud potential.

The surface area for code and configuration of `sip-capture` is deliberately
small so that it can be easily audited and deployed in a secure fashion.  One
goal is that a single engineer who is proficient in Go should be able to review
and understand the behavior of the whole `sip-capture` codebase in no more than
an afternoon.

## How does it work?

At it's core, `sip-capture` is simple:  it's a small Go application that uses
gopacket/libpcap to listen for SIP signalling, filters to select only the
desired messages, and then encapsulates each message in a simple data structure
and then publishes them to an MQTT topic.

There's a bit to unpack in that statement:

### A small Go application

The code base is simple and well documented enough you can audit it and
understand it yourself.  You should be able to confidently deploy it in your
stack, knowing that there are no unanticipated malicious surprises waiting for
you.  Because it's Go, it can be compiled for nearly any sort of Unix-like
system or you can use an existing pre-compiled Docker image.

### Use libpcap to capture SIP signaling

`sip-capture` is a passive SIP sensor, you deploy it where it can see your SIP
traffic; possibly directly on your SBC, on the same network switch/VLAN, or by
using port mirroring or spanning to deliver a copy of the SIP traffic to the
host where the agent is running.  It also gives you access to the full BPF
filtering capabilities of libpcap to narrow down capture to only specific sorts
of network traffic, to ensure you're not capturing unnecessary packets.

### Filters to select desired SIP messages

BPF works well for selecting transport-level network packets, but above and
beyond that, `sip-capture` contains a small filtering language that can further
inspect the SIP signaling for specific traits.  Select only certain SIP
methods, only requests or responses, only certain statuses, or even do full
regex selection on message headers or bodies.  Messages can also be
specifically excluded if they match certain criteria.

### Encapsulates each message in a simple data structure

SIP signaling can contain arbitrary data, including non-ASCII and even binary
encodings.  To be sure it can be transmitted cleanly over any transport, the
raw SIP message is wrapped in an JSON encoding structure, including some
metadata like the timestamp of capture, a generated message ID for
deduplication, and the version of `sip-agent` used for capture.  This makes it
easy to transmit and store the data without worrying about corruption or losing
fidelity.

### Publishes to an MQTT topic

MQTT messaging has become one of the "native" industry standard protocols for
IoT type sensors in recent years.  You can use standard off the shelf MQTT
brokers to receive the SIP signalling data and allow multiple applications to
subscribe to the appropriate topic to receive a copy for whatever processing
they choose.  Many cloud providers also provide MQTT integrations, including
AWS IoT and IoT Greengrass, Azure IoT, and Google Cloud IoT.  You can integrate
your SIP stack with an IoT rule engines to build enhanced call flows.

## What sip-capture doesn't (and should not) do

`sip-capture` is designed for simplicity, auditability, and good old fashioned
Unix *do one thing well* philosophy.  As such, there are a lot of things it could
do, but does not.

- It does not alter captured SIP messages in any way.
- It does not specifically extract any reversible identifiable data from the
  captured messages; it does inspect the message to apply filter rules to
  determine if messages should be published or not, and to generate unique
  message signature hashes as part of the capture metadata.
- It does not locally persist any data or configuration; it does emit operating
  logs on standard output, which the host environment may be configured to
  capture.
- It does not transmit any SIP data except over the configured MQTT topic
- It does not change what MQTT topic it publishes to based on any part of the
  message or operating environment, beyond start up configuration.
- It does not guarantee delivery, beyond MQTT QoS 1 with a backoff retry.
- It does not accept any form of configuration or control messages over MQTT,
  nor any form of device shadow, that could potentially cause it to change any
  behavior from how it was configured at start up.

`sip-capture` is meant to be one building block in a larger solution, and all
of these capabilities can be provided by other applications, primarily over
MQTT, such as being part of an AWS IoT Greengrass group, cloud IoT Core rules
engines, or even other local services attached to the same broker.  Excluding
them from `sip-agent` helps enable safer deployments on sensitive segments of
your network and to keep potentially dangerous capabilities outside that zone.

Because these features can be handled by other applications and would reduce
the simple and auditable nature of the code, requests for these capabilities
are very likely to be politely declined.


# Configuration Options

## Logging and Metrics

log level - string - optional - DEBUG, INFO, WARNING, ERROR.  Sets how verbose
the agent is.  INFO is default and generally only logs information about
startup and shutdown behaviors, and things like MQTT reconnections or timeouts.
DEBUG is very verbose, logging many lines for every single packet seen and may
include information contained within SIP packets.

Metrics Endpoint Address - string - optional - If set, `sip-capture` exports
Prometheus metrics on the `/metrics` path for integrating with standard
monitoring tools.  By default metrics are disabled.  Examples: `:9090`,
`0.0.0.0:8080`.

## Network and Packet Selection

interface - string - required - which networking interface to capture on;
this should be the name as libpcap expects to use it (such as 'eth0'), and
should be an interface that can be put into promiscuous mode to observe
your SIP traffic.

BPF filter - string - optional - If unset, `sip-capture` will see all
network traffic.  It will automatically drop any traffic which is not
SIP related.  However, you can improve efficiency by setting this to only
monitor the appropriate hosts, ports, and protocols that carry your SIP
traffic.  Of note, you may need to ensure that IP fragments are included in
the BPF filter or SIP messages fragmented across multiple packets may not be
captured correctly.  Assuming you're running your SIP signaling over UDP on the
IANA standard port 5060, the following example should select all SIP signaling:

```
(udp and port 5060) or (ip[6:2] & 0x1fff) != 0
```

This uses the filter language that
[libpcap](https://www.tcpdump.org/manpages/pcap-filter.7.html) understands.

SIP filters - string - optional - use the [DSL in the filters
directory](filters/doc.go) to select only the SIP messages of interest.  If no
filter is specified, every SIP packet selected by the BPF filter will be sent.

## MQTT Publishing

Broker - string - required - URL of where to connect to deliver mqtt.  Must
have one of the schemes 'tcp', 'ssl', or 'ws' (per paho.mqtt.golang client).
For example:  `tcp://localhost`, `tcp://broker.local.domain:1883`, or
`ssl://someaccount.iot.mycloudprovider.com:8883`

Message Topic - string - required - the topic upon which each selected SIP
message is published.  This can be any valid MQTT topic.  Examples:
`/my-company/nyc/pbx-2/sip-capture` or `/sip/debug/customer/alice`

ClientID - string - optional - if not set, will generate one based on the
machine environment.

TLS Certificate Files - strings - optional - if set, will load these as a TLS
client certificate and require their use connecting to the Broker.
