package ns123domain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	slog.Debug("presenting TXT record", "domain", domain.fqdn, "name", recordName)

	records, err := c.listRecords(domain)
	if err != nil {
		return err
	}
	slog.Debug("DNS API returned records while checking TXT", "count", len(records), "name", recordName)
	logTXTRecordCandidates("existing", recordName, records)
	for _, record := range records {
		if isTXTRecord(record, recordName, value) {
			slog.Debug("matching enabled TXT record already exists", "domain", domain.fqdn, "name", recordName)
			return nil
		}
	}

	record := dnsRecord{
		Name: recordName,
		Type: "TXT",
		TTL:  60,
		Attributes: []dnsRecordAttribute{
			{Key: "value", Value: value},
		},
	}
	if err := c.request(http.MethodPost, domain, "", record, nil); err != nil {
		return fmt.Errorf("creating TXT record %q: %w", fqdn, err)
	}
	slog.Debug("TXT creation request accepted; verifying with DNS API list", "domain", domain.fqdn, "name", recordName)
	createdRecords, err := c.listRecords(domain)
	if err != nil {
		return fmt.Errorf("verifying creation of TXT record %q: %w", fqdn, err)
	}
	slog.Debug("DNS API returned records after TXT creation", "count", len(createdRecords), "name", recordName)
	logTXTRecordCandidates("after-create", recordName, createdRecords)
	for _, createdRecord := range createdRecords {
		if isTXTRecord(createdRecord, recordName, value) {
			slog.Debug("matching enabled TXT record confirmed by follow-up list", "domain", domain.fqdn, "name", recordName)
			return nil
		}
	}
	slog.Debug("follow-up DNS API list did not contain an enabled matching TXT record", "domain", domain.fqdn, "name", recordName)
	return fmt.Errorf("DNS API did not return an enabled matching TXT record for %q after creation", fqdn)
}

func logTXTRecordCandidates(stage, name string, records []dnsRecord) {
	for _, record := range records {
		if !strings.EqualFold(record.Name, name) {
			continue
		}
		attributeKeys := make([]string, 0, len(record.Attributes))
		for _, attribute := range record.Attributes {
			attributeKeys = append(attributeKeys, attribute.Key)
		}
		slog.Debug("DNS record candidate", "stage", stage, "type", record.Type, "name", record.Name, "disabled", record.Disabled, "attributeKeys", attributeKeys)
	}
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
	slog.Debug("123domain API request", "method", method, "path", path)

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
		slog.Debug("123domain API request failed", "method", method, "path", path, "error", err)
		return fmt.Errorf("sending DNS API request: %w", err)
	}
	defer resp.Body.Close()
	slog.Debug("123domain API response", "method", method, "path", path, "status", resp.Status)

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
	if record.Disabled || !strings.EqualFold(record.Type, "TXT") || !strings.EqualFold(record.Name, name) {
		return false
	}
	for _, attribute := range record.Attributes {
		if attribute.Key == "value" && attribute.Value == value {
			return true
		}
	}
	return false
}
