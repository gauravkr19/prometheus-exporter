package nexus

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// License struct holds the license information fetched from Nexus
type License struct {
	ContactEmail    string `json:"contactEmail"`
	ContactCompany  string `json:"contactCompany"`
	ContactName     string `json:"contactName"`
	EffectiveDate   string `json:"effectiveDate"`
	ExpirationDate  string `json:"expirationDate"`
	LicenseType     string `json:"licenseType"`
	LicensedUsers   string `json:"licensedUsers"`
	Features        string `json:"features"`
	DaysUntilExpiry int    `json:"daysUntilExpiry"`
}

// Prometheus metrics
var (
	licenseMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "nexus_license_info",
			Help: "Nexus License Information",
		},
		[]string{
			"contact_email",
			"contact_company",
			"contact_name",
			"effective_date",
			"expiration_date",
			"license_type",
			"licensed_users",
			"features",
		},
	)
	daysUntilExpiryMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "nexus_license_days_until_expiry",
			Help: "Days until Nexus License expires",
		},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(licenseMetric)
	prometheus.MustRegister(daysUntilExpiryMetric)
}

// NewLicense creates a new License instance.
func NewLicense(license License) License {
	expirationDate, _ := time.Parse(time.RFC3339, license.ExpirationDate)
	daysUntilExpiry := int(time.Until(expirationDate).Hours() / 24)

	return License{
		ContactEmail:    license.ContactEmail,
		ContactCompany:  license.ContactCompany,
		ContactName:     license.ContactName,
		EffectiveDate:   license.EffectiveDate,
		ExpirationDate:  license.ExpirationDate,
		LicenseType:     license.LicenseType,
		LicensedUsers:   license.LicensedUsers,
		Features:        license.Features,
		DaysUntilExpiry: daysUntilExpiry,
	}
}

// RegisterMetrics registers license information as Prometheus metrics.
func RegisterMetrics(license License) {
	// Set the license information metric
	licenseMetric.With(prometheus.Labels{
		"contact_email":   license.ContactEmail,
		"contact_company": license.ContactCompany,
		"contact_name":    license.ContactName,
		"effective_date":  license.EffectiveDate,
		"expiration_date": license.ExpirationDate,
		"license_type":    license.LicenseType,
		"licensed_users":  license.LicensedUsers,
		"features":        license.Features,
	}).Set(1)

	// Set the days until expiry metric
	daysUntilExpiryMetric.Set(float64(license.DaysUntilExpiry))
}
