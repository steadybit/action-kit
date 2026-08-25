package client

import (
	"net/http"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_json_validation(t *testing.T) {
	rClient := resty.New().SetBaseURL("http://localhost:8080")
	httpmock.ActivateNonDefault(rClient.GetClient())
	client := NewActionClient("/", rClient)

	tests := []struct {
		name     string
		response string
		wantErr  bool
	}{
		{
			name:     "missing id",
			response: `{}`,
			wantErr:  true,
		},
		{
			name: "valid",
			response: `{ 
"id": "test", 
"label" : "test-label",
"version" : "1.0.0", 
"description": "lorem ipsum",
"kind": "attack",
"timeControl": "internal", 
"parameters": [],
"prepare": { "method": "POST", "path": "/prepare" },
"start": { "method": "POST", "path": "/start" }
}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			httpmock.RegisterResponder("GET", "http://localhost:8080/test", httpmock.NewStringResponder(200, tt.response))
			_, err := client.DescribeAction(action_kit_api.DescribingEndpointReference{Path: "/test"})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// jsonResponse is needed because resty only unmarshals a response into the result when the
// content type says JSON, and httpmock's string responders set no content type.
func jsonResponse(body string) *http.Response {
	res := httpmock.NewStringResponse(200, body)
	res.Header.Set("Content-Type", "application/json")
	return res
}

func jsonResponder(body string) httpmock.Responder {
	return func(*http.Request) (*http.Response, error) {
		return jsonResponse(body), nil
	}
}

func Test_action_execution_collects_results(t *testing.T) {
	rClient := resty.New().SetBaseURL("http://localhost:8080")
	httpmock.ActivateNonDefault(rClient.GetClient())
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "http://localhost:8080/", jsonResponder(
		`{"actions": [{"method": "GET", "path": "/test"}]}`))
	httpmock.RegisterResponder("GET", "http://localhost:8080/test", jsonResponder(`{
"id": "test",
"label": "test-label",
"version": "1.0.0",
"description": "lorem ipsum",
"kind": "attack",
"timeControl": "internal",
"parameters": [],
"prepare": { "method": "POST", "path": "/prepare" },
"start": { "method": "POST", "path": "/start" },
"status": { "method": "POST", "path": "/status", "callInterval": "1ms" },
"stop": { "method": "POST", "path": "/stop" }
}`))
	httpmock.RegisterResponder("POST", "http://localhost:8080/prepare", jsonResponder(`{"state": {}}`))
	httpmock.RegisterResponder("POST", "http://localhost:8080/start", jsonResponder(`{"state": {}}`))

	// Two status rounds, so this also covers accumulation across polls rather than the last
	// response winning.
	var statusCalls int
	httpmock.RegisterResponder("POST", "http://localhost:8080/status", func(*http.Request) (*http.Response, error) {
		statusCalls++
		if statusCalls == 1 {
			return jsonResponse(`{
"completed": false,
"artifacts": [{"label": "status-1.tar.gz", "data": "Zmlyc3Q="}],
"metrics": [{"name": "status-1", "metric": {}, "timestamp": "2024-01-01T00:00:00Z", "value": 1}],
"messages": [{"message": "status-1"}]
}`), nil
		}
		return jsonResponse(`{
"completed": true,
"artifacts": [{"label": "status-2.tar.gz", "data": "c2Vjb25k"}]
}`), nil
	})
	httpmock.RegisterResponder("POST", "http://localhost:8080/stop", jsonResponder(`{
"artifacts": [{"label": "stop.tar.gz", "data": "c3RvcA=="}],
"metrics": [{"name": "stop", "metric": {}, "timestamp": "2024-01-01T00:00:00Z", "value": 2}],
"messages": [{"message": "stop"}]
}`))

	exec, err := NewActionClient("/", rClient).RunAction("test", nil, struct{}{}, nil)
	require.NoError(t, err)
	require.NoError(t, exec.Wait())

	artifacts := exec.Artifacts()
	require.Len(t, artifacts, 3, "artifacts from every status round and from stop")
	labels := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		labels = append(labels, artifact.Label)
	}
	assert.Equal(t, []string{"status-1.tar.gz", "status-2.tar.gz", "stop.tar.gz"}, labels)
	assert.Equal(t, "Zmlyc3Q=", artifacts[0].Data)

	// The collector carries metrics and messages down the same path, so guard those too.
	assert.Len(t, exec.Metrics(), 2)
	assert.Len(t, exec.Messages(), 2)
}
