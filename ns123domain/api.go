package ns123domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type dnsAPIClient struct {
	baseURL       string
	authorization string
	httpClient    *http.Client
}

type dnsRecord struct {
	ID         string               `json:"id,omitempty"`
	Name       string               `json:"name"`
	Type       string               `json:"type"`
	TTL        int                  `json:"ttl"`
	Comment    string               `json:"comment,omitempty"`
	Disabled   bool                 `json:"disabled,omitempty"`
	UserData   string               `json:"userdata,omitempty"`
	Attributes []dnsRecordAttribute `json:"attributes"`
}

type dnsRecordAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func (c *dnsAPIClient) presentTXT(zone, fqdn, value string) error {
	domain, recordName, err := recordLocation(zone, fqdn)
	if err != nil {
		return err
	}

	records, err := c.listRecords(domain)
	if err != nil {
		return err
	}
	for _, record := range records {
		if isTXTRecord(record, recordName, value) {
			return nil
		}
	}

	record := dnsRecord{
		Name: recordName,
		Type: "TXT",
		TTL:  60,
		Attributes: []dnsRecordAttribute{
			{Key: "text", Value: value},
		},
	}
	if err := c.request(http.MethodPost, domain, "", record, nil); err != nil {
		return fmt.Errorf("creating TXT record %q: %w", fqdn, err)
	}
	return nil
}

func (c *dnsAPIClient) cleanTXT(zone, fqdn, value string) error {
	domain, recordName, err := recordLocation(zone, fqdn)
	if err != nil {
		return err
	}

	records, err := c.listRecords(domain)
	if err != nil {
		return err
	}
	for _, record := range records {
		if !isTXTRecord(record, recordName, value) {
			continue
		}
		if record.ID == "" {
			return fmt.Errorf("DNS API returned a matching TXT record for %q without an ID", fqdn)
		}
		if err := c.request(http.MethodDelete, domain, record.ID, nil, nil); err != nil {
			return fmt.Errorf("deleting TXT record %q (ID %q): %w", fqdn, record.ID, err)
		}
	}
	return nil
}

func (c *dnsAPIClient) listRecords(domain apiDomain) ([]dnsRecord, error) {
	var records []dnsRecord
	if err := c.request(http.MethodGet, domain, "", nil, &records); err != nil {
		return nil, fmt.Errorf("listing DNS records for %q: %w", domain.fqdn, err)
	}
	return records, nil
}

func (c *dnsAPIClient) request(method string, domain apiDomain, recordID string, body, result any) error {
	path := fmt.Sprintf("/v1/domains/%s/%s/dns", url.PathEscape(domain.toplevel), url.PathEscape(domain.name))
	if recordID != "" {
		path += "/" + url.PathEscape(recordID)
	}

	var requestBody io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding DNS API request: %w", err)
		}
		requestBody = bytes.NewReader(encoded)
	}
	req, err := http.NewRequest(method, strings.TrimRight(c.baseURL, "/")+path, requestBody)
	if err != nil {
		return fmt.Errorf("building DNS API request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", c.authorization)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending DNS API request: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("reading DNS API response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("DNS API returned %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
	}
	if result != nil {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("decoding DNS API response: %w", err)
		}
	}
	return nil
}

type apiDomain struct {
	name     string
	toplevel string
	fqdn     string
}

func recordLocation(zone, fqdn string) (apiDomain, string, error) {
	zone = strings.TrimSuffix(strings.TrimSpace(zone), ".")
	fqdn = strings.TrimSuffix(strings.TrimSpace(fqdn), ".")
	if zone == "" || fqdn == "" {
		return apiDomain{}, "", fmt.Errorf("resolved zone and FQDN must both be provided")
	}

	labels := strings.SplitN(zone, ".", 2)
	if len(labels) != 2 || labels[0] == "" || labels[1] == "" {
		return apiDomain{}, "", fmt.Errorf("resolved zone %q must include a domain name and toplevel", zone)
	}
	domain := apiDomain{name: labels[0], toplevel: labels[1], fqdn: zone}

	if strings.EqualFold(fqdn, zone) {
		return domain, "@", nil
	}
	suffix := "." + zone
	if len(fqdn) <= len(suffix) || !strings.EqualFold(fqdn[len(fqdn)-len(suffix):], suffix) {
		return apiDomain{}, "", fmt.Errorf("resolved FQDN %q is not within resolved zone %q", fqdn, zone)
	}
	return domain, fqdn[:len(fqdn)-len(suffix)], nil
}

func isTXTRecord(record dnsRecord, name, value string) bool {
	if !strings.EqualFold(record.Type, "TXT") || !strings.EqualFold(record.Name, name) {
		return false
	}
	for _, attribute := range record.Attributes {
		if attribute.Key == "text" && attribute.Value == value {
			return true
		}
	}
	return false
}
