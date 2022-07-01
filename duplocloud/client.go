package duplocloud

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Represents a connection to the Duplo API
type Client struct {
	HTTPClient *http.Client
	HostURL    string
	Token      string
	OTP        string
}

// NewClient creates a new Duplo API client
func NewClient(host, token string) (*Client, error) {
	if host != "" && token != "" {
		c := Client{
			HTTPClient: &http.Client{Timeout: 20 * time.Second},
			HostURL:    host,
			Token:      token,
		}
		return &c, nil
	}
	return nil, fmt.Errorf("missing config for Duplo 'host' and/or 'token'")
}

// NewClientWithOtp creates a new Duplo API client with an OTP code
func NewClientWithOtp(host, token, otp string) (c *Client, err error) {
	c, err = NewClient(host, token)
	if err == nil && c != nil && otp != "" {
		c.OTP = otp
	}
	return
}

type clientError struct {
	message  string
	status   int
	url      string
	response map[string]interface{}
}

func (e clientError) Error() string {
	return e.message
}

func (e clientError) Status() int {
	return e.status
}

func (e clientError) PossibleMissingAPI() bool {
	return e.status == 500 || e.status == 404
}

func (e clientError) URL() string {
	return e.url
}

func (e clientError) Response() map[string]interface{} {
	return e.response
}

// Represents an error from an API call.
type ClientError interface {
	Error() string
	Status() int
	PossibleMissingAPI() bool
	URL() string
	Response() map[string]interface{}
}

func newHttpError(req *http.Request, status int, message string) ClientError {
	response := map[string]interface{}{"Message": message}
	return clientError{status: status, url: req.URL.String(), message: message, response: response}
}

// An application logic error encountered in spite of a semantically correct response.
func appHttpError(req *http.Request, message string) ClientError {
	return newHttpError(req, -1, message)
}

// An error encountered in the HTTP response.
func responseHttpError(req *http.Request, res *http.Response) ClientError {
	status := res.StatusCode
	url := req.URL.String()
	response := map[string]interface{}{}

	// Read the body, but tolerate a failure.
	defer res.Body.Close()
	bytes, err := ioutil.ReadAll(res.Body)
	message := "(read of body failed)"
	if err == nil {
		message = string(bytes)
	}

	// Older APIs do not always return helpful errors to API clients.
	if !strings.HasPrefix(req.URL.Path, "/v3/") && (status == 400 || status == 404) {
		message = fmt.Sprintf("%s. Please verify your Duplo connection information.", message)
	}

	// Handle APIs that return proper JSON
	mime := strings.SplitN(res.Header.Get("content-type"), ";", 2)[0]
	if mime == "application/json" {
		err = json.Unmarshal(bytes, &response)
		if err != nil {
			logf(ERROR, "duplo-responseHttpError: failed to parse error response JSON: %s, %s", err, string(bytes))
		}
	}

	// Build the final error message.
	message = fmt.Sprintf("url: %s, status: %d, message: %s", url, status, message)
	logf(WARN, "[WARN] duplo-responseHttpError: %s", message)

	// Handle responses that are missing a message - or a JSON parse failure
	if _, ok := response["Message"]; !ok {
		response["Message"] = message
	}

	return clientError{status: res.StatusCode, url: url, message: message, response: response}
}

// An error encountered before we could parse the response.
func ioHttpError(req *http.Request, err error) ClientError {
	return newHttpError(req, -1, err.Error())
}

func (c *Client) doRequestWithStatus(req *http.Request, expectedStatus int) ([]byte, ClientError) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.Token))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	res, err := c.HTTPClient.Do(req)

	// Handle I/O errors
	if err != nil {
		return nil, ioHttpError(req, err)
	}

	// Pass through HTTP errors, unexpected redirects, or unexpected status codes.
	if res.StatusCode > 300 || (expectedStatus > 0 && expectedStatus != res.StatusCode) {
		return nil, responseHttpError(req, res)
	}

	// Otherwise, we have a response that needs reading.
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		logf(WARN, "duplo-doRequestWithStatus: %s", err)
		return nil, ioHttpError(req, err)
	}

	return body, nil
}

func (c *Client) doRequest(req *http.Request) ([]byte, ClientError) {
	return c.doRequestWithStatus(req, 0)
}

// Utility method to call an API without a request body, handling logging, etc.
func (c *Client) doAPI(verb string, apiName string, apiPath string, rp interface{}) ClientError {
	apiName = fmt.Sprintf("%sAPI %s", strings.ToLower(verb), apiName)

	// Build the request
	url := fmt.Sprintf("%s/%s", c.HostURL, apiPath)
	logf(TRACE, "%s: prepared request: %s", apiName, url)
	req, err := http.NewRequest(verb, url, nil)
	if err != nil {
		logf(TRACE, "%s: cannot build request: %s", apiName, err.Error())
		return nil
	}
	if c.OTP != "" {
		req.Header.Set("otpcode", c.OTP)
	}

	// Call the API and get the response.
	body, httpErr := c.doRequest(req)
	if httpErr != nil {
		logf(TRACE, "%s: failed: %s", apiName, httpErr.Error())
		return httpErr
	}
	bodyString := string(body)
	logf(TRACE, "%s: received response: %s", apiName, bodyString)

	// Check for an expected "null" response.
	if rp == nil {
		logf(TRACE, "%s: expected null response", apiName)
		if bodyString == "null" || bodyString == "" {
			return nil
		}
		message := fmt.Sprintf("%s: received unexpected response: %s", apiName, bodyString)
		logf(TRACE, message)
		return appHttpError(req, message)
	}

	// Otherwise, interpret it as an object.
	err = json.Unmarshal(body, rp)
	if err != nil {
		message := fmt.Sprintf("%s: cannot unmarshal response from JSON: %s", apiName, err.Error())
		logf(TRACE, message)
		return newHttpError(req, -1, message)
	}
	return nil
}

// Utility method to call an API with a GET request, handling logging, etc.
func (c *Client) getAPI(apiName string, apiPath string, rp interface{}) ClientError {
	return c.doAPI("GET", apiName, apiPath, rp)
}
