# Multi-stage build for petri-pilot
FROM golang:1.25-alpine AS builder

ARG VERSION=dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build \
    -ldflags "-s -w -X github.com/pflow-xyz/petri-pilot/internal/version.Version=${VERSION}" \
    -o /petri-pilot ./cmd/petri-pilot

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
COPY --from=builder /petri-pilot /usr/local/bin/petri-pilot
ENTRYPOINT ["petri-pilot"]
CMD ["mcp"]
