// Package desec provides an API client for desec.io.
//
// Based on https://github.com/j-be/cert-manager-webhook-desec, licensed under Apache-2.0.
package desec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// API is the basic implemenataion of an API client for desec.io
type API struct {
	BaseUrl string
	Token   string
}

// ErrorResponse defines the error response format
type ErrorResponse struct {
	Detail string `json:"detail,omitempty"`
}

// DNSDomain defines the format of a Domain object
type DNSDomain struct {
	Created    string `json:"created,omitempty"`
	Published  string `json:"published,omitempty"`
	Name       string `json:"name,omitempty"`
	MinimumTTL int    `json:"minimum_ttl,omitempty"`
	Touched    string `json:"touched,omitempty"`
}

// DNSDomains is a slice of Domain objects
type DNSDomains []DNSDomain

// RRSet defines the format of a Resource Record Set object
type RRSet struct {
	Domain  string   `json:"domain,omitempty"`
	SubName string   `json:"subname,omitempty"`
	Name    string   `json:"name,omitempty"`
	Type    string   `json:"type,omitempty"`
	Records []string `json:"records"`
	TTL     int      `json:"ttl,omitempty"`
	Created string   `json:"created,omitempty"`
	Touched string   `json:"touched,omitempty"`
}

// RRSets is a slice of Resource Record Set objects
type RRSets []RRSet

// request builds and executes the raw HTTP request
func (a *API) request(method, path string, body io.Reader, target interface{}) error {
	if path[0] != '/' {
		path = "/" + path
	}

	url := a.BaseUrl + path

	client := &http.Client{Timeout: time.Second * 10}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Token "+a.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := resp.Body.Close()
		if closeErr != nil {
			klog.Warningf("error closing deSEC API response body: %v", closeErr)
		}
	}()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var errResp ErrorResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
			return fmt.Errorf("%s %s unknown error occured", method, path)
		}
		return fmt.Errorf("%s %s error: %s", method, path, errResp.Detail)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return fmt.Errorf("%s %s response parsing error: %v", method, path, err)
	}

	return nil
}

// GetDNSDomains returns all dns domains managed by deSEC
func (a *API) GetDNSDomains() (DNSDomains, error) {
	domains := new(DNSDomains)
	err := a.request("GET", "domains/", nil, domains)
	if err != nil {
		return nil, err
	}
	return *domains, nil
}

// GetDNSDomain gets the dns domain associated with the given subdomain
func (a *API) GetDNSDomain(subdomain string) (*DNSDomain, error) {
	domains, err := a.GetDNSDomains()
	if err != nil {
		return nil, err
	}
	for _, v := range domains {
		if strings.HasSuffix(subdomain, v.Name) {
			return &v, nil
		}
	}
	return nil, fmt.Errorf("domain not found")
}

// GetRRSets returns all resource record sets for a given domain, subdomain and type
func (a *API) GetRRSets(subName, domainName, rtype string) (RRSets, error) {
	// Build path
	params := url.Values{"subname": {subName}, "type": {rtype}}
	path := "domains/" + domainName + "/rrsets/?" + params.Encode()

	// Call API
	rrsets := new(RRSets)
	err := a.request("GET", path, nil, rrsets)
	if err != nil {
		return nil, err
	}
	return *rrsets, nil
}

// AddRecord adds a resource record to a new or existing RRSet
func (a *API) AddRecord(subName, domainName, rtype, content string, ttl int) (RRSets, error) {
	// First check if there's already and existing RRSet
	rrsets, err := a.GetRRSets(subName, domainName, rtype)
	if err != nil {
		return nil, err
	}
	// Quote content if record type is TXT
	if rtype == "TXT" {
		content = "\"" + content + "\""
	}
	var rrset RRSet
	if len(rrsets) > 0 {
		// RRSet exists, so see if we need to append a new record
		rrset = rrsets[0]
		for _, r := range rrset.Records {
			if r == content {
				// record already exists so just return
				return rrsets, nil
			}
		}
		// record doesn't exists so append it
		rrset.Records = append(rrset.Records, content)
	} else {
		// No existing RRSet found, so create a new one
		records := []string{content}
		rrset = RRSet{SubName: subName, Type: rtype, Records: records, TTL: ttl}
	}
	// write RRSet to deSEC
	rrsets, err = a.updateRRSet(rrset, domainName)
	if err != nil {
		return nil, err
	}
	return rrsets, nil
}

// DeleteRecord deletes a record from an existing RRSet if it exists
func (a *API) DeleteRecord(subName, domainName, rtype, content string) (RRSets, error) {
	// Check if RRSet actually exists
	rrsets, err := a.GetRRSets(subName, domainName, rtype)
	if err != nil {
		return nil, err
	}
	if len(rrsets) > 0 {
		// Quote content if record type is TXT
		if rtype == "TXT" {
			content = "\"" + content + "\""
		}
		rrset := rrsets[0]
		var records []string
		// Create a new records slice containing all records except for the one to be deleted
		for _, r := range rrset.Records {
			if r != content {
				records = append(records, r)
			}
		}
		// Check that records have actually changed before sending update to the API
		if len(rrset.Records) != len(records) {
			// Fix empty slice from being marshalled as null
			if len(records) == 0 {
				records = make([]string, 0)
			}
			rrset.Records = records
			rrsets, err := a.updateRRSet(rrset, domainName)
			if err != nil {
				return nil, err
			}
			return rrsets, nil
		}
		return rrsets, nil
	}
	// No existing RRSet found so just return an empty RRSets object
	return RRSets{}, nil
}

func (a *API) updateRRSet(rrset RRSet, domainName string) (RRSets, error) {
	rrsets := RRSets{}
	rrsets = append(rrsets, rrset)
	rawJSON, err := json.Marshal(rrsets)
	if err != nil {
		return nil, err
	}
	path := "domains/" + domainName + "/rrsets/"
	err = a.request("PUT", path, bytes.NewBuffer(rawJSON), &rrsets)
	if err != nil {
		return nil, err
	}
	return rrsets, nil
}
