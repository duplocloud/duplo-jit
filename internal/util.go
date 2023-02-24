package internal

import (
	"log"
	"os"
)

func DieIf(err error, msg string) {
	if err != nil {
		Fatal(msg, err)
	}
}

func Fatal(msg string, err error) {
	log.Fatalf("%s: %s: %s", os.Args[0], msg, err)
}
