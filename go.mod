module github.com/nextcaller/sip-capture

go 1.13

require (
	github.com/eclipse/paho.mqtt.golang v1.2.0
	github.com/google/gopacket v1.1.18-0.20200612154125-403ca653c45d
	github.com/matryer/is v1.3.0
	github.com/povilasv/prommod v0.0.12
	github.com/prometheus/client_golang v1.7.1
	github.com/prometheus/common v0.10.0
	github.com/rs/zerolog v1.19.0
	golang.org/x/sys v0.0.0-20200625212154-ddb9806d33ae // indirect
	google.golang.org/protobuf v1.25.0 // indirect
)

// Until https://github.com/google/gopacket/pull/793 is merged.
replace github.com/google/gopacket => github.com/daroot/gopacket v1.1.18-0.20200622011357-62661eb151ef
