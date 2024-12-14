package main

import (
	"flag"
	"k8s-nodes-example/cmd"
	"strings"
)

func main() {
	config := parseFlags()

	app, err := cmd.NewApp(config)
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}

// parseFlags parses command line flags and returns a Config
func parseFlags() *cmd.Config {
	var namespaces []string
	var useMockData bool
	var logFilePath string

	flag.Var((*cmd.ArrayFlags)(&namespaces), "N", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.Var((*cmd.ArrayFlags)(&namespaces), "namespace", "Filter by namespace (can be specified multiple times or comma-separated, prefix with - to exclude)")
	flag.BoolVar(&useMockData, "mock-k8s-data", false, "Use mock Kubernetes data instead of real cluster")
	flag.StringVar(&logFilePath, "logfile", "", "Path to file for logging changes")
	flag.Parse()

	// Create maps for included and excluded namespaces
	includeNamespaces := make(map[string]bool)
	excludeNamespaces := make(map[string]bool)

	for _, ns := range namespaces {
		if strings.HasPrefix(ns, "-") {
			excludeNamespaces[strings.TrimPrefix(ns, "-")] = true
		} else {
			includeNamespaces[ns] = true
		}
	}

	return &cmd.Config{
		IncludeNamespaces: includeNamespaces,
		ExcludeNamespaces: excludeNamespaces,
		UseMockData:       useMockData,
		LogFilePath:       logFilePath,
	}
}
