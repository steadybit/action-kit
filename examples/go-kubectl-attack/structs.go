package main

type DescribingEndpointRef struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type AttackListResponse struct {
	Attacks []DescribingEndpointRef `json:"name"`
}
