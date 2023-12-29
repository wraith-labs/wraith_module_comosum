FROM docker.io/golang:1.21-alpine AS builder

WORKDIR /build/

COPY . /build/

RUN cd /build && \
    apk add build-base && \
    go install mvdan.cc/garble@latest && \
    go version && garble version && \
    garble build -trimpath -o wmc3 cmd/wm3/*.go

FROM alpine

COPY --from=builder /build/wmc3 /usr/bin/wmc3

ENV \
WMC3_DEBUG = "false" \
WMC3_YGG_IDENTITY = "" \
WMC3_YGG_STATIC_PEERS = "" \
WMC3_YGG_LISTENERS = "" \
WMC3_NATS_ADMIN_USER = "" \
WMC3_NATS_ADMIN_PASS = "" \
WMC3_NATS_LISTENER = "0.0.0.0:4222"

ENTRYPOINT ["/usr/bin/wmc3"]