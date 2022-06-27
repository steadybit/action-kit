package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func attackListHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	attackList := AttackListResponse{
		Attacks: []DescribingEndpointRef{
			{
				"GET",
				"/attacks/rollout-restart",
			},
		},
	}

	json.NewEncoder(w).Encode(attackList)
}

func rolloutRestartHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	attackList := AttackListResponse{
		Attacks: []DescribingEndpointRef{
			{
				"GET",
				"/attacks/rollout-restart",
			},
		},
	}

	json.NewEncoder(w).Encode(attackList)
}

func main() {
	http.HandleFunc("/attacks", attackListHandler)
	http.HandleFunc("/attacks/rollout-restart", rolloutRestartHandler)

	port := 8083
	fmt.Printf("Starting kubectl attack server on port %d", port)
	http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
