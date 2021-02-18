/*
Copyright 2019 Cafe Bazaar Cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DomainZoneSpec defines the desired state of DomainZone
type DomainZoneSpec struct {
	Wsid              string     `json:"wsid,omitempty"`
	Origin            string     `json:"origin" validate:"hostname_rfc1123"`
	TTL               int        `json:"ttl,omitempty" validate:"min=0"`
	Email             string     `json:"email,omitempty" validate:"email"`
	DedicatedNSDomain string     `json:"NSDomain,omitempty" validate:"hostname_rfc1123"` // deprecated
	Nameservers       []string   `json:"nameservers,omitempty" validate:"min=2,dive,hostname_rfc1123"`
	Records           RecordsMap `json:"records,omitempty" validate:"max=1000,dive,keys,recordname|eq=@|eq=*,endkeys,max=100,dive"`
}

// RecordsMap ...
type RecordsMap map[string]RecordList

// RecordList ...
type RecordList []Record

// Record ..
type Record struct {
	Name          string `json:"-"` // retrieved from DomainZoneSpec.Records map and set in Zone.FindRecords on runtime
	SpecifiedType string `json:"type,omitempty"`

	TTL         int           `json:"ttl,omitempty" validate:"min=0"`
	Weight      *int          `json:"weight,omitempty" validate:"omitempty,min=0"`
	GeoLocation []GeoLocation `json:"geo,omitempty" validate:"max=20,dive"`
	HealthCheck *HealthCheck  `json:"hc,omitempty" validate:"omitempty,dive"`

	MX           *MX      `json:"MX,omitempty" validate:"omitempty,dive"`
	A            []string `json:"A,omitempty" validate:"omitempty,max=100,dive,ipv4"`
	AFallback    []string `json:"AFallback,omitempty" validate:"omitempty,max=100,dive,ipv4"`
	AAAA         []string `json:"AAAA,omitempty" validate:"omitempty,max=100,dive,ipv6"`
	AAAAFallback []string `json:"AAAAFallback,omitempty" validate:"omitempty,max=100,dive,ipv6"`
	CNAME        string   `json:"CNAME,omitempty" validate:"omitempty,hostname_rfc1123"`
	TXT          string   `json:"TXT,omitempty" validate:"omitempty,lt=2048"`
	SPF          string   `json:"SPF,omitempty" validate:"omitempty,lt=2048"`
	NS           []string `json:"NS,omitempty" validate:"omitempty,max=100,dive,hostname_rfc1123"`
	PTR          string   `json:"PTR,omitempty" validate:"omitempty,hostname_rfc1123"`
	SRV          *SRV     `json:"SRV,omitempty" validate:"omitempty,dive"`
	URI          *URI     `json:"URI,omitempty" validate:"omitempty,dive"`
	ALIAS        string   `json:"ALIAS,omitempty" validate:"omitempty,hostname_rfc1123"`
}

// GeoLocation ...
type GeoLocation struct {
	ContinentCode string `json:"continent"`
	CountryCode   string `json:"country"`
}

// HealthCheck ...
type HealthCheck struct {
	Enabled  bool   `json:"enabled"`
	Protocol string `json:"protocol" validate:"oneof=tcp http https"`
	Port     int    `json:"port" validate:"gte=1,lte=65535"`
	Host     string `json:"host" validate:"omitempty,hostname_rfc1123"`
	Path     string `json:"path" validate:"omitempty,lte=128"`
}

// MX ...
type MX struct {
	Priority int    `json:"priority" validate:"min=0"`
	Host     string `json:"host" validate:"hostname_rfc1123|ip"`
}

// SRV ...
type SRV struct {
	Priority int    `json:"priority" validate:"min=0"`
	Weight   int    `json:"weight" validate:"min=0"`
	Port     int    `json:"port" validate:"min=1,max=65535"`
	Target   string `json:"target" validate:"lte=255"`
}

// URI ...
type URI struct {
	Priority int    `json:"priority" validate:"min=0"`
	Weight   int    `json:"weight" validate:"min=0"`
	Target   string `json:"target" validate:"uri"`
}

// DomainZoneStatus defines the observed state of DomainZone
type DomainZoneStatus struct {
	Status string `json:"status"`
}

// +kubebuilder:object:root=true
// +kubebuilder:printcolumn:name="origin",type="string",JSONPath=".spec.origin",description="origin"
// +kubebuilder:printcolumn:name="nameservers",type="string",JSONPath=".spec.nameservers",description="authoritative nameservers to be set by user"
// +kubebuilder:printcolumn:name="ns (legacy)",type="string",JSONPath=".spec.NSDomain",description="legacy ns domain"
// +kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.status",description="status"
// +kubebuilder:printcolumn:name="age",type="date",JSONPath=".metadata.creationTimestamp"

// DomainZone is the Schema for the domainzones API
type DomainZone struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainZoneSpec   `json:"spec,omitempty"`
	Status DomainZoneStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DomainZoneList contains a list of DomainZone
type DomainZoneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DomainZone `json:"items"`
}

// Type returns type of the record
// TODO(ali) SPF, TLSA, CAA, SMIME?, LOC, SSHP
func (r *Record) Type() string {
	if len(r.A) > 0 {
		return "A"
	} else if len(r.AAAA) > 0 {
		return "AAAA"
	} else if r.TXT != "" {
		return "TXT"
	} else if r.SPF != "" {
		return "SPF"
	} else if r.SRV != nil {
		return "SRV"
	} else if r.CNAME != "" {
		return "CNAME"
	} else if len(r.NS) > 0 {
		return "NS"
	} else if r.PTR != "" {
		return "PTR"
	} else if r.URI != nil {
		return "URI"
	} else if r.MX != nil {
		return "MX"
	} else if r.ALIAS != "" {
		return "ALIAS"
	}

	return "A"
}
