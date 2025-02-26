FROM golang:1.23-bullseye AS builder
WORKDIR /app

RUN mkdir -p /root/.cache/go-build
RUN go env -w GOMODCACHE=/root/.cache/go-build && go env -w CGO_ENABLED=1

COPY go.mod go.sum Makefile ./
RUN --mount=type=cache,target=/root/.cache/go-build make deps

COPY . .

RUN make build

FROM alpine:latest

RUN apk add --no-cache ca-certificates libc6-compat gcompat

COPY --from=builder /app/build/loadtest /usr/local/bin/loadtest

ENTRYPOINT ["loadtest", "-config"]
