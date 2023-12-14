package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"

	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/cert-manager/cert-manager/pkg/acme/webhook/apis/acme/v1alpha1"
	"github.com/cert-manager/cert-manager/pkg/acme/webhook/cmd"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"

	cis "github.com/IBM-Cloud/bluemix-go/api/cis/cisv1"
	cissession "github.com/IBM-Cloud/bluemix-go/session"
)

var GroupName = os.Getenv("GROUP_NAME")
var IbmCloudApiKey = os.Getenv("IBMCLOUD_API_KEY")

func main() {
	if GroupName == "" {
		log.Fatal("GROUP_NAME must be specified")
	}

	if IbmCloudApiKey == "" {
		log.Fatal("IBMCLOUD_API_KEY must be specified")
	}

	cmd.RunWebhookServer(GroupName, &ibmCloudCisProviderSolver{})
}

type ibmCloudCisProviderSolver struct {
	client         *kubernetes.Clientset
	ibmCloudCisApi cis.CisServiceAPI
}

type ibmCloudCisDnsProviderConfig struct {
	IbmCloudCisCrns []string                 `json:"ibmCloudCisCrns"`
	APIKeySecretRef cmmeta.SecretKeySelector `json:"apiKeySecretRef"`
}

func (c *ibmCloudCisProviderSolver) Name() string {
	return "ibm-cloud-cis"
}

func (c *ibmCloudCisProviderSolver) Present(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	zonesApi := c.ibmCloudCisApi.Zones()

	for _, crn := range cfg.IbmCloudCisCrns {
		myZones, err := zonesApi.ListZones(crn)
		if err != nil {
			log.WithError(err).WithField("crn", crn).Error("Error listing zones")
			continue
		}

		longestMatchZone := findLongestMatchingZone(myZones, ch.ResolvedFQDN)
		if longestMatchZone != nil {
			if err := c.createDNSChallengeRecord(crn, longestMatchZone.Id, ch); err != nil {
				return err
			}
		}
	}

	return nil
}

func findLongestMatchingZone(zones []cis.Zone, fqdn string) *cis.Zone {
	var longestMatchZone *cis.Zone
	var longestMatchLength int

	for _, zone := range zones {
		zoneNameWithDot := zone.Name + "."
		if strings.HasSuffix(fqdn, zoneNameWithDot) && len(zoneNameWithDot) > longestMatchLength {
			longestMatchLength = len(zoneNameWithDot)
			longestMatchZone = &zone
		}
	}

	return longestMatchZone
}

func (c *ibmCloudCisProviderSolver) createDNSChallengeRecord(crn, zoneID string, ch *v1alpha1.ChallengeRequest) error {
	dnsAPI := c.ibmCloudCisApi.Dns()

	_, err := dnsAPI.CreateDns(crn, zoneID, cis.DnsBody{
		Name:    ch.ResolvedFQDN,
		DnsType: "TXT",
		Content: ch.Key,
	})

	if err != nil {
		log.WithError(err).WithFields(log.Fields{"crn": crn, "zoneID": zoneID}).Error("Error creating DNS01 challenge")
		return err
	}

	log.WithFields(log.Fields{"fqdn": ch.ResolvedFQDN, "key": ch.Key}).Info("DNS01 challenge created")
	return nil
}

func (c *ibmCloudCisProviderSolver) CleanUp(ch *v1alpha1.ChallengeRequest) error {
	cfg, err := loadConfig(ch.Config)
	if err != nil {
		return err
	}

	zonesApi := c.ibmCloudCisApi.Zones()

	for _, crn := range cfg.IbmCloudCisCrns {
		myZones, err := zonesApi.ListZones(crn)
		if err != nil {
			log.WithError(err).WithField("crn", crn).Error("Error listing zones")
			continue
		}

		longestMatchZone := findLongestMatchingZone(myZones, ch.ResolvedFQDN)
		if longestMatchZone != nil {
			if err := c.deleteMatchingTXTRecords(crn, longestMatchZone.Id, ch); err != nil {
				log.WithError(err).Error("Error deleting TXT record")
			}
		}
	}

	return nil
}

func (c *ibmCloudCisProviderSolver) deleteMatchingTXTRecords(crn, zoneID string, ch *v1alpha1.ChallengeRequest) error {
	dnsAPI := c.ibmCloudCisApi.Dns()

	myDnsrecs, err := dnsAPI.ListDns(crn, zoneID)
	if err != nil {
		return err
	}

	for _, myDnsrec := range myDnsrecs {
		if myDnsrec.DnsType == "TXT" && (myDnsrec.Name+".") == ch.ResolvedFQDN && myDnsrec.Content == ch.Key {
			if err := dnsAPI.DeleteDns(crn, zoneID, myDnsrec.Id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *ibmCloudCisProviderSolver) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	cl, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return err
	}

	ibmSession, err := cissession.New()
	if err != nil {
		return fmt.Errorf("IBM Cloud session failed: %w", err)
	}

	ibmCloudCisApi, err := cis.New(ibmSession)
	if err != nil {
		return err
	}

	c.ibmCloudCisApi = ibmCloudCisApi
	c.client = cl
	return nil
}

func loadConfig(cfgJSON *extapi.JSON) (ibmCloudCisDnsProviderConfig, error) {
	cfg := ibmCloudCisDnsProviderConfig{}
	if cfgJSON == nil {
		return cfg, fmt.Errorf("config JSON is nil")
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %w", err)
	}
	return cfg, nil
}
