FROM golang:buster AS builder

WORKDIR /src

# Install things we need in order to build CGO with libpcap, compress the
# resulting binary, and then run it as non-root in the prod container.
RUN apt-get update && apt-get install -y upx-ucl libpcap-dev libcap2-bin

# This will cache all our dependencies so long as neither go.* file changes.
COPY go.sum go.mod ./
RUN go mod download

# Now we can iterate on our actual code without having to re-fetch the module
# dependencies every time we do `docker build`.
COPY . ./

# Do the build, with appropriate variables.
RUN make clean build

# Make a single location for all the files we're going to copy over to the prod
# container to avoid unnecessary extra image layers.
RUN mkdir -p /dist /dist/usr/lib/x86_64-linux-gnu/ /dist/lib/x86_64-linux-gnu/ /dist/sbin \
 && upx -9 -o /dist/sip-capture /src/sip-capture \
 && cp /usr/lib/x86_64-linux-gnu/libpcap* /dist/usr/lib/x86_64-linux-gnu/ \
 && cp /lib/x86_64-linux-gnu/libcap* /dist/lib/x86_64-linux-gnu/ \
 && cp /sbin/setcap /dist/sbin/setcap

# Use distroless to minimize both image size and attack surface.
# base-debian10 over static-debian10 because we have cgo linkage for libpcap
FROM gcr.io/distroless/base-debian10

# Copy necessary files from builder; this includes not just the binary but the
# pcap libraries and the setcap/libcap necessary to run rootless.
COPY --from=builder /dist /

# Run as nonroot and still allow use of pcap, via setcap.
# COPY will not preserve xattrs, so we must run this in the prod container.
# There's no shell in distroless, so use vector of args form.
RUN ["/sbin/setcap", "CAP_NET_RAW,CAP_NET_BIND_SERVICE=+eip", "/sip-capture"]

# Yay, that much less attack surface.
USER nonroot

# Expose the default bind for Prometheus /metrics endpoint.
# Normally unnecessary; this container is probably going to run network_mode host.
# Mostly only useful for testing or in scenarios where sip-capture is running
# in a container in the same docker network as the source of SIP messages.
EXPOSE 9900

CMD ["/sip-capture"]

# Set these after the caching layers so that we don't have to do any hard work
# if nothing has changed.
ARG VERSION=local
ARG BUILD_REF=unknown
ARG BUILD_DATE=unknown

LABEL	org.opencontainers.image.title="sip-capture" \
	org.opencontainers.image.description="Capture and filter SIP signaling and forward via MQTT" \
	org.opencontainers.image.vendor="NextCaller" \
	org.opencontainers.image.url="http://nextcaller.com" \
	org.opencontainers.image.source="https://github.com/nextcaller/sip-capture" \
	org.opencontainers.image.licenses="MIT" \
	org.opencontainers.image.version="${VERSION}" \
	org.opencontainers.image.revision="${BUILD_REF}" \
	org.opencontainers.image.created="${BUILD_DATE}"
