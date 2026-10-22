// Command covprog is built with -cover by TestCoverage_accumulates_runs: each run covers one branch.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "first" {
		fmt.Println("first branch")
	} else {
		fmt.Println("second branch")
	}
}
