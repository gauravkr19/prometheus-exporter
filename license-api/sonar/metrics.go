package sonar

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

// Prometheus metrics
var (
	licenseMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "sonar_license_info",
			Help: "sonar License Information",
		},
		[]string{
			"expires_at",
			"is_expired",
			"edition",
			"is_valid_edition",
			"max_loc",
			"loc",
			"is_official_distribution",
			"is_supported",
			"remaining_loc_threshold",
		},
	)
	daysUntilExpiryMetric = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "sonar_license_days_until_expiry",
			Help: "Days until Sonar License expires",
		},
	)
)

func init() {
	// Register metrics with Prometheus
	prometheus.MustRegister(licenseMetric)
	prometheus.MustRegister(daysUntilExpiryMetric)
}

// RegisterMetrics registers license information as Prometheus metrics.
func RegisterMetrics(license License) {
	licenseMetric.With(prometheus.Labels{
		"expires_at":               license.ExpiresAt.String(),
		"is_expired":               fmt.Sprint(license.IsExpired),
		"edition":                  license.Edition,
		"is_valid_edition":         fmt.Sprint(license.IsValidEdition),
		"max_loc":                  fmt.Sprint(license.MaxLoC),
		"loc":                      fmt.Sprint(license.LoC),
		"is_official_distribution": fmt.Sprint(license.IsOfficialDistribution),
		"is_supported":             fmt.Sprint(license.IsSupported),
		"remaining_loc_threshold":  fmt.Sprint(license.RemainingLocThreshold),
	}).Set(1)

	// Set the days until expiry metric
	daysUntilExpiryMetric.Set(float64(license.DaysUntilExpiry))
}
