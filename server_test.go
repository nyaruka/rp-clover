package clover

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func setUpTest(t *testing.T) *Server {
	config := NewConfig()
	server := NewServer(config, http.Dir("static"))

	err := server.Start()
	if err != nil {
		t.Fatalf("error starting server: %s", err)
	}
	time.Sleep(10 * time.Millisecond)
	return server
}

func makeTestRequest(path string, values url.Values, authenticate bool, assertStatus int, assertBody string) (err error) {
	url := "http://localhost:8081" + path
	var req *http.Request
	if values == nil {
		req, err = http.NewRequest(http.MethodGet, url, nil)
	} else {
		req, err = http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(values.Encode())))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	if err != nil {
		return err
	}

	if authenticate {
		req.SetBasicAuth("admin", "sesame123")
	}

	resp, err := http.DefaultClient.Do(req)

	if err == nil {
		if resp.StatusCode != assertStatus {
			return fmt.Errorf("expected status: %d got %d", assertStatus, resp.StatusCode)
		}
		rBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if !strings.Contains(string(rBody), assertBody) {
			return fmt.Errorf("did not find: '%s' in response body: '%s'", assertBody, string(rBody))
		}
	}

	return err
}

const testConfig = `
[
	{
		"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
		"name": "Nigeria",
		"country": "NE",
		"scheme": "tel",
		"channels": [
			{
				"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
				"name": "U-Report Nigeria",
				"url": "https://foobar",
				"keywords": [
					"one"
				]
			}
		]
	}
]`

func TestEndpoints(t *testing.T) {
	s := setUpTest(t)
	defer s.Stop()

	tcs := []struct {
		path         string
		body         url.Values
		authenticate bool
		responseCode int
		responseText string
	}{
		{"/", nil, false, 200, "Dev"},
		{"/admin", nil, false, 401, "Unauthorized"},
		{"/admin", nil, true, 200, "Clover Configuration"},
		{"/admin", url.Values{"config": []string{"arst"}}, true, 200, "invalid character"},
		{"/admin", url.Values{"config": []string{testConfig}}, true, 200, "configuration saved"},
		{"/admin", url.Values{"config": []string{"[]"}}, true, 200, "configuration saved"},
		{"/foo", nil, false, 404, "not found"},
	}

	for i, tc := range tcs {
		err := makeTestRequest(tc.path, tc.body, tc.authenticate, tc.responseCode, tc.responseText)
		assert.NoErrorf(t, err, "test %d: error making request", i)
	}
}
