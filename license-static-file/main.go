package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gopkg.in/yaml.v2"
)

// LicenseInfo represents the structure of the license information
type LicenseInfo struct {
	Licenses []License `yaml:"licenses"`
}

// License represents the structure of each license
type License struct {
	Name               string `yaml:"name"`
	Version            string `yaml:"version"`
	PONumber           string `yaml:"po_number"`
	POExpiryDate       string `yaml:"po_expiry_date"`
	PORenewalOwner     string `yaml:"po_renewal_owner"`
	EOLDate            string `yaml:"eol_date"`
	EOSDate            string `yaml:"eos_date"`
	TotalCapacity      string `yaml:"total_capacity"`
	CurrentUtilization string `yaml:"current_utilization"`
	LicenseExpiryDate  string `yaml:"license_expiry_date"`
	VendorSupport      string `yaml:"vendor_support"`
}

var (
	licenseVersionGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "software_version",
			Help: "Version of the software license",
		},
		[]string{"software", "version"},
	)
	poNumberGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "po_number",
			Help: "Purchase Order Number",
		},
		[]string{"software"},
	)
	poExpiryDateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "po_expiry_days",
			Help: "Days until PO expiration",
		},
		[]string{"software"},
	)
	eolDateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eol_days",
			Help: "Days until End of Life",
		},
		[]string{"software"},
	)
	eosDateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "eos_days",
			Help: "Days until End of Support",
		},
		[]string{"software"},
	)
	totalCapacityGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_capacity",
			Help: "Total Capacity",
		},
		[]string{"software"},
	)
	currentUtilizationGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "current_utilization",
			Help: "Current Utilization",
		},
		[]string{"software"},
	)
	licenseExpiryDateGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "license_expiry_days",
			Help: "Days until License Expiry",
		},
		[]string{"software"},
	)
	vendorSupportGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "vendor_support",
			Help: "Vendor Support Contact",
		},
		[]string{"software", "support_contact"},
	)
	poRenewalOwnerGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "po_renewal_owner",
			Help: "Owners to renew the license",
		},
		[]string{"software", "po_renewal_owner"},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(licenseVersionGauge)
	prometheus.MustRegister(poNumberGauge)
	prometheus.MustRegister(poExpiryDateGauge)
	prometheus.MustRegister(eolDateGauge)
	prometheus.MustRegister(eosDateGauge)
	prometheus.MustRegister(totalCapacityGauge)
	prometheus.MustRegister(currentUtilizationGauge)
	prometheus.MustRegister(licenseExpiryDateGauge)
	prometheus.MustRegister(vendorSupportGauge)
	prometheus.MustRegister(poRenewalOwnerGauge)
}

func readLicenseInfo(filePath string) (LicenseInfo, error) {
	var licenseInfo LicenseInfo
	data, err := os.ReadFile(filePath)
	if err != nil {
		return licenseInfo, err
	}
	err = yaml.Unmarshal(data, &licenseInfo)
	return licenseInfo, err
}

func parseDate(dateStr string) (float64, error) {
	if dateStr == "NA" || dateStr == "" {
		return 0, fmt.Errorf("invalid date")
	}
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, err
	}
	return time.Until(date).Hours() / 24, nil
}

func parseFloat(str string) (float64, error) {
	if str == "" {
		return 0, fmt.Errorf("invalid number")
	}
	return strconv.ParseFloat(str, 64)
}

func updateMetrics(licenses []License, invalidDateLog map[string]bool) {
	for _, license := range licenses {
		name := license.Name
		version := license.Version
		// poRenewalOwner := license.PORenewalOwner

		// Parse numerical fields
		poNumber, _ := parseFloat(license.PONumber)
		totalCapacity, _ := parseFloat(license.TotalCapacity)
		currentUtilization, _ := parseFloat(license.CurrentUtilization)

		// Parse date fields
		daysUntilPOExpiry, err := parseDate(license.POExpiryDate)
		if err != nil && !invalidDateLog[license.POExpiryDate] {
			log.Printf("Invalid PO expiry date for %s: %v", name, err)
			invalidDateLog[license.POExpiryDate] = true
		}
		daysUntilEOL, err := parseDate(license.EOLDate)
		if err != nil && !invalidDateLog[license.EOLDate] {
			log.Printf("Invalid EOL date for %s: %v", name, err)
			invalidDateLog[license.EOLDate] = true
		}
		daysUntilEOS, err := parseDate(license.EOSDate)
		if err != nil && !invalidDateLog[license.EOSDate] {
			log.Printf("Invalid EOS date for %s: %v", name, err)
			invalidDateLog[license.EOSDate] = true
		}
		daysUntilLicenseExpiry, err := parseDate(license.LicenseExpiryDate)
		if err != nil && !invalidDateLog[license.LicenseExpiryDate] {
			log.Printf("Invalid license expiry date for %s: %v", name, err)
			invalidDateLog[license.LicenseExpiryDate] = true
		}

		// Update Prometheus metrics
		licenseVersionGauge.WithLabelValues(name, version).Set(1) // with label
		poNumberGauge.WithLabelValues(name).Set(poNumber)
		poExpiryDateGauge.WithLabelValues(name).Set(daysUntilPOExpiry)
		eolDateGauge.WithLabelValues(name).Set(daysUntilEOL)
		eosDateGauge.WithLabelValues(name).Set(daysUntilEOS)
		totalCapacityGauge.WithLabelValues(name).Set(totalCapacity)
		currentUtilizationGauge.WithLabelValues(name).Set(currentUtilization)
		licenseExpiryDateGauge.WithLabelValues(name).Set(daysUntilLicenseExpiry)
		// poRenewalOwnerGauge.WithLabelValues(name).Set(poRenewalOwner)
		poRenewalOwnerGauge.WithLabelValues(name, license.PORenewalOwner).Set(1)
		vendorSupportGauge.WithLabelValues(name, license.VendorSupport).Set(1) // with label
	}
}

func main() {
	// Configuration file path
	configFile := os.Getenv("LICENSE_CONFIG_PATH")
	if configFile == "" {
		configFile = "license_info.yaml"
	}

	// Start Prometheus server
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Println("Starting server at :8000")
		log.Fatal(http.ListenAndServe(":8000", nil))
	}()

	// Map to keep track of logged invalid dates
	invalidDateLog := make(map[string]bool)

	// Update metrics periodically
	for {
		licenseInfo, err := readLicenseInfo(configFile)
		if err != nil {
			log.Printf("Error reading license info: %v", err)
		} else {
			updateMetrics(licenseInfo.Licenses, invalidDateLog)
		}
		time.Sleep(60 * time.Second)
	}
}
