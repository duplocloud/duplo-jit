VERSION=0.3.1

default: all

.PHONY:
clean:
	rm -f duplo-aws-credential-process

install: all
	sudo install -o root -m 755 duplo-aws-credential-process /usr/local/bin/duplo-aws-credential-process

test: all

all: duplo-aws-credential-process

duplo-aws-credential-process: duplocloud/*.go cmd/duplo-aws-credential-process/*.go
	go build ./cmd/duplo-aws-credential-process/
