#
## Build stage
#FROM golang:alpine AS builder
#
#WORKDIR /workspace
#
## Copy the Go Modules manifests
#COPY go.mod go.sum ./
#
## Cache dependencies
#RUN go mod download
#
## Copy the source code
#COPY . .
#
## Build the binary
#RUN CGO_ENABLED=0 GOOS=linux GOMAXPROCS=$(nproc) go build -o bin/bvcnid cmd/bvcnid/main.go
#
## Final stage
#FROM alpine:3.14
#
## Install necessary packages
#RUN apk --no-cache update && apk --no-cache add iptables
#
## Copy the binary from the build stage
#COPY --from=builder /workspace/bin/bvcnid /bvcnid
#
## Set the entrypoint
#ENTRYPOINT ["/bvcnid"]



# Build stage
FROM golang:alpine AS builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.sum ./

# Cache dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the binary
RUN set -e; \
    GOOS=linux GOMAXPROCS=$(nproc) go build -o bin/bvcnid cmd/bvcnid/main.go; \
    GOOS=linux go build -o bin/bvcni cmd/bvcni/main.go

# Final stage
FROM alpine:3.14

# Install necessary packages
RUN apk --no-cache update && apk --no-cache add iptables

# Copy the binary from the build stage
COPY --from=builder /workspace/bin/bvcnid /bvcnid
COPY --from=builder /workspace/bin/bvcni /bvcni

# Create a shell script to copy CNI binary and execute bvcnid
RUN echo -e '#!/bin/sh\n\
cp /bvcni /host/opt/cni/bin/bvcni\n\
exec /bvcnid "$@"' > /entrypoint.sh \
    && chmod +x /entrypoint.sh

# Set the entrypoint
ENTRYPOINT ["/entrypoint.sh"]
