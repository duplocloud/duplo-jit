VERSION=0.4.1

default: all

.PHONY:
clean:
	rm -f duplo-aws-credential-process *.exe

install: all
	sudo install -o root -m 755 duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process

test: all

all: duplo-aws-credential-process

duplo-aws-credential-process: Makefile duplocloud/*.go cmd/duplo-aws-credential-process/*.go
	go build -ldflags "-X main.version=$(VERSION)-dev -X main.commit=$(shell git rev-parse HEAD)" ./cmd/duplo-aws-credential-process/
