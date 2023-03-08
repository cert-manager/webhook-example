package server

import (
	"fmt"
	"io"
	"net"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook"
	whapi "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apiserver"
	"github.com/pluralsh/plural-certmanager-webhook/pkg/api/generated/openapi"
	"github.com/spf13/cobra"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

const defaultEtcdPathPrefix = "/registry/acme.cert-manager.io"

type WebhookServerOptions struct {
	RecommendedOptions *genericoptions.RecommendedOptions

	SolverGroup string
	Solvers     []webhook.Solver

	StdOut io.Writer
	StdErr io.Writer
}

func NewWebhookServerOptions(out, errOut io.Writer, groupName string, solvers ...webhook.Solver) *WebhookServerOptions {
	o := &WebhookServerOptions{
		// TODO we will nil out the etcd storage options.  This requires a later level of k8s.io/apiserver
		RecommendedOptions: genericoptions.NewRecommendedOptions(
			defaultEtcdPathPrefix,
			apiserver.Codecs.LegacyCodec(whapi.SchemeGroupVersion),
		),

		SolverGroup: groupName,
		Solvers:     solvers,

		StdOut: out,
		StdErr: errOut,
	}
	o.RecommendedOptions.Etcd = nil
	o.RecommendedOptions.Admission = nil

	return o
}

func NewCommandStartWebhookServer(out, errOut io.Writer, stopCh <-chan struct{}, groupName string, solvers ...webhook.Solver) *cobra.Command {
	o := NewWebhookServerOptions(out, errOut, groupName, solvers...)

	cmd := &cobra.Command{
		Short: "Launch an ACME solver API server",
		Long:  "Launch an ACME solver API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			if err := o.RunWebhookServer(stopCh); err != nil {
				return err
			}
			return nil
		},
	}

	flags := cmd.Flags()
	o.RecommendedOptions.AddFlags(flags)

	return cmd
}

func (o WebhookServerOptions) Validate(args []string) error {
	return nil
}

func (o *WebhookServerOptions) Complete() error {
	return nil
}

// Config creates a new webhook server config that includes generic upstream
// apiserver options, rest client config and the Solvers configured for this
// webhook server
func (o WebhookServerOptions) Config() (*apiserver.Config, error) {
	// TODO have a "real" external address
	if err := o.RecommendedOptions.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(apiserver.Codecs)
	if err := o.RecommendedOptions.ApplyTo(serverConfig); err != nil {
		return nil, err
	}
	serverConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(openapi.GetOpenAPIDefinitions, openapinamer.NewDefinitionNamer(scheme))

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig: apiserver.ExtraConfig{
			SolverGroup: o.SolverGroup,
			Solvers:     o.Solvers,
		},
	}
	return config, nil
}

// RunWebhookServer creates a new apiserver, registers an API Group for each of
// the configured solvers and runs the new apiserver.
func (o WebhookServerOptions) RunWebhookServer(stopCh <-chan struct{}) error {
	config, err := o.Config()
	if err != nil {
		return err
	}

	server, err := config.Complete().New()
	if err != nil {
		return err
	}

	return server.GenericAPIServer.PrepareRun().Run(stopCh)
}
