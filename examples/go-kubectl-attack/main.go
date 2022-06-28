package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

func init() {
	InfoLogger = log.New(os.Stderr, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(os.Stderr, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

func getAttackList(w http.ResponseWriter, req *http.Request) {
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

func getRolloutRestartDescription(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(DescribeAttackResponse{
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
	})
}

func prepareRolloutRestart(w http.ResponseWriter, req *http.Request) {
	var prepareAttackRequest PrepareAttackRequest
	err := json.NewDecoder(req.Body).Decode(&prepareAttackRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PrepareAttackResponse{
		State: RolloutRestartState{
			Cluster:    prepareAttackRequest.Target.Attributes["k8s.cluster-name"][0],
			Namespace:  prepareAttackRequest.Target.Attributes["k8s.deployment"][0],
			Deployment: prepareAttackRequest.Target.Attributes["k8s.deployment"][0],
		},
	})
}

func startRolloutRestart(w http.ResponseWriter, req *http.Request) {
	var startAttackRequest StartAttackRequest
	err := json.NewDecoder(req.Body).Decode(&startAttackRequest)
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(StartAttackResponse{
		State: startAttackRequest.State,
	})
}

func stopRolloutRestart(w http.ResponseWriter, req *http.Request) {
	InfoLogger.Printf("noop\n")
}

func main() {
	http.HandleFunc("/attacks", getAttackList)
	http.HandleFunc("/attacks/rollout-restart", getRolloutRestartDescription)
	http.HandleFunc("/attacks/rollout-restart/prepare", prepareRolloutRestart)
	http.HandleFunc("/attacks/rollout-restart/start", startRolloutRestart)
	http.HandleFunc("/attacks/rollout-restart/stop", stopRolloutRestart)

	port := 8083
	InfoLogger.Printf("Starting kubectl attack server on port %d. Get started via /attacks\n", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
