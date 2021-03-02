package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// JSON body for Bluecat entity requests and responses.
type bluecatEntity struct {
	ID         string `json:"id,omitempty"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Properties string `json:"properties"`
}

type entityResponse struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Properties string `json:"properties"`
}

var baseURL string
var token string
var configName string

func bluecatLogin(bluecatURL, username, password string, bluecatConfigName string) error {
	baseURL = bluecatURL
	configName = bluecatConfigName
	queryArgs := map[string]string{
		"username": username,
		"password": password,
	}

	resp, err := bluecatSendRequest(http.MethodGet, "login", nil, queryArgs)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	authBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("bluecat: %w", err)
	}
	authResp := string(authBytes)

	if strings.Contains(authResp, "Authentication Error") {
		msg := strings.Trim(authResp, "\"")
		return fmt.Errorf("bluecat: request failed: %s", msg)
	}

	token = regexp.MustCompile("BAMAuthToken: [^ ]+").FindString(authResp)
	return nil
}

func bluecatLogout() error {
	if len(token) == 0 {
		return nil
	}

	resp, err := bluecatSendRequest(http.MethodGet, "logout", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("bluecat: request failed to delete session with HTTP status code %d", resp.StatusCode)
	}

	authBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	authResp := string(authBytes)

	if !strings.Contains(authResp, "successfully") {
		msg := strings.Trim(authResp, "\"")
		return fmt.Errorf("bluecat: request failed to delete session: %s", msg)
	}

	token = ""

	return nil
}

func bluecatLookupConfID() (uint, error) {
	queryArgs := map[string]string{
		"parentId": strconv.Itoa(0),
		"name":     configName,
		"type":     "Configuration",
	}

	resp, err := bluecatSendRequest(http.MethodGet, "getEntityByName", nil, queryArgs)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var conf entityResponse
	err = json.NewDecoder(resp.Body).Decode(&conf)
	if err != nil {
		return 0, fmt.Errorf("bluecat: %w", err)
	}
	return conf.ID, nil
}

func bluecatLookupViewID(viewName string) (uint, error) {
	confID, err := bluecatLookupConfID()
	if err != nil {
		return 0, err
	}

	queryArgs := map[string]string{
		"parentId": strconv.FormatUint(uint64(confID), 10),
		"name":     viewName,
		"type":     "View",
	}

	resp, err := bluecatSendRequest(http.MethodGet, "getEntityByName", nil, queryArgs)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var view entityResponse
	err = json.NewDecoder(resp.Body).Decode(&view)
	if err != nil {
		return 0, fmt.Errorf("bluecat: %w", err)
	}

	return view.ID, nil
}

func bluecatLookupParentZoneID(viewID uint, fqdn string) (uint, string, error) {
	parentViewID := viewID
	name := ""

	if fqdn != "" {
		zones := strings.Split(strings.Trim(fqdn, "."), ".")
		last := len(zones) - 1
		name = zones[0]

		for i := last; i > -1; i-- {
			zoneID, err := bluecatGetZone(parentViewID, zones[i])
			if err != nil || zoneID == 0 {
				return parentViewID, name, err
			}
			if i > 0 {
				name = strings.Join(zones[0:i], ".")
			}
			parentViewID = zoneID
		}
	}

	return parentViewID, name, nil
}

func bluecatGetZone(parentID uint, name string) (uint, error) {
	queryArgs := map[string]string{
		"parentId": strconv.FormatUint(uint64(parentID), 10),
		"name":     name,
		"type":     "Zone",
	}

	resp, err := bluecatSendRequest(http.MethodGet, "getEntityByName", nil, queryArgs)

	// Return an empty zone if the named zone doesn't exist
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return 0, fmt.Errorf("bluecat: could not find zone named %s", name)
	}
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var zone entityResponse
	err = json.NewDecoder(resp.Body).Decode(&zone)
	if err != nil {
		return 0, fmt.Errorf("bluecat: %w", err)
	}

	return zone.ID, nil
}

func bluecatDeploy(entityID uint) error {
	queryArgs := map[string]string{
		"entityId": strconv.FormatUint(uint64(entityID), 10),
	}

	resp, err := bluecatSendRequest(http.MethodPost, "quickDeploy", nil, queryArgs)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func bluecatSendRequest(method, resource string, payload interface{}, queryArgs map[string]string) (*http.Response, error) {
	url := fmt.Sprintf("%s/Services/REST/v1/%s", baseURL, resource)

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("bluecat: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("bluecat: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if len(token) > 0 {
		req.Header.Set("Authorization", token)
	}

	q := req.URL.Query()
	for argName, argVal := range queryArgs {
		q.Add(argName, argVal)
	}
	req.URL.RawQuery = q.Encode()
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bluecat: %w", err)
	}

	if resp.StatusCode >= 400 {
		errBytes, _ := ioutil.ReadAll(resp.Body)
		errResp := string(errBytes)
		return nil, fmt.Errorf("bluecat: request failed with HTTP status code %d\n Full message: %s",
			resp.StatusCode, errResp)
	}

	return resp, nil
}
