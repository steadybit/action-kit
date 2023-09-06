package client

import (
	"github.com/go-resty/resty/v2"
	"github.com/jarcoal/httpmock"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/stretchr/testify/assert"
	"testing"
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
