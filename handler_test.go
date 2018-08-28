package clover

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const handlerConfig = `
[
	{
		"uuid": "5fb66333-7f8c-47aa-9aa5-bfee37b79b22",
		"name": "Nigeria",
		"country": "NE",
		"scheme": "tel",
		"channels": [
			{
				"uuid": "557d3353-6b89-441a-aee5-8c398fd7a61f",
				"name": "Handler1",
				"url": "https://handler1",
				"keywords": [
					"one"
				]
			},
			{
				"uuid": "3d0cd397-2228-4185-86db-7e3272fc423e",
				"name": "Handler2",
				"url": "https://handler2",
				"keywords": [
					"two"
				]
			}
		]
	}
]`

func TestHandler(t *testing.T) {
	s := setUpTest(t)
	defer s.Stop()

	tsBody := ""
	tsStatus := 200
	var tsReq *http.Request

	// start our test server
	server := httptest.NewServer(http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		tsReq = req
		resp.WriteHeader(tsStatus)
		resp.Write([]byte(tsBody))
	}))
	defer server.Close()

	// set up our config, replacing our server with our test server
	config := strings.Replace(handlerConfig, "https://handler1", server.URL+"/handler1", -1)
	config = strings.Replace(config, "https://handler2", server.URL+"/handler2", -1)
	err := makeTestRequest("/admin", http.MethodPost, url.Values{"config": []string{config}}, true, 200, "configuration saved")
	assert.NoError(t, err)

	tcs := []struct {
		path           string
		values         url.Values
		responseStatus int
		responseText   string
		assertPath     string
		assertStatus   int
		assertText     string
	}{
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive", nil, 0, "", "", 400, "missing sender"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b11/receive", nil, 0, "", "", 404, "interchange not found"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=test", nil, 200, "handled", "/handler1?sender=2065551212&message=test", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=test&other=foo", nil, 200, "handled", "/handler1?sender=2065551212&message=test&other=foo", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=test", nil, 400, "downstream error", "/handler1?sender=2065551212&message=test", 400, "downstream error"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=test", nil, 500, "downstream server error", "/handler1?sender=2065551212&message=test", 500, "downstream server error"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive", url.Values{"sender": []string{"2065551212"}, "message": []string{"hello"}}, 200, "handled post", "/handler1", 200, "handled post"},

		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=TWO", nil, 200, "handled", "/handler2?sender=2065551212&message=TWO", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=other", nil, 200, "handled", "/handler2?sender=2065551212&message=other", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551213&message=other", nil, 200, "handled", "/handler1?sender=2065551213&message=other", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=one", nil, 200, "handled", "/handler1?sender=2065551212&message=one", 200, "handled"},
		{"/i/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/receive?sender=2065551212&message=other", nil, 200, "handled", "/handler1?sender=2065551212&message=other", 200, "handled"},
	}

	for i, tc := range tcs {
		// set our response values
		tsStatus = tc.responseStatus
		tsBody = tc.responseText
		tsReq = nil

		url := "http://localhost:8081" + tc.path
		var req *http.Request
		if tc.values == nil {
			req, err = http.NewRequest(http.MethodGet, url, nil)
		} else {
			req, err = http.NewRequest(http.MethodPost, url, bytes.NewReader([]byte(tc.values.Encode())))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		assert.NoError(t, err, "test %d: error building request", i)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err, "test %d: error making request", i)

		if err == nil {
			assert.Equal(t, tc.assertStatus, resp.StatusCode, "test %d: mismatched status", i)
			rBody, err := ioutil.ReadAll(resp.Body)
			assert.NoError(t, err, "test %d: error reading body", i)

			if !strings.Contains(string(rBody), tc.assertText) {
				assert.Failf(t, "test %d: did not get expected text", "%s not in %s", i, tc.assertText, string(rBody))
			}

			if tsReq != nil {
				assert.Equal(t, server.URL+tc.assertPath, "http://"+tsReq.Host+tsReq.URL.String(), "test %d: mismatched URL")
			}

			if tc.values != nil {
				assert.Equal(t, http.MethodPost, tsReq.Method, "test %d: was not a post", i)
				assert.Equal(t, tc.values, tsReq.Form, "test %d: did not pass through values", i)
			}
		}
	}
}
