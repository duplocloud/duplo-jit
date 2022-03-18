package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var cacheDir string
var noCache *bool

// mustInitCache initializes the cacheDir or panics.
func mustInitCache() {
	var err error

	if *noCache {
		return
	}

	cacheDir, err = os.UserCacheDir()
	dieIf(err, "cannot find cache directory")
	cacheDir = filepath.Join(cacheDir, "duplo-aws-credential-process")
	err = os.MkdirAll(cacheDir, 0700)
	dieIf(err, "cannot create cache directory")
}

// cacheReadUnmarshal reads JSON and unmarshals into the target, returning true on success.
func cacheReadUnmarshal(file string, target interface{}) bool {

	if !*noCache && cacheDir != "" {
		file = filepath.Join(cacheDir, file)
		bytes, err := os.ReadFile(file)

		if err == nil {
			err = json.Unmarshal(bytes, target)
			if err == nil {
				return true
			}

			log.Printf("warning: %s: invalid JSON in cache: %s", file, err)
		} else if !errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: %s: unable to read from cache", file)
		}
	}

	return false
}

// cacheWriteMustMarshal unmarshals the source and writes JSON.
// It returns the JSON bytes and ignores cache write failures.
func cacheWriteMustMarshal(file string, source interface{}) []byte {

	// Convert the source to JSON
	json, err := json.Marshal(source)
	dieIf(err, "cannot marshal to JSON")

	// Cache the JSON
	if !*noCache {
		file = filepath.Join(cacheDir, file)

		err = os.WriteFile(file, json, 0600)
		if err != nil {
			log.Printf("warning: %s: unable to write to cache", file)
		}
	}

	return json
}

// cacheGetAwsConfigOutput tries to read prior AWS creds fromt the cache.
func cacheGetAwsConfigOutput(cacheKey string) (creds *AwsConfigOutput) {
	var file string

	// Read credentials from the cache.
	if !*noCache {
		file = fmt.Sprintf("%s,aws-creds.json", cacheKey)
		creds = &AwsConfigOutput{}
		if !cacheReadUnmarshal(file, creds) {
			creds = nil
		}
	}

	// Check credentials for expiry.
	if creds != nil {
		five_minutes_from_now := time.Now().UTC().Add(5 * time.Minute)
		expiration, err := time.Parse(time.RFC3339, creds.Expiration)

		// Invalid expiration?
		if err != nil {
			log.Printf("warning: %s: invalid Expiration time in credentials cache: %s", cacheKey, creds.Expiration)
			creds = nil

			// Expires in five minutes or less?
		} else if five_minutes_from_now.After(expiration) {
			creds = nil
		}
	}

	// Clear the cache if the creds expired.
	if creds == nil && file != "" {
		err := os.Remove(file)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("warning: %s: unable to remove from credentials cache", cacheKey)
		}
	}

	return
}
