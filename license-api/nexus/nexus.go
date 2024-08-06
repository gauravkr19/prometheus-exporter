package nexus

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

// Config holds the configuration for Nexus client
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
}

// SetupNexus setsup nexus client
func SetupNexus() (*http.Client, Config) {
	nexusConfig := Config{
		URL:      os.Getenv("NEXUS_URL"),
		Username: os.Getenv("NEXUS_USERNAME"),
		Password: os.Getenv("NEXUS_PASSWORD"),
		Insecure: true,
	}
	nexusClient := NewClient(nexusConfig)
	return nexusClient, nexusConfig
}

// NewClient creates a new Nexus client
func NewClient(config Config) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Insecure},
	}
	return &http.Client{Transport: transport}
}

// UpdateNexusLicense fetches and updates the Nexus license metrics
func UpdateNexusLicense(client *http.Client, config Config) {
	license, err := GetLicense(client, config)
	if err != nil {
		log.Printf("Failed to fetch Nexus license: %v", err)
		return
	}

	// Create a License instance and calculate DaysUntilExpiration
	licenseInfo := NewLicense(license)

	RegisterMetrics(licenseInfo)
}

// GetLicense fetches the license information from Nexus
func GetLicense(client *http.Client, config Config) (License, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/service/rest/v1/system/license", config.URL), nil)
	if err != nil {
		return License{}, err
	}
	req.SetBasicAuth(config.Username, config.Password)

	resp, err := client.Do(req)
	if err != nil {
		return License{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return License{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return License{}, err
	}

	var license License
	if err := json.Unmarshal(body, &license); err != nil {
		return License{}, err
	}

	// Calculate days until expiry
	expiryDate, err := time.Parse(time.RFC3339, license.ExpirationDate)
	if err != nil {
		return License{}, err
	}
	license.DaysUntilExpiry = int(time.Until(expiryDate).Hours() / 24)

	return license, nil
}
