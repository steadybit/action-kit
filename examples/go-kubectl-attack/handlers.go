package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
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
		Name:        "Kubernetes Rollout Restart Deployment",
		Description: "Execute a rollout restart for a Kubernetes deployment",
		Version:     "1.0.1",
		Category:    "resource",
		Target:      "kubernetes-deployment",
		TimeControl: "ONE_SHOT",
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
		Stop: EndpointRef{
			"POST",
			"/attacks/rollout-restart/stop",
		},
	})
}

func prepareRolloutRestart(w http.ResponseWriter, _ *http.Request, body []byte) {
	var prepareAttackRequest PrepareAttackRequest
	err := json.Unmarshal(body, &prepareAttackRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
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
	var startAttackRequest StartAttackRequest
	err := json.Unmarshal(body, &startAttackRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, cmdErr.Error(), http.StatusInternalServerError)
		return
	}

	if startAttackRequest.State.Wait {
		cmd := exec.Command("kubectl",
			"rollout",
			"status",
			"--namespace",
			startAttackRequest.State.Namespace,
			fmt.Sprintf("deployment/%s", startAttackRequest.State.Deployment))
		cmdOut, cmdErr := cmd.CombinedOutput()
		if cmdErr != nil {
			ErrorLogger.Printf("Failed to watch rollout status %s: %s", cmdErr, cmdOut)
			http.Error(w, cmdErr.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StartAttackResponse{
		State: startAttackRequest.State,
	})
}

func stopRolloutRestart(w http.ResponseWriter, req *http.Request, body []byte) {
	InfoLogger.Printf("noop\n")
}
