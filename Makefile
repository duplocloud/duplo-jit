default: all

.PHONY:
clean:
	rm -f duplo-aws-credential-process

all: duplo-aws-credential-process

duplo-aws-credential-process: duplocloud/*.go cmd/duplo-aws-credential-process/*.go
	go build ./cmd/duplo-aws-credential-process/
