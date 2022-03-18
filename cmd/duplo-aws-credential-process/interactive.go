package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/skratchdot/open-golang/open"
)

type tokenResult struct {
	token string
	err   error
}

func tokenViaPost(baseUrl string, timeout time.Duration) (string, error) {

	// Create the listener on a random port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	// Get the port being listened to.
	localPort := listener.Addr().(*net.TCPAddr).Port

	// Run the HTTP server on localhost.
	done := make(chan tokenResult)
	go func() {
		mux := http.NewServeMux()

		mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
			var bytes []byte
			var err error
			completed := false
			status := "ok"

			// Only allow the specified Duplo to give us creds.
			res.Header().Add("Access-Control-Allow-Origin", baseUrl)

			// A POST means we are done, whether good or bad.
			if req.Method == "POST" {
				defer req.Body.Close()

				completed = true

				// Authorize the origin, and get the POST body.
				origin := req.Header.Get("Origin")
				if origin != baseUrl {
					err = fmt.Errorf("unauthorized origin: %s", origin)
				} else {
					bytes, err = io.ReadAll(req.Body)
				}
			}

			// Send the proper response.
			if completed {
				if err != nil {
					res.WriteHeader(500)
					status = "failed"
				} else {
					status = "done"
					if len(bytes) == 0 {
						err = errors.New("canceled")
					}
				}
			}
			_, _ = fmt.Fprintf(res, "\"%s\"\n", status)

			// If we are done, send the result to the channel.
			if completed {
				done <- tokenResult{token: string(bytes), err: err}
			}
		})
		_ = http.Serve(listener, mux)
	}()

	// Open the browser.
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=duplo-aws-credential-process&localPort=%d", baseUrl, localPort)
	err = open.Run(url)
	dieIf(err, "failed to open interactive browser session")

	// Wait for the token result, and return it.
	select {
	case tokenResult := <-done:
		return tokenResult.token, tokenResult.err
	case <-time.After(timeout):
		return "", errors.New("timed out")
	}
}

func mustTokenInteractive(host string) string {
	token, err := tokenViaPost(host, 180*time.Second)
	dieIf(err, "failed to get token from interactive browser session")

	return token
}
