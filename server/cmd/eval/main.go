package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"suppq.local/server/internal/evaluation"
)

func main() {
	input := flag.String("input", "", "private evaluation JSON file")
	output := flag.String("output", "", "optional report JSON file")
	allowSmall := flag.Bool("allow-small", false, "allow a non-production dataset for tool testing")
	flag.Parse()
	if *input == "" {
		fmt.Fprintln(os.Stderr, "-input is required")
		os.Exit(2)
	}
	data, err := os.ReadFile(*input)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	var dataset evaluation.Dataset
	if err = json.Unmarshal(data, &dataset); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	report, err := evaluation.Evaluate(dataset, evaluation.ProductionThresholds, *allowSmall)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	encoded, _ := json.MarshalIndent(report, "", "  ")
	encoded = append(encoded, '\n')
	if *output != "" {
		if err = os.WriteFile(*output, encoded, 0o600); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
	} else {
		_, _ = os.Stdout.Write(encoded)
	}
	if !report.Passed {
		os.Exit(1)
	}
}
