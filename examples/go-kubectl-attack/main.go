package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func attackListHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	attackList := AttackListResponse{
		Attacks: []EndpointRef{
			{
				"GET",
				"/attacks/rollout-restart",
			},
		},
	}

	json.NewEncoder(w).Encode(attackList)
}

func describeRolloutRestartHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	attackList := DescribeAttackResponse{
		Id:          "com.steadybit.example.attacks.kubernetes.rollout-restart",
		Name:        "Kubernetes Rollout Restart Deployment",
		Description: "Execute a rollout restart for a Kubernetes deployment",
		Version:     "1.0.0",
		Target:      "kubernetes-deployment",
		TimeControl: "ONE_SHOT",
		Parameters:  []AttackParameter{},
		Prepare: EndpointRef{
			"POST",
			"/attacks/rollout-restart/prepare",
		},
		Start: EndpointRef{
			"POST",
			"/attacks/rollout-restart/start",
		},
		Stop: EndpointRef{
			"POST",
			"/attacks/rollout-restart/stop",
		},
	}

	json.NewEncoder(w).Encode(attackList)
}

func main() {
	http.HandleFunc("/attacks", attackListHandler)
	http.HandleFunc("/attacks/rollout-restart", describeRolloutRestartHandler)

	port := 8083
	fmt.Printf("Starting kubectl attack server on port %d. Get started via /attacks", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
