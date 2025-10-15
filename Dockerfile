FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS builder
WORKDIR /app

ARG TARGETOS
ARG TARGETARCH

ENV GOOS=$TARGETOS
ENV GOARCH=$TARGETARCH
ENV CGO_ENABLED=0

RUN mkdir -p /root/.cache/go-build
RUN go env -w GOMODCACHE=/root/.cache/go-build

COPY go.mod go.sum Makefile ./
RUN --mount=type=cache,target=/root/.cache/go-build make deps

COPY . .

RUN make build

FROM alpine:latest

RUN apk add --no-cache ca-certificates libc6-compat gcompat

COPY --from=builder /app/wallets.json /usr/local/wallets.json
COPY --from=builder /app/txs.json /usr/local/txs.json
COPY --from=builder /app/build/loadtest /usr/local/bin/loadtest

ENTRYPOINT ["loadtest", "-config"]
