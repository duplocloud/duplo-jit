package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/skratchdot/open-golang/open"
)

type TokenResult struct {
	Token string `json:"token"`
	OTP   string `json:"otp,omitempty"`
	err   error
}

func handlerTokenViaPost(baseUrl string, res http.ResponseWriter, req *http.Request) (completed bool, bytes []byte) {
	var err error
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

	// TODO: output any errors to the console

	_, _ = fmt.Fprintf(res, "\"%s\"\n", status)

	return
}

func tokenViaPost(baseUrl string, admin bool, timeout time.Duration) TokenResult {

	// Create the listener on a random port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return TokenResult{err: err}
	}

	// Get the port being listened to.
	localPort := listener.Addr().(*net.TCPAddr).Port

	// Run the HTTP server on localhost.
	done := make(chan TokenResult)
	go func() {
		mux := http.NewServeMux()

		// legacy API, with no facility for OTP
		mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
			completed, bytes := handlerTokenViaPost(baseUrl, res, req)

			// If we are done, send the result to the channel.
			if completed {
				done <- TokenResult{Token: string(bytes), err: err}
			}
		})

		// API with facility for OTP
		mux.HandleFunc("/v2/callbackWithOtp", func(res http.ResponseWriter, req *http.Request) {
			completed, bytes := handlerTokenViaPost(baseUrl, res, req)

			// If we are done, send the result to the channel.
			if completed {
				rp := TokenResult{}
				err = json.Unmarshal(bytes, &rp)
				if err != nil {
					message := fmt.Sprintf("%s: cannot unmarshal response from JSON: %s", "/v2/callbackWithOtp", err.Error())
					rp.err = errors.New(message)
				}
				done <- rp
			}
		})

		_ = http.Serve(listener, mux)
	}()

	// Open the browser.
	adminFlag := ""
	if admin {
		adminFlag = "&isAdmin=true"
	}
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=duplo-aws-credential-process&localPort=%d%s", baseUrl, localPort, adminFlag)
	err = open.Run(url)
	dieIf(err, "failed to open interactive browser session")

	// Wait for the token result, and return it.
	select {
	case tokenResult := <-done:
		return tokenResult
	case <-time.After(timeout):
		return TokenResult{err: errors.New("timed out")}
	}
}

func mustTokenInteractive(host string, admin bool) (tokenResult TokenResult) {
	tokenResult = tokenViaPost(host, admin, 180*time.Second)
	dieIf(tokenResult.err, "failed to get token from interactive browser session")
	return
}
