package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/term"
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
		// URL-decode the token.
		decodedToken, err := url.QueryUnescape(token)
		if err != nil {
			http.Error(res, "invalid token encoding", http.StatusBadRequest)
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
		return true, []byte(decodedToken)
	}

	// If it's a POST request, use legacy behavior.
	if req.Method == "POST" {
		defer func() { _ = req.Body.Close() }()
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

func TokenViaListener(baseUrl string, admin bool, cmd string, port int, timeout time.Duration) TokenResult {
	cooldownDuration, cooldownEnabled := IsAuthCooldownEnabled()
	isTTY := term.IsTerminal(int(os.Stderr.Fd()))

	openBrowser := true
	listenPort := port

	// If auth cooldown is enabled and we are NOT in a TTY (i.e. automated caller like
	// kubectl), check before creating the listener. TTY callers (user at terminal)
	// always bypass the cooldown for immediate access.
	if cooldownEnabled && !isTTY {
		var earlyResult *TokenResult
		listenPort, openBrowser, timeout, earlyResult = checkCooldownBeforeListen(baseUrl, admin, cmd, port, cooldownDuration)
		if earlyResult != nil {
			return *earlyResult
		}
	} else if cooldownEnabled {
		log.Printf("auth cooldown: TTY detected, bypassing cooldown for %s", GetHostCacheKey(baseUrl))
	}

	// Create the listener.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", listenPort))
	if err != nil {
		if listenPort != 0 && listenPort != port {
			return recoverRelayBindFailure(baseUrl, admin, cmd, port, listenPort, cooldownDuration)
		}
		return TokenResult{err: err}
	}

	// Get the port being listened to.
	localPort := listener.Addr().(*net.TCPAddr).Port

	// Set or update cooldown now that we have the port.
	if cooldownEnabled && !isTTY {
		if result := acquireOrUpdateCooldown(baseUrl, admin, cmd, port, localPort, openBrowser, cooldownDuration, listener); result != nil {
			return *result
		}
	}

	// Run the HTTP server on localhost.
	done := make(chan TokenResult)
	go func() {
		mux := http.NewServeMux()

		// legacy API, with no facility for OTP
		mux.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
			completed, tokenBytes := handlerToken(baseUrl, localPort, admin, res, req)

			// If we are done, send the result to the channel.
			if completed {
				done <- TokenResult{Token: string(tokenBytes)}
			}
		})

		// API with facility for OTP
		mux.HandleFunc("/v2/callbackWithOtp", func(res http.ResponseWriter, req *http.Request) {
			completed, tokenBytes := handlerToken(baseUrl, localPort, admin, res, req)

			// If we are done, send the result to the channel.
			if completed {
				rp := getTokenResult(tokenBytes)
				done <- rp
			}
		})

		_ = http.Serve(listener, mux)
	}()

	// Open the browser only for fresh starts (not port-reuse relays).
	if openBrowser {
		url := getInteractiveUrl(admin, baseUrl, cmd, localPort)
		err = open.Run(url)
		DieIf(err, "failed to open interactive browser session")
	} else {
		log.Printf("auth cooldown: relay — listening on port %d for existing browser tab", localPort)
	}

	// Wait for the token result, and return it.
	select {
	case tokenResult := <-done:
		if tokenResult.err == nil {
			ClearAuthCooldown(baseUrl, admin)
		}
		return tokenResult
	case <-time.After(timeout):
		_ = listener.Close()
		return TokenResult{err: errors.New("timed out")}
	}
}

func getTokenResult(tokenBytes []byte) TokenResult {
	tokenResult := TokenResult{}
	if len(tokenBytes) == 0 {
		tokenResult.err = errors.New("canceled")
	} else {
		err := json.Unmarshal(tokenBytes, &tokenResult)
		if err != nil {
			message := fmt.Sprintf("%s: cannot unmarshal response from JSON: %s", "/v2/callbackWithOtp", err.Error())
			tokenResult.err = errors.New(message)
		}
	}
	return tokenResult
}

func getInteractiveUrl(admin bool, baseUrl string, cmd string, localPort int) string {
	adminFlag := ""
	if admin {
		adminFlag = "&isAdmin=true"
	}
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=%s&localPort=%d%s&redirect=true", baseUrl, cmd, localPort, adminFlag)
	return url
}

func MustTokenInteractive(host string, admin bool, cmd string, port int) (tokenResult TokenResult) {
	tokenResult = TokenViaListener(host, admin, cmd, port, 180*time.Second)
	DieIf(tokenResult.err, "failed to get token from interactive session (timed out or canceled)")
	return
}
