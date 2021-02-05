// Package endpoint implements functionality for simple monitoring of
// HTTP endpoints on whether or not they return a 200 response
package endpoint

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Endpoint supports being read-in/unmarshaled from a YAML list
// Timestamp is the time at which the Endpoint was checked for up/down
// Name is purely informational and may be useful when alerting
// URL is the URL to send HTTP GET requests to
// Healthy is the current status of the Endpoint from an HTTP GET request
// Err allows errors to be passed along in the channel
type Endpoint struct {
	Timestamp time.Time
	Name      string `json:"name"`
	URL       string `json:"url"`
	Healthy   bool   `json:"healthy,omitempty"`
	Err       error
}

// ReadEndpoints
func ReadEndpoints(e []byte) ([]Endpoint, error) {
	var es []Endpoint

	err := json.Unmarshal(e, &es)
	return es, err
}

// Generator takes in a variable number of endpoints and converts them into a channel
// following the "pipeline" architecture. This is an entrypoint function to the rest
// of the functions processing Endpoints and can be interrupted/will trigger downstream
// functions to stop processing with the done channel.
func Generator(done <-chan interface{}, endpoints ...Endpoint) <-chan Endpoint {
	epStream := make(chan Endpoint)
	go func() {
		defer close(epStream)
		for _, ep := range endpoints {
			select {
			case <-done:
				return
			case epStream <- ep:
			}
		}
	}()
	return epStream
}

// CheckIsDown takes in a channel of Endpoints and checks if each Endpoint is down
// using an HTTP GET request and stamps it with a timestamp.
func CheckIsDown(done <-chan interface{}, epStream <-chan Endpoint) <-chan Endpoint {
	checkedStream := make(chan Endpoint)
	go func() {
		defer close(checkedStream)

		for ep := range epStream {
			// Timestamp with current local time
			ep.Timestamp = time.Now()

			// Handle x509: certificate signed by unknown authority
			tr := &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			client := &http.Client{Transport: tr}
			res, err := client.Get(ep.URL)
			if err != nil {
				ep.Err = err
				ep.Healthy = false
			} else {
				// We do not want to close the response body if there is an error above
				defer res.Body.Close()
				// If 200 or 204
				if res.StatusCode == http.StatusOK || res.StatusCode == http.StatusNoContent {
					ep.Err = nil
					ep.Healthy = true
				} else {
					ep.Err = fmt.Errorf("%v %v", res.Status, res.Proto)
					ep.Healthy = false
				}
			}

			select {
			case <-done:
				return
			case checkedStream <- ep:
			}
		}
	}()
	return checkedStream
}
