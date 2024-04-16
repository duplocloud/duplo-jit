FROM golang:latest AS builder

COPY . /app
WORKDIR /app

RUN make

FROM debian:stable-slim

COPY --from=builder /app/duplo-jit /usr/local/bin/duplo-jit
COPY --from=builder /app/duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process

ENTRYPOINT [ "duplo-jit" ]