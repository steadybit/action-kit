package main

type EndpointRef struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type ErrorResponse struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
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
	Label       string            `json:"label"`
	Description string            `json:"description"`
	Version     string            `json:"version"`
	Category    string            `json:"category"`
	Target      string            `json:"target"`
	TimeControl string            `json:"timeControl"`
	Parameters  []AttackParameter `json:"parameters"`
	Prepare     EndpointRef       `json:"prepare"`
	Start       EndpointRef       `json:"start"`
	Status      EndpointRef       `json:"status"`
	Stop        EndpointRef       `json:"stop"`
}

type Target struct {
	Name       string              `json:"name"`
	Attributes map[string][]string `json:"attributes"`
}

type PrepareAttackRequest struct {
	Config map[string]interface{} `json:"config"`
	Target Target                 `json:"target"`
}

type RolloutRestartState struct {
	Cluster    string
	Namespace  string
	Deployment string
	Wait       bool
}

type PrepareAttackResponse struct {
	State RolloutRestartState `json:"state"`
}

type StartAttackRequest struct {
	State RolloutRestartState `json:"state"`
}

type StartAttackResponse struct {
	State RolloutRestartState `json:"state"`
}

type AttackStatusRequest struct {
	State RolloutRestartState `json:"state"`
}

type AttackStatusResponse struct {
	Completed bool `json:"completed"`
}
