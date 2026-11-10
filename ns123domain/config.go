package ns123domain

import (
	"encoding/json"
	"fmt"

	certmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	extapi "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

type providerConfig struct {
	K8S123DomainAPISecretRef certmetav1.LocalObjectReference `json:"apiSecretKeyRef"`
}

func loadConfig(cfgJSON *extapi.JSON) (providerConfig, error) {
	cfg := providerConfig{}
	if cfgJSON == nil {
		return cfg, nil
	}
	if err := json.Unmarshal(cfgJSON.Raw, &cfg); err != nil {
		return cfg, fmt.Errorf("error decoding solver config: %w", err)
	}
	return cfg, nil
}
