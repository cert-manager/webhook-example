package cmd

import (
	"flag"
	"os"
	"runtime"

	"github.com/cert-manager/cert-manager/cmd/util"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	logf "github.com/cert-manager/cert-manager/pkg/logs"
	"github.com/pluralsh/plural-certmanager-webhook/pkg/server"
	"k8s.io/component-base/logs"
)

// RunWebhookServer creates and starts a new apiserver that acts as a external
// webhook server for solving DNS challenges using the provided solver
// implementations. This can be used as an entry point by external webhook
// implementations, see
// https://github.com/cert-manager/webhook-example/blob/899c408751425f8d0842b61c0e62fd8035d00316/main.go#L23-L31
func RunWebhookServer(groupName string, hooks ...webhook.Solver) {
	stopCh, exit := util.SetupExitHandler(util.GracefulShutdown)
	defer exit() // This function might call os.Exit, so defer last

	logs.InitLogs()
	defer logs.FlushLogs()

	if len(os.Getenv("GOMAXPROCS")) == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	}

	cmd := server.NewCommandStartWebhookServer(os.Stdout, os.Stderr, stopCh, groupName, hooks...)
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	if err := cmd.Execute(); err != nil {
		logf.Log.Error(err, "error executing command")
		util.SetExitCode(err)
	}
}
