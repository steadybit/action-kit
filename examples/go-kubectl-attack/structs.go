package main

type EndpointRef struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type AttackListResponse struct {
	Attacks []EndpointRef `json:"attacks"`
}

type AttackParameter struct {
	Label        string `json:"label"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Description  string `json:"description"`
	Required     bool   `json:"required"`
	Advanced     bool   `json:"advanced"`
	Order        int    `json:"order"`
	DefaultValue string `json:"defaultValue"`
}

type DescribeAttackResponse struct {
	Id          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Target      string            `json:"target"`
	TimeControl string            `json:"timeControl"`
	Parameters  []AttackParameter `json:"parameters"`
	Prepare     EndpointRef       `json:"prepare"`
	Start       EndpointRef       `json:"start"`
	Stop        EndpointRef       `json:"stop"`
}
