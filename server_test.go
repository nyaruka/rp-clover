package clover

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nyaruka/rp-clover/models"
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

func makeTestRequest(path string, method string, values url.Values, authenticate bool, assertStatus int, assertBody string) (err error) {
	url := "http://localhost:8081" + path
	var req *http.Request
	if values == nil {
		req, err = http.NewRequest(method, url, nil)
	} else {
		req, err = http.NewRequest(method, url, bytes.NewReader([]byte(values.Encode())))
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
		rBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		if resp.StatusCode != assertStatus {
			return fmt.Errorf("expected status: %d got %d: %s", assertStatus, resp.StatusCode, string(rBody))
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
		method       string
		body         url.Values
		authenticate bool
		responseCode int
		responseText string
	}{
		{"/", http.MethodGet, nil, false, 200, "Dev"},
		{"/admin", http.MethodGet, nil, false, 401, "Unauthorized"},
		{"/admin", http.MethodGet, nil, true, 200, "Clover Configuration"},
		{"/admin", http.MethodPost, url.Values{"config": []string{"arst"}}, true, 200, "invalid character"},
		{"/admin", http.MethodPost, url.Values{"config": []string{testConfig}}, true, 200, "configuration saved"},
		{"/admin", http.MethodPost, url.Values{"config": []string{"[]"}}, true, 200, "configuration saved"},
		{"/foo", http.MethodGet, nil, false, 404, "not found"},
	}

	for i, tc := range tcs {
		err := makeTestRequest(tc.path, tc.method, tc.body, tc.authenticate, tc.responseCode, tc.responseText)
		assert.NoErrorf(t, err, "test %d: error making request", i)
	}
}

func TestMapping(t *testing.T) {
	s := setUpTest(t)
	defer s.Stop()

	// set up our config, replacing our server with our test server
	err := makeTestRequest("/admin", http.MethodPost, url.Values{"config": []string{testConfig}}, true, 200, "configuration saved")
	assert.NoError(t, err)

	tcs := []struct {
		path              string
		method            string
		assertStatus      int
		assertText        string
		assertInterchange string
		assertURN         string
		assertChannelUUID string
	}{
		{"/admin/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/map", http.MethodDelete, 400, "missing", "5fb66333-7f8c-47aa-9aa5-bfee37b79b22", "tel:+12065551212", ""},
		{"/admin/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/map?urn=tel:%2B12065551212", http.MethodDelete, 200, "removed", "5fb66333-7f8c-47aa-9aa5-bfee37b79b22", "tel:+12065551212", ""},
		{"/admin/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/map?urn=tel:%2B12065551212&channel=557d3353-6b89-441a-aee5-8c398fd7a61f", http.MethodPost, 200, "created", "5fb66333-7f8c-47aa-9aa5-bfee37b79b22", "tel:+12065551212", "557d3353-6b89-441a-aee5-8c398fd7a61f"},
		{"/admin/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/map?urn=tel:%2B12065551212", http.MethodPost, 400, "channel not found", "5fb66333-7f8c-47aa-9aa5-bfee37b79b22", "tel:+12065551212", "557d3353-6b89-441a-aee5-8c398fd7a61f"},
		{"/admin/5fb66333-7f8c-47aa-9aa5-bfee37b79b22/map?urn=tel:%2B12065551212", http.MethodDelete, 200, "removed", "5fb66333-7f8c-47aa-9aa5-bfee37b79b22", "tel:+12065551212", ""},
	}

	for i, tc := range tcs {
		err := makeTestRequest(tc.path, tc.method, nil, true, tc.assertStatus, tc.assertText)
		assert.NoError(t, err, "test %d: error making request", i)

		interchange, err := models.GetInterchange(context.Background(), s.db, tc.assertInterchange)
		assert.NoError(t, err, "test %d: error looking up interchange", i)

		channel, err := models.GetChannelForURN(context.Background(), s.db, interchange, tc.assertURN)
		assert.NoError(t, err, "test %d: error looking up channel", i)

		if tc.assertChannelUUID == "" {
			assert.Nil(t, channel)
		} else {
			assert.Equal(t, tc.assertChannelUUID, channel.UUID)
		}
	}
}
