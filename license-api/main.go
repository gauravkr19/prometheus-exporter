package main

import (
	"log"

	"net/http"
	"time"

	"github.com/gauravkr19/prometheus-exporters/gitlab"
	"github.com/gauravkr19/prometheus-exporters/nexus"
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

	// Nexus setup
	nexusClient, nexusConfig := nexus.SetupNexus()

	go StartPrometheusEndpoint()

	// Initial license check
	gitlab.UpdateGitlabLicense(gitClient)
	go nexus.UpdateNexusLicense(nexusClient, nexusConfig)

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

			if token.TokenExpiryDays() <= 2 {
				gitlab.RotateTokenAndSetExpiry(gitClient, vaultClient, &token)
				gitlabToken = gitlab.ReadVaultKV2(vaultClient, vaultKVPath)

				gitClient, _ = gitlab.CreateGitLabClient(gitlabToken.Token)

			}

			gitlab.UpdateGitlabLicense(gitClient)
			go nexus.UpdateNexusLicense(nexusClient, nexusConfig)
		}
	}
}
