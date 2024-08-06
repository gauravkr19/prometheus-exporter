package gitlab

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/xanzy/go-gitlab"
)

// License represents the structure of the license information
type License struct {
	ID               int             `json:"id"`
	Plan             string          `json:"plan"`
	CreatedAt        *time.Time      `json:"created_at"`
	StartsAt         *gitlab.ISOTime `json:"starts_at"`
	ExpiresAt        *gitlab.ISOTime `json:"expires_at"`
	HistoricalMax    int             `json:"historical_max"`
	MaximumUserCount int             `json:"maximum_user_count"`
	Licensee         Licensee        `json:"licensee"`
	AddOns           AddOns          `json:"add_ons"`
	Expired          bool            `json:"expired"`
	Overage          int             `json:"overage"`
	UserLimit        int             `json:"user_limit"`
	ActiveUsers      int             `json:"active_users"`
	DaysUntilExpiry  int
	RemainingUsers   int
}

// Licensee represents the licensee information
type Licensee struct {
	Name    string `json:"Name"`
	Email   string `json:"Email"`
	Company string `json:"Company"`
}

// AddOns represents the add-ons information
type AddOns struct{}

// Prometheus metrics
var (
	licenseMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gitlab_license",
			Help: "License information from GitLab",
		},
		[]string{"plan", "created_at", "starts_at", "expires_at", "historical_max", "maximum_user_count", "licensee_name", "licensee_email", "licensee_company", "add_ons", "expired", "overage", "user_limit", "active_users", "days_until_expiry", "remaining_users"},
	)

	daysUntilExpiryMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "gitlab_license_days_until_expiry",
			Help: "Days until Gitlab License expires",
		},
	)
)

func init() {
	// Register the custom metrics with Prometheus's default registry
	prometheus.MustRegister(licenseMetric)
	prometheus.MustRegister(daysUntilExpiryMetric)
}

// func NewLicense recreates License struct to add additional label daysUntilExpiration and convert ISOTime to time.Time
func NewLicense(license *gitlab.License) License {
	expirationTime := time.Time(*license.ExpiresAt) // Convert ISOTime to time.Time
	daysUntilExpiration := 0

	if expirationTime.After(time.Now()) {
		daysUntilExpiration = int(expirationTime.Sub(time.Now()).Hours() / 24)
	}

	return License{
		ID:               license.ID,
		Plan:             license.Plan,
		CreatedAt:        license.CreatedAt,
		StartsAt:         license.StartsAt,
		ExpiresAt:        license.ExpiresAt,
		HistoricalMax:    license.HistoricalMax,
		MaximumUserCount: license.MaximumUserCount,
		Licensee:         Licensee{Name: license.Licensee.Name, Email: license.Licensee.Email, Company: license.Licensee.Company},
		AddOns:           AddOns{},
		Expired:          license.Expired,
		Overage:          license.Overage,
		UserLimit:        license.UserLimit,
		ActiveUsers:      license.ActiveUsers,
		DaysUntilExpiry:  daysUntilExpiration,
		RemainingUsers:   license.UserLimit - license.ActiveUsers,
	}
}

// RegisterMetrics registers license information as Prometheus metrics.
func RegisterMetrics(license License) {
	licenseMetric.With(prometheus.Labels{
		"plan":               license.Plan,
		"created_at":         license.CreatedAt.String(),
		"starts_at":          license.StartsAt.String(),
		"expires_at":         license.ExpiresAt.String(),
		"historical_max":     fmt.Sprint(license.HistoricalMax),
		"maximum_user_count": fmt.Sprint(license.MaximumUserCount),
		"licensee_name":      license.Licensee.Name,
		"licensee_email":     license.Licensee.Email,
		"licensee_company":   license.Licensee.Company,
		"add_ons":            fmt.Sprint(license.AddOns),
		"expired":            fmt.Sprint(license.Expired),
		"overage":            fmt.Sprint(license.Overage),
		"user_limit":         fmt.Sprint(license.UserLimit),
		"active_users":       fmt.Sprint(license.ActiveUsers),
		"days_until_expiry":  fmt.Sprint(license.DaysUntilExpiry),
		"remaining_users":    fmt.Sprint(license.RemainingUsers),
	}).Set(1)

	daysUntilExpiryMetric.Set(float64(license.DaysUntilExpiry))
}
