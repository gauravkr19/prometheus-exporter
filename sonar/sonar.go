package sonar

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const layout = "2006-01-02" // As the ExpiresAt format does not comply with time.Time "2006-01-02" format, ie. without time/TZ
type Time struct {
	time.Time
}

// License struct holds the license information fetched from sonar
type License struct {
	ExpiresAt              Time   `json:"expiresAt"`
	IsExpired              bool   `json:"isExpired"`
	Edition                string `json:"edition"`
	IsValidEdition         bool   `json:"isValidEdition"`
	MaxLoC                 int    `json:"maxLoc"`
	LoC                    int    `json:"loc"`
	IsOfficialDistribution bool   `json:"isOfficialDistribution"`
	IsSupported            bool   `json:"isSupported"`
	RemainingLocThreshold  int    `json:"remainingLocThreshold"`
	DaysUntilExpiry        int    `json:"daysUntilExpiry"`
}

// Config holds the configuration for sonar client
type Config struct {
	URL      string
	Username string
	Password string
	Insecure bool
}

// Setupsonar setsup sonar client
func SetupSonar() (*http.Client, Config) {
	sonarConfig := Config{
		URL:      os.Getenv("SONAR_URL"),
		Username: os.Getenv("SONAR_USERNAME"),
		Password: os.Getenv("SONAR_PASSWORD"),
		Insecure: true,
	}
	sonarClient := NewClient(sonarConfig)
	return sonarClient, sonarConfig
}

// NewClient creates a new sonar client
func NewClient(config Config) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: config.Insecure},
	}
	return &http.Client{Transport: transport}
}

// UpdateSonarLicense fetches and updates the Sonar license metrics
func UpdateSonarLicense(client *http.Client, config Config) {
	license, err := GetLicense(client, config)
	if err != nil {
		log.Printf("Failed to fetch Sonar license: %v", err)
		return
	}
	licenseInfo := NewLicense(license)

	RegisterMetrics(licenseInfo)
}

// GetLicense fetches the license information from Sonar
func GetLicense(client *http.Client, config Config) (License, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/editions/show_license", config.URL), nil)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return License{}, err
	}

	var license License
	if err := json.Unmarshal(body, &license); err != nil {
		return License{}, err
	}

	// Calculate days until expiry
	license.DaysUntilExpiry = int(time.Until(license.ExpiresAt.Time).Hours() / 24)

	return license, nil
}

// NewLicense creates a new License instance.
func NewLicense(license License) License {
	var daysUntilExpiry int

	// Check if ExpiresAt is a zero time (default uninitialized state)
	if !license.ExpiresAt.IsZero() {
		daysUntilExpiry = int(time.Until(license.ExpiresAt.Time).Hours() / 24)
	}
	license.DaysUntilExpiry = daysUntilExpiry

	return License{
		ExpiresAt:              license.ExpiresAt,
		IsExpired:              license.IsExpired,
		Edition:                license.Edition,
		IsValidEdition:         license.IsValidEdition,
		MaxLoC:                 license.MaxLoC,
		LoC:                    license.LoC,
		IsOfficialDistribution: license.IsOfficialDistribution,
		IsSupported:            license.IsSupported,
		RemainingLocThreshold:  license.RemainingLocThreshold,
		DaysUntilExpiry:        daysUntilExpiry,
	}
}

// UnmarshalJSON deals with issue in parsing ExpiresAt which is nil with *time.Time type due to "2006-01-02" format, ie. without time/TZ
func (ct *Time) UnmarshalJSON(b []byte) error {
	str := strings.Trim(string(b), `"`)

	t, err := time.Parse(layout, str)
	if err != nil {
		return err
	}
	ct.Time = t
	return nil
}
