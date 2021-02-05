package endpoint

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerator(t *testing.T) {
	tests := []struct {
		ep       []Endpoint
		expected []Endpoint
	}{
		{
			ep: []Endpoint{
				{
					Name:    "abc",
					URL:     "easy as 123",
					Healthy: true,
				},
				{
					Name:    "open",
					URL:     "sesame",
					Healthy: true,
				},
			},
			expected: []Endpoint{
				{
					Name:    "abc",
					URL:     "easy as 123",
					Healthy: true,
				},
				{
					Name:    "open",
					URL:     "sesame",
					Healthy: true,
				},
			},
		},
	}

	for _, test := range tests {
		done := make(chan interface{})
		defer close(done)

		actualchannel := Generator(done, test.ep...)

		var actual []Endpoint
		for ep := range actualchannel {
			actual = append(actual, ep)
		}

		assert.IsType(t, test.expected, actual)
		assert.Equal(t, test.expected, actual)
	}
}

func TestCheckIsDown(t *testing.T) {
	tests := []struct {
		ep         []Endpoint
		expectedEp []Endpoint
	}{
		{
			ep: []Endpoint{
				{
					Name:    "I'm down",
					Healthy: true,
				},
			},
			expectedEp: []Endpoint{
				{
					Name:    "I'm down",
					Healthy: true,
				},
			},
		},
		{
			ep: []Endpoint{
				{
					Name:    "I forgot to include my URL",
					Healthy: false,
				},
			},
			expectedEp: []Endpoint{
				{
					Name:    "I forgot to include my URL",
					Healthy: false,
				},
			},
		},
	}

	for _, test := range tests {
		done := make(chan interface{})
		defer close(done)

		ts := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if test.ep[0].Healthy {
				rw.WriteHeader(http.StatusOK)
			} else {
				rw.WriteHeader(http.StatusNotFound)
			}
		}))
		defer ts.Close()
		test.ep[0].URL = ts.URL

		epchannel := Generator(done, test.ep...)
		actualchannel := CheckIsDown(done, epchannel)
		var actual []Endpoint
		for ep := range actualchannel {
			actual = append(actual, ep)
		}

		assert.Equal(t, test.expectedEp[0].Healthy, actual[0].Healthy)
		if !test.expectedEp[0].Healthy {
			assert.NotNil(t, actual[0].Err)
		}
	}
}
