FROM docker.io/node:18-alpine AS uibuilder

WORKDIR /build/

COPY ./cmd/pc3/ui /build/ui

RUN cd /build/ui && \
    rm -rf dist/* && \
    npm install && \
    npm run build

FROM docker.io/golang:1.19-alpine AS builder

WORKDIR /build/

COPY . /build/

COPY --from=uibuilder /build/ui/dist/. /build/cmd/pc3/ui/dist

RUN cd /build && \
    apk add build-base && \
    go version && \
    go build -trimpath -o pc3 cmd/pc3/*.go

FROM alpine

COPY --from=builder /build/pc3 /usr/bin/pc3

ENTRYPOINT ["/usr/bin/pc3"]