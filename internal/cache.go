package internal

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/duplocloud/duplo-aws-jit/duplocloud"
)

var cacheDir string
var noCache bool

// MustInitCache initializes the cacheDir or panics.
func MustInitCache(cmd string, disabled bool) {
	var err error

	noCache := disabled
	if noCache {
		return
	}

	cacheDir, err = os.UserCacheDir()
	DieIf(err, "cannot find cache directory")
	cacheDir = filepath.Join(cacheDir, cmd)
	err = os.MkdirAll(cacheDir, 0700)
	DieIf(err, "cannot create cache directory")
}

// cacheReadUnmarshal reads JSON and unmarshals into the target, returning true on success.
func cacheReadUnmarshal(file string, target interface{}) bool {

	if !noCache && cacheDir != "" {
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
	DieIf(err, "cannot marshal to JSON")

	// Cache the JSON
	if !noCache {
		file = filepath.Join(cacheDir, file)

		err = os.WriteFile(file, json, 0600)
		if err != nil {
			log.Printf("warning: %s: unable to write to cache", file)
		}
	}

	return json
}

// CacheGetAwsConfigOutput tries to read prior AWS creds from the cache.
func CacheGetAwsConfigOutput(cacheKey string) (creds *AwsConfigOutput) {
	var file string

	// Read credentials from the cache.
	if !noCache {
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

// CacheGetDuploOutput tries to read prior AWS creds from the cache.
func CacheGetDuploOutput(cacheKey string, host string) (creds *DuploCredsOutput) {
	var file string

	// Read credentials from the cache.
	if !noCache {
		file = fmt.Sprintf("%s,duplo-creds.json", cacheKey)
		creds = &DuploCredsOutput{}
		if !cacheReadUnmarshal(file, creds) {
			creds = nil
		}
	}

	// Check credentials for expiry - by trying to retrieve system features
	if creds != nil {

		// Retrieve system features.
		client, err := duplocloud.NewClient(host, creds.DuploToken)
		if err == nil {
			var features *duplocloud.DuploSystemFeatures
			features, err = client.FeaturesSystem()
			if features != nil {
				creds.NeedOTP = features.IsOtpNeeded
			}
		}

		// If we have any errors, assume that the credentials have expired
		if err != nil {
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
