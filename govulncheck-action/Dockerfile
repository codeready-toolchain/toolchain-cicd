################################################################################################
# Builder image
# See https://hub.docker.com/_/golang/
################################################################################################
FROM golang:1.22 as builder

ARG OS=linux
ARG ARCH=amd64

WORKDIR /usr/src/app

# pre-copy/cache parent go.mod for pre-downloading dependencies and only redownloading them in subsequent builds if they change
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN go build -v -o govulncheck main.go

################################################################################################
# image to be run by the GitHub Action job
################################################################################################
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest as govulncheck

# Copy the generated binary into the $PATH so it can be invoked
COPY --from=builder /usr/src/app/govulncheck /usr/local/bin/

# Run as non-root user
USER 1001

ENTRYPOINT ["/usr/local/bin/govulncheck"]