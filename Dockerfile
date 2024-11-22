FROM golang:latest AS builder

COPY . /app
WORKDIR /app

RUN make

FROM debian:stable-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/duplo-jit /usr/local/bin/duplo-jit
COPY --from=builder /app/duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process

ENTRYPOINT [ "duplo-jit" ]