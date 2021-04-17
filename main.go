package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"

	validator "github.com/go-playground/validator/v10"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/jetstack/cert-manager/pkg/acme/webhook/cmd"
	"github.com/jetstack/cert-manager/pkg/issuer/acme/dns/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/aliorouji/cert-manager-webhook-sotoon/api/v1beta1"
)

var GroupName = os.Getenv("GROUP_NAME")

var validate *validator.Validate

func init() {
	validate = validator.New()
}

func main() {
	if GroupName == "" {
		panic("GROUP_NAME must be specified")
	}

	// This will register our custom DNS provider with the webhook serving
	// library, making it available as an API under the provided GroupName.
	// You can register multiple DNS provider implementations with a single
	// webhook, where the Name() method will be used to disambiguate between
	// the different implementations.
	cmd.RunWebhookServer(GroupName,
		&sotoonDNSProviderSolver{},
	)
}

// sotoonDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/jetstack/cert-manager/pkg/acme/webhook.Solver`
// interface.
type sotoonDNSProviderSolver struct {
	// If a Kubernetes 'clientset' is needed, you must:
	// 1. uncomment the additional `client` field in this structure below
	// 2. uncomment the "k8s.io/client-go/kubernetes" import at the top of the file
	// 3. uncomment the relevant code in the Initialize method below
	// 4. ensure your webhook's service account has the required RBAC role
	//    assigned to it for interacting with the Kubernetes APIs you need.
	client *kubernetes.Clientset
}

// sotoonDNSProviderConfig is a structure that is used to decode into when
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
type sotoonDNSProviderConfig struct {
	// Change the two fields below according to the format of the configuration
	// to be decoded.
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.

	Endpoint          string                   `json:"endpoint" validate:"url"`
	Namespace         string                   `json:"namespace" validate:"omitempty,hostname_rfc1123"`
	APITokenSecretRef corev1.SecretKeySelector `json:"apiTokenSecretRef"`
}

func (c *sotoonDNSProviderConfig) validate() error {
	return validate.Struct(c)
}

func (c *sotoonDNSProviderSolver) secret(ref corev1.SecretKeySelector, namespace string) (string, error) {
	if ref.Name == "" {
		return "", nil
	}

	secret, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), ref.Name, metav1.GetOptions{})
	if err != nil {
		return "", err
	}

	bytes, ok := secret.Data[ref.Key]
	if !ok {
		return "", fmt.Errorf("key not found %q in secret '%s/%s'", ref.Key, namespace, ref.Name)
	}
	return string(bytes), nil
}

func (c *sotoonDNSProviderSolver) sotoonClient(ch *v1alpha1.ChallengeRequest, cfg *sotoonDNSProviderConfig) (*rest.RESTClient, error) {
	apiToken, err := c.secret(cfg.APITokenSecretRef, ch.ResourceNamespace)
	if err != nil {
		return nil, err
	}

	v1beta1.AddToScheme(scheme.Scheme)

	restConfig := &rest.Config{}
	restConfig.Host = cfg.Endpoint
	restConfig.APIPath = "/apis"
	restConfig.BearerToken = apiToken
	restConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: v1beta1.GroupName, Version: v1beta1.GroupVersion}
	restConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	restConfig.UserAgent = rest.DefaultKubernetesUserAgent()

	return rest.UnversionedRESTClientFor(restConfig)
}

// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *sotoonDNSProviderSolver) Name() string {
	return "sotoon"
}

func getRelevantZones(sotoonClient *rest.RESTClient, namespace, origin string) (*v1beta1.DomainZoneList, error) {
	dzl := &v1beta1.DomainZoneList{}

	if err := sotoonClient.
		Get().
		Namespace(namespace).
		Resource("domainzones").
		VersionedParams(&metav1.ListOptions{LabelSelector: fmt.Sprintf("dns.ravh.ir/origin=%s", origin)}, scheme.ParameterCodec).
		Do(context.TODO()).
		Into(dzl); err != nil {
		return nil, err
	}

	return dzl, nil
}

