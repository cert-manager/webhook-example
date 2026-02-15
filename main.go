package main

import (
	"os"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	"github.com/proton11/cert-manager-desec-webhook/solver"
)

// Entrypoint of the application
func main() {

	// Read the custom group name from environment variables
	groupName, ok := os.LookupEnv("GROUP_NAME")
	// Without a custom group name, return the default (also defined in the Helm chart)
	if !ok || groupName == "" {
		groupName = "acme.pr0ton11.github.com"
	}
	// Start the webhook server with our solver
	cmd.RunWebhookServer(groupName, &solver.DeSECDNSProviderSolver{})
}
