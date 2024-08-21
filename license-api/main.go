package main

import (
	"log"

	"time"
    "net/http"

	"github.com/gauravkr19/prometheus-exporters/gitlab"
	"github.com/gauravkr19/prometheus-exporters/nexus"
	"github.com/gauravkr19/prometheus-exporters/sonar"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Start Prometheus endpoint
func StartPrometheusEndpoint() {
	http.Handle("/metrics", promhttp.Handler())
    log.Println("Starting License exporter server at :8081")
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func main() {
	// GitLab setup
	gitClient, gitlabToken, vaultClient, vaultKVPath := gitlab.SetupGitLab()
	
    // Nexus.Sonar setup
	nexusClient, nexusConfig := nexus.SetupNexus()
	sonarClient, sonarConfig := sonar.SetupSonar()

    go StartPrometheusEndpoint()

	// Initial license check
	gitlab.UpdateGitlabLicense(gitClient)
    go nexus.UpdateNexusLicense(nexusClient, nexusConfig)
    go sonar.UpdateSonarLicense(sonarClient, sonarConfig)

	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			token := gitlab.Token{
				ID:        gitlabToken.ID,
				ExpiresAt: gitlabToken.ExpiresAt,
				Active:    gitlabToken.Active,
				Token:     gitlabToken.Token,
			}

			if token.TokenExpiryDays() <= 0 {
				gitlab.RotateTokenAndSetExpiry(gitClient, vaultClient, &token)
				gitlabToken = gitlab.ReadVaultKV2(vaultClient, vaultKVPath)

				gitClient, _ = gitlab.CreateGitLabClient(gitlabToken.Token)

			}

			gitlab.UpdateGitlabLicense(gitClient)
            go nexus.UpdateNexusLicense(nexusClient, nexusConfig)
            go sonar.UpdateSonarLicense(sonarClient, sonarConfig)
		}
	}
}

