# Build the manager binary
FROM golang:1.26.1-bookworm@sha256:ab3d6955bbc813a0f3fdf220c1d817dd89c0b3f283777db8ece4a32fe7858edd AS builder

# Make sure we use go modules
WORKDIR vcluster

# Copy the Go Modules manifests
COPY . .

# Install dependencies
RUN go mod vendor

# Build cmd
RUN CGO_ENABLED=0 GO111MODULE=on go build -mod vendor -o /plugin main.go

# we use alpine for easier debugging
FROM alpine

# Set root path as working directory
WORKDIR /

COPY --from=builder /plugin .

ENTRYPOINT ["/plugin"]