func addTXTRecord(sotoonClient *rest.RESTClient, zone *v1beta1.DomainZone, subdomain, target string) error {
	if zone.Status.Status == "OK" {
		if zone.Spec.Records == nil {
			zone.Spec.Records = make(v1beta1.RecordsMap)
		}

		records := zone.Spec.Records[subdomain]
		if records == nil {
			records = v1beta1.RecordList{}
		}

		for _, r := range records {
			if r.TXT == target {
				return nil
			}
		}

		records = append(records, v1beta1.Record{
			SpecifiedType: "TXT",
			TXT:           target,
			TTL:           30,
		})

		zone.Spec.Records[subdomain] = records

		zoneData, err := json.Marshal(zone)
		if err != nil {
			return err
		}

		if err := sotoonClient.Put().Name(zone.Name).Namespace(zone.Namespace).Resource("domainzones").Body(zoneData).Do(context.TODO()).Into(zone); err != nil {
			return err
		}
	}

	return nil
}

func removeTXTRecord(sotoonClient *rest.RESTClient, zone *v1beta1.DomainZone, subdomain, target string) error {
	if zone.Status.Status == "OK" {
		if zone.Spec.Records == nil {
			return nil
		}

		records := zone.Spec.Records[subdomain]
		if records == nil {
			return nil
		}

		var newRecords v1beta1.RecordList
		for _, r := range records {
			if r.TXT != target {
				newRecords = append(newRecords, r)
			}
		}

		if len(newRecords) == len(records) {
			return nil
		}

		zone.Spec.Records[subdomain] = newRecords

		zoneData, err := json.Marshal(zone)
		if err != nil {
			return err
		}

		if err := sotoonClient.Put().Name(zone.Name).Namespace(zone.Namespace).Resource("domainzones").Body(zoneData).Do(context.TODO()).Into(zone); err != nil {
			return err
		}
	}

	return nil
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *sotoonDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch)
	if err != nil {
		return err
	}

	sotoonClient, err := c.sotoonClient(ch, cfg)
	if err != nil {
		return err
	}

	origin := util.UnFqdn(ch.ResolvedZone)
	zones, err := getRelevantZones(sotoonClient, cfg.Namespace, origin)
	if err != nil {
		return err
	}

	subdomain := getSubDomain(origin, ch.ResolvedFQDN)
	target := ch.Key

	for _, zone := range zones.Items {
		if err := addTXTRecord(sotoonClient, &zone, subdomain, target); err != nil {
			return err
		}
	}

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *sotoonDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch)
	if err != nil {
		return err
	}

	sotoonClient, err := c.sotoonClient(ch, cfg)
	if err != nil {
		return err
	}

	origin := util.UnFqdn(ch.ResolvedZone)
	zones, err := getRelevantZones(sotoonClient, cfg.Namespace, origin)
	if err != nil {
		return err
	}

	subdomain := getSubDomain(origin, ch.ResolvedFQDN)
	target := ch.Key

	for _, zone := range zones.Items {
		if err := removeTXTRecord(sotoonClient, &zone, subdomain, target); err != nil {
			return err
		}
	}

	// TODO: add code that deletes a record from the DNS provider's console
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
func (c *sotoonDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	c.client = cl

	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(ch *v1alpha1.ChallengeRequest) (*sotoonDNSProviderConfig, error) {
	cfgJSON := ch.Config

	cfg := &sotoonDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}

	if err := json.Unmarshal(cfgJSON.Raw, cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %v", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	if cfg.Namespace == "" {
		cfg.Namespace = ch.ResourceNamespace
	}

	return cfg, nil
}

// utils
func getSubDomain(domain, fqdn string) string {
	if idx := strings.Index(fqdn, "."+domain); idx != -1 {
		return fqdn[:idx]
	}

	return util.UnFqdn(fqdn)
}
