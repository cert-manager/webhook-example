package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"net/http"
	"io"
	"bytes"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
)

var GroupName = os.Getenv("GROUP_NAME")

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
		&abioncoreDNSProviderSolver{},
	)
}

// abioncoreDNSProviderSolver implements the provider-specific logic needed to
// 'present' an ACME challenge TXT record for your own DNS provider.
// To do so, it must implement the `github.com/cert-manager/cert-manager/pkg/acme/webhook.Solver`
// interface.
type abioncoreDNSProviderSolver struct {

}

// customDNSProviderConfig is a structure that is used to decode into when
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
type abioncoreDNSProviderConfig struct {
	// Change the two fields below according to the format of the configuration
	// to be decoded.
	// These fields will be set by users in the
	// `issuer.spec.acme.dns01.providers.webhook.config` field.
	APIKey string `json:"x-api-key"`
}
// Name is used as the name for this DNS solver when referencing it on the ACME
// Issuer resource.
// This should be unique **within the group name**, i.e. you can have two
// solvers configured with the same Name() **so long as they do not co-exist
// within a single webhook deployment**.
// For example, `cloudflare` may be used as the name of a solver.
func (c *abioncoreDNSProviderSolver) Name() string {
	return "abion-core"
}

type Entry struct {
    Data Data `json:"data"`
}
type Txt struct {
	TTL      int    `json:"ttl"`
    Rdata    string `json:"rdata"`
	Comments string `json:"comments"`
}
type AcmeChallenge struct {
    Txt []Txt `json:"TXT"`
}
type Records struct {
    AcmeChallenge AcmeChallenge `json:"_acme-challenge"`
}
type Attributes struct {
    Records Records `json:"records"`
}
type Data struct {
    Type       string     `json:"type"`
    ID         string     `json:"id"`
    Attributes Attributes `json:"attributes"`
}

// Present is responsible for actually presenting the DNS record with the
// DNS provider.
// This method should tolerate being called multiple times with the same value.
// cert-manager itself will later perform a self check to ensure that the
// solver has correctly configured the DNS provider.
func (c *abioncoreDNSProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {

	// klog.V(2).Infof("Present: namespace=%s, zone=%s, fqdn=%s",
	// ch.ResourceNamespace, ch.ResolvedZone, ch.ResolvedFQDN)

	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	// Get name and zone from challenge request
	// name := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	zone := strings.TrimSuffix(ch.ResolvedZone, ".")
	
	// TODO: do something more useful with the decoded configuration
	fmt.Printf("Decoded configuration %v", cfg)

	// https://demo.abion.com/pmapi/  ################### Main Api

	// Get all zones (GET https://demo.abion.com/pmapi/v1/zones)
	// Create client
	client := &http.Client{}

	// Create request
	request, err := http.NewRequest("GET", "https://demo.abion.com/pmapi/v1/zones/"+zone, nil)
	// Headers
	request.Header.Add("X-API-KEY", cfg.APIKey)

	// Fetch Request
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Failure : ", err)
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("did not get expected HTTP 200 but %s", response.Status)
	}

	// Display Results
	fmt.Println("response Status : ", response.Status)
	fmt.Println("response Headers : ", response.Header)

	// Create DNS
	entry, err := json.Marshal(
		Entry{
			Data{"zone", zone, 
				Attributes{
					Records{
						AcmeChallenge{
							[]Txt{
								{
									300,
									ch.Key,
									"Let's Encrypt _acme-challenge",
								},
							},
						},
					},
				},
			},
		},
	)
	body := bytes.NewBuffer(entry)
	fmt.Println("entry json struct : ", body)

	// Create request
	request, err = http.NewRequest("PATCH", "https://demo.abion.com/pmapi/v1/zones/"+zone, body)
	// Headers
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-API-KEY", cfg.APIKey)

	// Fetch Request
	response, err = client.Do(request)
	if err != nil {
		fmt.Println("Failure : ", err)
	}

	// Read Response Body
	responseBody, _ := io.ReadAll(response.Body)

	// Display Results
	fmt.Println("response Status : ", response.Status)
	fmt.Println("response Headers : ", response.Header)
	fmt.Println("response Body : ", string(responseBody))

	return nil
}

// CleanUp should delete the relevant TXT record from the DNS provider console.
// If multiple TXT records exist with the same record name (e.g.
// _acme-challenge.example.com) then **only** the record with the same `key`
// value provided on the ChallengeRequest should be cleaned up.
// This is in order to facilitate multiple DNS validations for the same domain
// concurrently.
func (c *abioncoreDNSProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	// Get name and zone from challenge request
	// name := strings.TrimSuffix(strings.TrimSuffix(ch.ResolvedFQDN, ch.ResolvedZone), ".")
	zone := strings.TrimSuffix(ch.ResolvedZone, ".")
	
	// https://demo.abion.com/pmapi/  ################### Main Api

	// Get all zones (GET https://demo.abion.com/pmapi/v1/zones)
	// Create client
	client := &http.Client{}

	// Create request
	request, err := http.NewRequest("GET", "https://demo.abion.com/pmapi/v1/zones/"+zone, nil)
	// Headers
	request.Header.Add("X-API-KEY", cfg.APIKey)

	// Fetch Request
	response, err := client.Do(request)
	if err != nil {
		fmt.Println("Failure : ", err)
		return err
	}
	if response.StatusCode != 200 {
		return fmt.Errorf("did not get expected HTTP 200 but %s", response.Status)
	}

	// Display Results
	fmt.Println("response Status : ", response.Status)
	fmt.Println("response Headers : ", response.Header)
	//fmt.Println("response Body : ", respBody.Zones[0].ZoneID)

	// Clear txt
	entry, err := json.Marshal(
		Entry{
			Data{"zone", zone, 
				Attributes{
					Records{
						AcmeChallenge{
							nil,
						},
					},
				},
			},
		},
	)
	body := bytes.NewBuffer(entry)

	// Create request
	request, err = http.NewRequest("PATCH", "https://demo.abion.com/pmapi/v1/zones/"+zone, body)
	// Headers
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("X-API-KEY", cfg.APIKey)

	// Fetch Request
	response, err = client.Do(request)
	if err != nil {
		fmt.Println("Failure : ", err)
	}

	// Read Response Body
	responseBody, _ := io.ReadAll(response.Body)

	// Display Results
	fmt.Println("response Status : ", response.Status)
	fmt.Println("response Headers : ", response.Header)
	fmt.Println("response Body : ", string(responseBody))

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
func (c *abioncoreDNSProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	///// UNCOMMENT THE BELOW CODE TO MAKE A KUBERNETES CLIENTSET AVAILABLE TO
	///// YOUR CUSTOM DNS PROVIDER

	//cl, err := kubernetes.NewForConfig(kubeClientConfig)
	//if err != nil {
	//	return err
	//}
	//
	//c.client = cl

	///// END OF CODE TO MAKE KUBERNETES CLIENTSET AVAILABLE
	return nil
}

// loadConfig is a small helper function that decodes JSON configuration into
// the typed config struct.
func loadConfig(cfgJSON *extapi.JSON) (abioncoreDNSProviderConfig, error) {

	cfg := abioncoreDNSProviderConfig{}
	// handle the 'base case' where no configuration has been provided
	if cfgJSON == nil {
		return cfg, nil
	}
	
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding config: %v", err)
	}

	return cfg, nil
}
