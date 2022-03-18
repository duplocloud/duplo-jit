VERSION=0.3.3

default: all

.PHONY:
clean:
	rm -f duplo-aws-credential-process

install: all
	sudo install -o root -m 755 duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process

test: all

all: duplo-aws-credential-process

duplo-aws-credential-process: Makefile duplocloud/*.go cmd/duplo-aws-credential-process/*.go
	go build -ldflags "-X main.version=v$(VERSION)-dev -X main.commit=$(shell git rev-parse HEAD)" ./cmd/duplo-aws-credential-process/
