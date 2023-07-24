package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	dnssdk "github.com/G-Core/gcore-dns-sdk-go"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	certmgrv1 "github.com/jetstack/cert-manager/pkg/apis/meta/v1"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	providerName    = "gcore"
	groupNameEnvVar = "GROUP_NAME"
	txtType         = "TXT"
)

func main() {

	groupName := os.Getenv(groupNameEnvVar)
	if groupName == "" {
		panic(fmt.Sprintf("%s must be specified", groupNameEnvVar))
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided groupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(groupName,
		&gcoreDNSProviderSolver{},
	)
}

// gcoreDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type gcoreDNSProviderSolver struct {
	client             *kubernetes.Clientset
	ttl                int
	propagationTimeout int
}

// gcoreDNSProviderConfig is a structure that is used to decode into when
// solving a DNS01 challenge.
// This information is provided by cert-manager, and may be a reference to
// additional configuration that's needed to solve the challenge for this
// particular certificate or issuer.
// This typically includes references to Secret resources containing DNS
// provider credentials, in cases where a 'multi-tenant' DNS solver is being
// created.
// If you do *not* require per-issuer or per-certificate configuration to be
// provided to your webhook, you can skip decoding altogether in favour of
// using CLI flags or similar to provide configuration.
// You should not include sensitive information here. If credentials need to
// be used by your provider here, you should reference a Kubernetes Secret
// resource and fetch these credentials using a Kubernetes clientset.
type gcoreDNSProviderConfig struct {
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	APIKeySecretRef certmgrv1.SecretKeySelector `json:"apiKeySecretRef"`

	// +optional. Base url for API requests
	ApiUrl string `json:"apiUrl"`
	// +optional. Permanent token if you don't want to use a k8s secret
	ApiToken string `json:"apiToken"`

	// +optional
	TTL int `json:"ttl"`
	// +optional
	Timeout int `json:"timeout"`
	// +optional
	PropagationTimeout int `json:"propagationTimeout"`
	// +optional
	PollingInterval int `json:"pollingInterval"`
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *gcoreDNSProviderSolver) Name() string {
	return providerName
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *gcoreDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	sdk, err := c.initSDK(ch)
	if err != nil {
		return fmt.Errorf("init sdk: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.propagationTimeout)*time.Second)
	defer cancel()

	err = c.upsertTxtRecord(ctx, sdk, ch)
	if err != nil {
		return fmt.Errorf("detect zone: %w", err)
	}

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *gcoreDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	sdk, err := c.initSDK(ch)
	if err != nil {
		return fmt.Errorf("init sdk: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(c.propagationTimeout)*time.Second)
	defer cancel()

	fqdn := strings.Trim(ch.ResolvedFQDN, ".")
	zone, err := c.detectZone(ctx, fqdn, sdk)
	if err != nil {
		return fmt.Errorf("detect zone: %w", err)
	}

	err = sdk.DeleteRRSet(ctx, zone, fqdn, txtType)
	if err != nil {
		return fmt.Errorf("delete rrset: %w", err)
	}
	return nil
}

// Initialize will be called when the webhook first starts.
// This method can be used to instantiate the webhook, i.e. initialising
// connections or warming up caches.
// Typically, the kubeClientConfig parameter is used to build a Kubernetes
// client that can be used to fetch resources from the Kubernetes API, e.g.
// Secret resources containing credentials used to authenticate with DNS
// provider accounts.
// The stopCh can be used to handle early termination of the webhook, in cases
// where a SIGTERM or similar signal is sent to the webhook process.
func (c *gcoreDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, _ <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return fmt.Errorf("client: %w", err)
	}
	c.client = cl
	return nil
}

func (c *gcoreDNSProviderSolver) upsertTxtRecord(ctx context.Context, sdk *dnssdk.Client, ch *v1alpha1.ChallengeRequest) error {
	fqdn := strings.Trim(ch.ResolvedFQDN, ".")
	zone, err := c.detectZone(ctx, fqdn, sdk)
	if err != nil {
		return fmt.Errorf("detect zone: %w", err)
	}
	recordsToAdd := []dnssdk.ResourceRecord{{Content: []interface{}{ch.Key}, Enabled: true}}
	rrset, err := sdk.RRSet(ctx, zone, fqdn, txtType)
	if err == nil {
		rrset.Records = append(rrset.Records, recordsToAdd...)
		err = sdk.UpdateRRSet(ctx, zone, fqdn, txtType, rrset)
		if err != nil {
			return fmt.Errorf("update rrset: %w", err)
		}
		return nil
	}
	err = sdk.AddZoneRRSet(ctx,
		zone,
		fqdn,
		txtType,
		recordsToAdd,
		c.ttl)
	if err != nil {
		return fmt.Errorf("add rrset: %w", err)
	}
	return nil
}

func (c *gcoreDNSProviderSolver) initSDK(ch *v1alpha1.ChallengeRequest) (*dnssdk.Client, error) {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return nil, fmt.Errorf("load cfg: %w", err)
	}
	apiFullUrl := cfg.ApiUrl
	if apiFullUrl == "" {
		apiFullUrl = "https://api.gcore.com/dns"
	}
	apiURL, err := url.Parse(apiFullUrl)
	if err != nil || apiFullUrl == "" {
		return nil, fmt.Errorf("parse api url %s: %w", apiFullUrl, err)
	}
	token := cfg.ApiToken
	if token == "" {
		token, err = c.extractApiTokenFromSecret(cfg, ch)
		if err != nil {
			return nil, fmt.Errorf("get token: %w", err)
		}
	}
	sdk := dnssdk.NewClient(dnssdk.PermanentAPIKeyAuth(token), func(client *dnssdk.Client) {
		client.BaseURL = apiURL
	})
	if cfg.Timeout > 0 {
		sdk.HTTPClient.Timeout = time.Duration(cfg.Timeout) * time.Second
	}
	if cfg.TTL == 0 {
		cfg.TTL = 300
	}
	c.ttl = cfg.TTL
	if cfg.PropagationTimeout == 0 {
		cfg.PropagationTimeout = 60 * 5
	}
	c.propagationTimeout = cfg.PropagationTimeout
	return sdk, nil
}

func (c *gcoreDNSProviderSolver) extractApiTokenFromSecret(
	cfg gcoreDNSProviderConfig, ch *v1alpha1.ChallengeRequest) (string, error) {
	sec, err := c.client.CoreV1().
		Secrets(ch.ResourceNamespace).
		Get(context.Background(), cfg.APIKeySecretRef.LocalObjectReference.Name, metaV1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("extract secret: %w", err)
	}

	secBytes, ok := sec.Data[cfg.APIKeySecretRef.Key]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret \"%s/%s\"",
			cfg.APIKeySecretRef.Key,
			cfg.APIKeySecretRef.LocalObjectReference.Name,
			ch.ResourceNamespace)
	}

	return string(secBytes), nil
}

func (c *gcoreDNSProviderSolver) detectZone(ctx context.Context, fqdn string, sdk *dnssdk.Client) (string, error) {
	lastErr := fmt.Errorf("empty list")
	zones := extractAllZones(fqdn)
	n := len(zones) - 1
	for i := range zones {
		dnsZone, err := sdk.Zone(ctx, zones[n-i])
		if err == nil {
			return dnsZone.Name, nil
		}
		lastErr = err
	}
	return "", fmt.Errorf("zone %q not found: %w", fqdn, lastErr)
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (gcoreDNSProviderConfig, error) {
	cfg := gcoreDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	return cfg, nil
}

func extractAllZones(fqdn string) []string {
	parts := strings.Split(strings.Trim(fqdn, "."), ".")
	if len(parts) < 3 {
		return nil
	}

	var zones []string
	for i := 1; i < len(parts)-1; i++ {
		zones = append(zones, strings.Join(parts[i:], "."))
	}

	return zones
}
