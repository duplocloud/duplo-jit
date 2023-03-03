VERSION=0.5.3

default: all

.PHONY:
clean:
	rm -f duplo-aws-credential-process *.exe
	rm -f duplo-jit *.exe

install: all
	sudo install -o root -m 755 duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process
	sudo install -o root -m 755 duplo-jit /usr/local/bin/duplo-jit

test: all

all: duplo-aws-credential-process duplo-jit

duplo-jit: Makefile duplocloud/*.go internal/*.go cmd/duplo-jit/*.go
	go build -ldflags "-X main.version=$(VERSION)-dev -X main.commit=$(shell git rev-parse HEAD)" ./cmd/duplo-jit/

duplo-aws-credential-process: Makefile duplocloud/*.go internal/*.go cmd/duplo-aws-credential-process/*.go
	go build -ldflags "-X main.version=$(VERSION)-dev -X main.commit=$(shell git rev-parse HEAD)" ./cmd/duplo-aws-credential-process/
