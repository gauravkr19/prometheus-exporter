package gitlab

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"os"

	"github.com/hashicorp/vault/api"
	"github.com/xanzy/go-gitlab"
)

// Token represents the token structure.
type Token struct {
	ID        int
	ExpiresAt string
	Active    bool
	Token     string
}

// func ReadVaultKV2 reads from Vault KV2 backend
func ReadVaultKV2(vaultClient *api.Client, path string) *Token {
	serviceAccountToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	if err != nil {
		log.Fatalf("Error reading service account token: %v", err)
	}

	// Authenticate with Vault using the Kubernetes/JWT auth method
	authPath := os.Getenv("authPath")
	authData := map[string]interface{}{
		"role": os.Getenv("authRole"),
		"jwt":  string(serviceAccountToken),
	}

	secret, err := vaultClient.Logical().Write(authPath, authData)
	if err != nil {
		log.Fatalf("Error authenticating with Vault: %v", err)
	}

	// Set the Vault token from the authentication response
	vaultClient.SetToken(secret.Auth.ClientToken)

	// Read the secret from the KV2 backend
	secret, err = vaultClient.Logical().Read(path)
	if err != nil {
		log.Fatalf("Error reading Vault KV2: %v", err)
	}
	data := secret.Data["data"].(map[string]interface{})

	var id int
	switch v := data["id"].(type) {
	case string:
		id, err = strconv.Atoi(v)
		if err != nil {
			log.Fatalf("Error converting id string to int: %v", err)
		}
	case float64:
		id = int(v)
	case json.Number:
		id, err = strconv.Atoi(v.String())
		if err != nil {
			log.Fatalf("Error converting id json.Number to int: %v", err)
		}
	default:
		log.Fatalf("Error converting id to int: unexpected type %T", v)
	}

	expiresAt, ok := data["expires_at"].(string)
	if !ok {
		log.Fatalf("Error converting expires_at to string")
	}

	var active bool
	switch v := data["active"].(type) {
	case string:
		active, err = strconv.ParseBool(v)
		if err != nil {
			log.Fatalf("Error converting active string to bool: %v", err)
		}
	case bool:
		active = v
	default:
		log.Fatalf("Error converting active to bool: unexpected type %T", v)
	}

	token, ok := data["token"].(string)
	if !ok {
		log.Fatalf("Error converting token to string")
	}

	return &Token{
		ID:        id,
		ExpiresAt: expiresAt,
		Active:    active,
		Token:     token,
	}
}

// SetupGitLab sets up Vault and reads the GitLab token from Vault.
func SetupGitLab() (*gitlab.Client, *Token, *api.Client, string) {
	vaultClient := CreateVaultClient()
	vaultKVPath := os.Getenv("VAULT_PATH")
	gitlabToken := ReadVaultKV2(vaultClient, vaultKVPath)

	gitClient, err := CreateGitLabClient(gitlabToken.Token)
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}

	// Return gitClient, gitlabToken, and vaultClient
	return gitClient, gitlabToken, vaultClient, vaultKVPath
}

// Create a Vault client
func CreateVaultClient() *api.Client {
	config := api.DefaultConfig()
	config.Address = os.Getenv("VAULT_URL")
	config.ConfigureTLS(&api.TLSConfig{Insecure: true})

	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("Error creating Vault client: %v", err)
	}
	return client
}

// gitClient to be created with every token refresh
func CreateGitLabClient(token string) (*gitlab.Client, error) {
	// Create a custom HTTP transport with InsecureSkipVerify set to true
	httpTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Create a custom HTTP client with the custom transport
	httpClient := &http.Client{
		Transport: httpTransport,
	}

	// Create a new GitLab client with the custom HTTP client
	gitClient, err := gitlab.NewClient(token,
		gitlab.WithBaseURL(os.Getenv("GITLAB_URL")),
		gitlab.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	return gitClient, nil
}

// TokenExpiryDays calculates the number of days until the token expires
func (t *Token) TokenExpiryDays() int {
	expiryTime, err := time.Parse("2006-01-02", t.ExpiresAt)

	if err != nil {
		log.Fatalf("Error parsing ExpiresAt date: %v", err)
	}
	daysUntilExpiry := int(time.Until(expiryTime).Hours() / 24)
	return daysUntilExpiry
}

// UpdateGitlabLicense fetches and registers GitLab license information.
func UpdateGitlabLicense(gitClient *gitlab.Client) {
	// Get license information
	license, _, err := gitClient.License.GetLicense()
	if err != nil {
		log.Fatalf("Failed to get license: %v", err)
	}

	// Create a License instance and calculate DaysUntilExpiration
	licenseInfo := NewLicense(license)

	RegisterMetrics(licenseInfo)
}

// RotateTokenAndSetExpiry rotates the GitLab token and updates its expiry.
func RotateTokenAndSetExpiry(gitClient *gitlab.Client, vaultClient *api.Client, token *Token) {

	glTokenExpiryDays := 90
	if envGLTokenExpiryDays, exists := os.LookupEnv("GL_TOKEN_EXPIRY_DAYS"); exists {
		if val, err := strconv.Atoi(envGLTokenExpiryDays); err == nil {
			glTokenExpiryDays = val
		}
	}

	newExpiryDate := gitlab.ISOTime(time.Now().AddDate(0, 0, glTokenExpiryDays))
	rotateOptions := &gitlab.RotatePersonalAccessTokenOptions{
		ExpiresAt: &newExpiryDate,
	}

	// Rotate the personal access token, expires_at can be set for Gitlab vers 16.6 onwards
	rotatedToken, _, err := gitClient.PersonalAccessTokens.RotatePersonalAccessTokenByID(token.ID, rotateOptions)
	if err != nil {
		log.Fatalf("Failed to rotate personal access token: %v", err)
	}
	if err != nil {
		log.Fatalf("Error rotating token: %v", err)
	}

	log.Printf("Successfully rotated Gitlab token")
	WriteTokenToVault(vaultClient, rotatedToken)
}

// WriteTokenToVault writes the new token to Vault.
func WriteTokenToVault(client *api.Client, rotatedToken *gitlab.PersonalAccessToken) {
	vaultKVPath := os.Getenv("VAULT_PATH")
	data := map[string]interface{}{
		"id":         rotatedToken.ID,
		"expires_at": rotatedToken.ExpiresAt.String(),
		"active":     rotatedToken.Active,
		"token":      rotatedToken.Token,
	}
	// Wrap data map inside another map with key "data" for KV-v2
	payload := map[string]interface{}{
		"data": data,
	}

	secret, err := client.Logical().Write(vaultKVPath, payload)
	if err != nil {
		log.Fatalf("Error writing token to Vault KV2: %v", err)
	}
	log.Printf("Write response from Vault: %v", secret)
	log.Printf("Successfully wrote Gitlab token to Vault backend")
}
