package main

import (
	"log"
	"os"
	"strconv"

	"net/http"
	"time"

	"github.com/gauravkr19/prometheus-exporters/gitlab"
	"github.com/gauravkr19/prometheus-exporters/nexus"
	"github.com/gauravkr19/prometheus-exporters/sonar"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start Prometheus endpoint
func StartPrometheusEndpoint() {
	http.Handle("/metrics", promhttp.Handler())
	log.Println("Starting License exporter server at :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}

func main() {
	gitClient, gitlabToken, vaultClient, vaultKVPath, err := gitlab.InitGitlabVault()
	if err != nil {
		log.Fatalf("GitLab setup failed: %v", err)
	}

	nexusClient, nexusConfig := nexus.SetupNexus()
	sonarClient, sonarConfig := sonar.SetupSonar()

	go StartPrometheusEndpoint()

	gitlab.UpdateGitlabLicense(gitClient)
	go nexus.UpdateNexusLicense(nexusClient, nexusConfig)
	go sonar.UpdateSonarLicense(sonarClient, sonarConfig)

	probeIntervalHours := 4
	if envProbeIntervalHours, exists := os.LookupEnv("PROBE_INTERVAL_HOURS"); exists {
		if val, err := strconv.Atoi(envProbeIntervalHours); err == nil {
			probeIntervalHours = val
		}
	}

	probeInterval := time.Duration(probeIntervalHours)
	ticker := time.NewTicker(probeInterval * time.Hour)
	defer ticker.Stop()

	const rotateBeforeExpiryDays = 5

	for range ticker.C {
		// gitlabToken is already *gitlab.Token; no need to copy it.
		if gitlabToken.TokenExpiryDays() <= rotateBeforeExpiryDays {			
			rotatedToken, err := gitlab.RotateTokenAndSetExpiry(gitClient, vaultClient)
			if err != nil {
				// rotatedToken may still contain the new PAT when only
				// the Vault write failed.
				log.Printf("GitLab PAT rotation failed: %v", err)
				continue
			}			

			gitlabToken = rotatedToken

			// The old GitLab client contains the revoked token.
			gitClient, err = gitlab.CreateGitLabClient(gitlabToken.Token)
			if err != nil {
				log.Printf("Failed to recreate GitLab client: %v", err)
				continue
			}
		}

		log.Printf("Gitlab Token expiry days: %v", gitlabToken.TokenExpiryDays())
		gitlab.UpdateGitlabLicense(gitClient)
		go nexus.UpdateNexusLicense(nexusClient, nexusConfig)
		go sonar.UpdateSonarLicense(sonarClient, sonarConfig)
	}
	_ = vaultKVPath
}
