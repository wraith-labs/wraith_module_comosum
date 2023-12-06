FROM docker.io/golang:1.21-alpine AS builder

WORKDIR /build/

COPY . /build/

RUN cd /build && \
    apk add build-base olm-dev && \
    go version && \
    go build -trimpath -o pc3 cmd/pc3/*.go

FROM alpine

COPY --from=builder /build/pc3 /usr/bin/pc3

ENV \
WMP_HOMESERVER= \
WMP_USERNAME= \
WMP_PASSWORD= \
WMP_ADMIN_ROOM= \
WMP_ID_PINECONE= \
WMP_LOG_PINECONE= \
WMP_INBOUND_TCP_PINECONE= \
WMP_INBOUND_WEB_PINECONE= \
WMP_DEBUG_ENDPOINT_PINECONE= \
WMP_USE_MULTICAST_PINECONE= \
WMP_STATIC_PEERS_PINECONE=

ENTRYPOINT ["/usr/bin/pc3"]