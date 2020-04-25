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
