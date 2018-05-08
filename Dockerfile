FROM golang:1.10.1 AS builder

RUN apt-get update && apt-get install -y make git

COPY . /go/src/github.com/docker/swarmkit

WORKDIR /go/src/github.com/docker/swarmkit

RUN make bin/swarmctl

FROM scratch

COPY --from=builder /go/src/github.com/docker/swarmkit/bin /bin
