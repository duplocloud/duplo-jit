package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/skratchdot/open-golang/open"
)

type tokenResult struct {
	token string
	err   error
}

func tokenViaPost(baseUrl string, localPort int, timeout time.Duration) (string, error) {
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=post-test&localPort=%d", baseUrl, localPort)

	done := make(chan tokenResult)

	// Run the HTTP server on localhost.
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", localPort)
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
		_ = http.ListenAndServe(addr, mux)
	}()

	// Open the browser.
	err := open.Run(url)
	if err != nil {
		return "", err
	}

	// Wait for the token result, and return it.
	select {
	case tokenResult := <-done:
		return tokenResult.token, tokenResult.err
	case <-time.After(timeout):
		return "", errors.New("timed out")
	}
}

func mustTokenInteractive(host string) string {

	token, err := tokenViaPost(host, 4201, 180*time.Second)
	dieIf(err, "failed to get token from interactive browser session")

	return token
}
