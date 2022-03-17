package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/skratchdot/open-golang/open"
)

/*
func chromeUserDataDir() (string, error) {
	var subdir string

	// Get the user config dir for the OS.
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	// Get teh user data subdirectory.
	switch runtime.GOOS {
	case "windows":
		subdir = "Local\\Google\\Chrome\\User Data"
	case "darwin", "ios":
		subdir = "Google/Chrome"
	default: // Unix
		subdir = "google-chrome"
	}

	return filepath.Join(dir, subdir), nil
}
*/

func tokenViaRod(url, userDataDir string) (string, error) {
	url = fmt.Sprintf("%s/app/user/profile", url)

	if path, exists := launcher.LookPath(); exists {
		log.Printf(" > Found Browser")

		// Launch a visible browser that the user can interact with.
		u := launcher.New().Bin(path).Headless(false).UserDataDir(userDataDir).MustLaunch()
		log.Printf(" > Launched Browser")

		// Load the page, and give up to 120 seconds for the entire operation - including login.
		page := rod.New().ControlURL(u).MustConnect().Timeout(time.Second * 120).MustPage(url)
		log.Printf(" > Page Opened")

		// Click the show icon
		page.MustElement("#temporary-api-token-card .unmask-password").MustClick()
		log.Printf(" > Icon Clicked")

		time.Sleep(time.Second * 1)

		// Wait until we can see the temp token element.
		token := page.MustElement("#temporary-api-token-card .unmasked-password").MustText()
		return token, nil
	} else {
		return "", errors.New("Google Chrome is not installed")
	}
}

type tokenResult struct {
	token string
	err   error
}

func tokenViaPost(baseUrl string, localPort int) (string, error) {
	url := fmt.Sprintf("%s/app/user/verify-token?localAppName=post-test&localPort=%d", baseUrl, localPort)

	done := make(chan tokenResult)

	// Run the HTTP server on localhost.
	go func() {
		addr := fmt.Sprintf("127.0.0.1:%d", localPort)
		mux := http.NewServeMux()
		log.Printf(" > Starting server")

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
					err = fmt.Errorf("Unauthorized origin: %s", origin)
				} else {
					bytes, err = io.ReadAll(req.Body)
				}
			}

			// Send the proper response.
			if completed {
				if err != nil {
					log.Printf(" ! Reading token: %s", err)
					res.WriteHeader(500)
					status = "failed"
				} else {
					log.Printf(" > Reading token")
					status = "done"
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
	case <-time.After(10 * time.Second):
		return "", errors.New("timed out")
	}
}

var flagApproach = flag.String("approach", "rod", "approach to use")

func main() {
	flag.Parse()

	var err error
	token := ""
	baseUrl := "https://oneclick.duplocloud.net"

	// Use browser automation to read the token.
	if flagApproach == nil || *flagApproach == "rod" {

		// Get the user data dir.
		userDataDir, err := os.UserCacheDir() //chromeUserDataDir()
		if err != nil {
			log.Printf(" ! %s", err)
		}
		userDataDir = filepath.Join(userDataDir, "rod-test")

		// Get the token.
		token, err = tokenViaRod(baseUrl, userDataDir)

		// Use a background server to receive it from the browser
	} else {
		token, err = tokenViaPost(baseUrl, 4201)
	}

	if err != nil {
		log.Printf(" ! %s", err)
	} else {
		log.Printf(" > Token: %s", token)
	}
}
