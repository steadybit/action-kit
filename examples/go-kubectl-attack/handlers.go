package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

func getAttackList(w http.ResponseWriter, _ *http.Request, _ []byte) {
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

func getRolloutRestartDescription(w http.ResponseWriter, _ *http.Request, _ []byte) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DescribeAttackResponse{
		Id:          "com.steadybit.example.attacks.kubernetes.rollout-restart",
		Label:       "Kubernetes Rollout Restart Deployment",
		Description: "Execute a rollout restart for a Kubernetes deployment",
		Version:     "1.0.2",
		Category:    "resource",
		Target:      "kubernetes-deployment",
		TimeControl: "INTERNAL",
		Parameters: []AttackParameter{
			{
				Label:        "Wait for rollout completion",
				Name:         "wait",
				Type:         "boolean",
				Advanced:     true,
				DefaultValue: "false",
			},
		},
		Prepare: EndpointRef{
			"POST",
			"/attacks/rollout-restart/prepare",
		},
		Start: EndpointRef{
			"POST",
			"/attacks/rollout-restart/start",
		},
		State: EndpointRef{
			"POST",
			"/attacks/rollout-restart/state",
		},
		Stop: EndpointRef{
			"POST",
			"/attacks/rollout-restart/stop",
		},
	})
}

func prepareRolloutRestart(w http.ResponseWriter, _ *http.Request, body []byte) {
	w.Header().Set("Content-Type", "application/json")

	var prepareAttackRequest PrepareAttackRequest
	err := json.Unmarshal(body, &prepareAttackRequest)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ErrorResponse{
			Title:  "Failed to read request body",
			Detail: err.Error(),
		})
		return
	}

	wait := false
	if prepareAttackRequest.Config["wait"] != nil {
		wait = prepareAttackRequest.Config["wait"].(bool)
	}
	json.NewEncoder(w).Encode(PrepareAttackResponse{
		State: RolloutRestartState{
			Cluster:    prepareAttackRequest.Target.Attributes["k8s.cluster-name"][0],
			Namespace:  prepareAttackRequest.Target.Attributes["k8s.namespace"][0],
			Deployment: prepareAttackRequest.Target.Attributes["k8s.deployment"][0],
			Wait:       wait,
		},
	})
}

func startRolloutRestart(w http.ResponseWriter, _ *http.Request, body []byte) {
	w.Header().Set("Content-Type", "application/json")

	var startAttackRequest StartAttackRequest
	err := json.Unmarshal(body, &startAttackRequest)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ErrorResponse{
			Title:  "Failed to read request body",
			Detail: err.Error(),
		})
		return
	}

	InfoLogger.Printf("Starting rollout restart attack for %s\n", startAttackRequest)

	cmd := exec.Command("kubectl",
		"rollout",
		"restart",
		"--namespace",
		startAttackRequest.State.Namespace,
		fmt.Sprintf("deployment/%s", startAttackRequest.State.Deployment))
	cmdOut, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		ErrorLogger.Printf("Failed to execute rollout restart %s: %s", cmdErr, cmdOut)
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ErrorResponse{
			Title:  fmt.Sprintf("Failed to execute rollout restart %s: %s", cmdErr, cmdOut),
			Detail: cmdErr.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(StartAttackResponse{
		State: startAttackRequest.State,
	})
}

func rolloutRestartState(w http.ResponseWriter, _ *http.Request, body []byte) {
	w.Header().Set("Content-Type", "application/json")

	var attackStateRequest AttackStateRequest
	err := json.Unmarshal(body, &attackStateRequest)
	if err != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ErrorResponse{
			Title:  "Failed to read request body",
			Detail: err.Error(),
		})
		return
	}

	InfoLogger.Printf("Checking rollout restart attack state for %s\n", attackStateRequest)

	if !attackStateRequest.State.Wait {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(AttackStateResponse{
			true,
		})
		return
	}

	cmd := exec.Command("kubectl",
		"rollout",
		"status",
		"--watch=false",
		"--namespace",
		attackStateRequest.State.Namespace,
		fmt.Sprintf("deployment/%s", attackStateRequest.State.Deployment))
	cmdOut, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		w.WriteHeader(500)
		json.NewEncoder(w).Encode(ErrorResponse{
			Title:  fmt.Sprintf("Failed to check rollout status %s: %s", cmdErr, cmdOut),
			Detail: cmdErr.Error(),
		})
		return
	}

	cmdOutStr := string(cmdOut)
	completed := !strings.Contains(strings.ToLower(cmdOutStr), "waiting")
	json.NewEncoder(w).Encode(AttackStateResponse{
		completed,
	})
}

func stopRolloutRestart(_ http.ResponseWriter, _ *http.Request, _ []byte) {
}
