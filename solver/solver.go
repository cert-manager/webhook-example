package solver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	acme "github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	v1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/cert-manager/cert-manager/pkg/issuer/acme/dns/util"
	"github.com/nrdcg/desec"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Configuration for the DeSEC DNS-01 challenge solver
type DeSECDNSProviderSolverConfig struct {
	// Reference to the kubernetes secret containing the API token for deSEC
	APIKeySecretRef v1.SecretKeySelector `json:"apiKeySecretRef"`
	// A global namespace (e.g APIKeySecretRefNamespace is not required, because ClusterIssuer provides the cert-manager namespace as default value for global issuers)
}

// A DNS-01 challenge solver for the DeSEC DNS Provider
type DeSECDNSProviderSolver struct {
	// Client to communicate with the kubernetes API
	k8s *kubernetes.Clientset
}

// Returns the name of the DNS solver
func (s *DeSECDNSProviderSolver) Name() string {
	return "desec"
}

// Initializes a new client
func (s *DeSECDNSProviderSolver) getClient(config *apiextensionsv1.JSON, namespace string) (*desec.Client, error) {
	// Check if configuration is empty or was not parsed
	if config == nil {
		return nil, fmt.Errorf("missing configuration in issuer found; webhook configuration requires apiKeySecretRef containing deSEC API token")
	}
	// Initialize the configuration object and unmarshal json
	solverConfig := DeSECDNSProviderSolverConfig{}
	if err := json.Unmarshal(config.Raw, &solverConfig); err != nil {
		return nil, fmt.Errorf("invalid configuration in issuer found; webhook configuration requires apiKeySecretRef containing deSEC API token")
	}
	// Check if the k8s client has been initialized
	// This should never happen as cert-manager calls s.Initialize() which assigns the k8s client
	if s.k8s == nil {
		return nil, fmt.Errorf("k8s client has not been initialized by cert-manager; this should never happen")
	}
	// Read the secret from k8s
	secret, err := s.k8s.CoreV1().Secrets(namespace).Get(context.Background(), solverConfig.APIKeySecretRef.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8s secret %s not found in namespace %s", solverConfig.APIKeySecretRef.Name, namespace)
	}
	token, ok := secret.Data[solverConfig.APIKeySecretRef.Key]
	if !ok {
		return nil, fmt.Errorf("k8s secret key %s not found in secret %s in namespace %s", solverConfig.APIKeySecretRef.Key, solverConfig.APIKeySecretRef.Name, namespace)
	}
	// Finally assign the client
	client := desec.New(string(token), desec.NewDefaultClientOptions())

	// Return the client (reuse if initialized)
	return client, nil
}

// Present presents the TXT DNS entry after completion of the ACME DNS-01 challenge
func (s *DeSECDNSProviderSolver) Present(req *acme.ChallengeRequest) error {
	// Create or reuse the API client
	apiClient, err := s.getClient(req.Config, req.ResourceNamespace)
	if err != nil {
		return err
	}
	zone := util.UnFqdn(req.ResolvedZone)
	fqdn := util.UnFqdn(req.ResolvedFQDN)
	// Cut the zone from the fqdn to retrieve the subdomain
	subdomain := util.UnFqdn(strings.Replace(fqdn, zone, "", 0))
	// Check if zone is managed in deSEC
	domain, err := apiClient.Domains.Get(context.Background(), zone)
	if err != nil {
		return fmt.Errorf("domain %s could not be retrieved from deSEC API: %w", zone, err)
	}
	// Create the TXT record to be created
	recordSet := desec.RRSet{
		Domain:  domain.Name,
		SubName: subdomain,
		Records: []string{fmt.Sprintf("\"%s\"", req.Key)},
		Type:    "TXT",
		TTL:     3600,
	}
	// Create the TXT record
	_, err = apiClient.Records.Create(context.Background(), recordSet)
	if err != nil {
		return fmt.Errorf("DNS record %s creation failed: %w", fqdn, err)
	}
	// Return no error
	return nil
}

// Cleanup removes the TXT DNS entry after completion of the ACME DNS-01 challenge
func (s *DeSECDNSProviderSolver) CleanUp(req *acme.ChallengeRequest) error {
	// Create or reuse the API client
	apiClient, err := s.getClient(req.Config, req.ResourceNamespace)
	if err != nil {
		return err
	}
	zone := util.UnFqdn(req.ResolvedZone)
	fqdn := util.UnFqdn(req.ResolvedFQDN)
	// Cut the zone from the fqdn to retrieve the subdomain
	subdomain := util.UnFqdn(strings.Replace(fqdn, zone, "", 0))
	// Check if zone is managed in deSEC
	domain, err := apiClient.Domains.Get(context.Background(), zone)
	if err != nil {
		return fmt.Errorf("domain %s could not be retrieved from deSEC API: %w", zone, err)
	}
	// Delete the TXT record
	err = apiClient.Records.Delete(context.Background(), domain.Name, subdomain, "TXT")
	if err != nil {
		return fmt.Errorf("DNS record %s deletion failed: %w", fqdn, err)
	}
	// Return no error
	return nil
}

// Initializes the solver
func (s *DeSECDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	// Create the k8s client
	k8s, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}
	// Assign the k8s client to the solver
	s.k8s = k8s
	// Return no error
	return nil
}
