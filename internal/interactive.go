package internal

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

func handlerToken(baseUrl string, localPort int, admin bool, res http.ResponseWriter, req *http.Request) (completed bool, tokenBytes []byte) {
	// Only allow the specified Duplo to give us creds.
	res.Header().Add("Access-Control-Allow-Origin", baseUrl)
	res.Header().Add("Access-Control-Allow-Headers", "X-Requested-With, Accept, Content-Type")

	// Check if this is a GET request carrying the token in the query string.
	if req.Method == "GET" {
		token := req.URL.Query().Get("t")
		if token == "" {
			// If the token is missing, return an error response.
			http.Error(res, "missing token", http.StatusBadRequest)
			return true, nil
		}
		// Build the redirect URL back to Duplo with success=true.
		adminFlag := ""
		if admin {
			adminFlag = "&isAdmin=true"
		}
		redirectURL := fmt.Sprintf("%s/app/user/verify-token?localAppName=duplo-jit&localPort=%d%s&success=true", baseUrl, localPort, adminFlag)
		// Issue an HTTP redirect.
		http.Redirect(res, req, redirectURL, http.StatusFound)
		return true, []byte(token)
	}

	// If it's a POST request, use legacy behavior.
	if req.Method == "POST" {
		defer req.Body.Close()
		// Authorize by checking the Origin.
		origin := req.Header.Get("Origin")
		var err error
		if origin != baseUrl {
			err = fmt.Errorf("unauthorized origin: %s", origin)
		} else {
			tokenBytes, err = io.ReadAll(req.Body)
		}
		completed = true
		status := "done"
		if err != nil {
			res.WriteHeader(500)
			status = "failed"
		}
		_, _ = fmt.Fprintf(res, "\"%s\"\n", status)
		return completed, tokenBytes
	}

	// For any other method, return a 405 Method Not Allowed.
	res.WriteHeader(http.StatusMethodNotAllowed)
	return false, nil
}

func TokenViaPost(baseUrl string, admin bool, cmd string, port int, timeout time.Duration) TokenResult {

	// Create the listener on a random port.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
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
			completed, tokenBytes := handlerToken(baseUrl, port, admin, res, req)

			// If we are done, send the result to the channel.
			if completed {
				done <- TokenResult{Token: string(tokenBytes), err: err}
			}
		})

		// API with facility for OTP
		mux.HandleFunc("/v2/callbackWithOtp", func(res http.ResponseWriter, req *http.Request) {
			completed, tokenBytes := handlerToken(baseUrl, port, admin, res, req)

			// If we are done, send the result to the channel.
			if completed {
				rp := TokenResult{}
				if len(tokenBytes) == 0 {
					rp.err = errors.New("canceled")
				} else {
					err = json.Unmarshal(tokenBytes, &rp)
					if err != nil {
						message := fmt.Sprintf("%s: cannot unmarshal response from JSON: %s", "/v2/callbackWithOtp", err.Error())
						rp.err = errors.New(message)
					}
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
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=%s&localPort=%d%s&redirect=true", baseUrl, cmd, localPort, adminFlag)
	err = open.Run(url)
	DieIf(err, "failed to open interactive browser session")

	// Wait for the token result, and return it.
	select {
	case tokenResult := <-done:
		return tokenResult
	case <-time.After(timeout):
		return TokenResult{err: errors.New("timed out")}
	}
}

func MustTokenInteractive(host string, admin bool, cmd string, port int) (tokenResult TokenResult) {
	tokenResult = TokenViaPost(host, admin, cmd, port, 180*time.Second)
	DieIf(tokenResult.err, "failed to get token from interactive browser session")
	return
}
