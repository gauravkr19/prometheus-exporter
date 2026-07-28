package gitlab

import (
	"crypto/tls"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"fmt"
	"os"

	"github.com/hashicorp/vault/api"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// Token represents the token structure.
type Token struct {
	ID        int // not in use
	ExpiresAt string
	Active    bool // not in use
	Token     string
}

// func ReadVaultKV2 reads from Vault KV2 backend
func ReadVaultKV2(vaultClient *api.Client, path string) (*Token, error) {
	serviceAccountToken, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
	// serviceAccountToken, err := os.ReadFile("/home/coder/proj-license-update/prometheus-exporters/token")
	if err != nil {
		return nil, fmt.Errorf("Error reading the default token file in pod: %w", err)
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
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Vault secret has invalid KV2 data structure")
	}

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
	}, nil
}

// InitGitlabVault sets up Vault and reads the GitLab token from Vault.
func InitGitlabVault() (*gitlab.Client, *Token, *api.Client, string, error) {
	vaultClient, err := CreateVaultClient()
	if err != nil {
		log.Fatalf("vault client creation failed: %v", err)
	}
	vaultKVPath := os.Getenv("VAULT_PATH")
	gitlabToken, err := ReadVaultKV2(vaultClient, vaultKVPath)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to read GitLab token from Vault: %w", err)
	}

	gitClient, err := CreateGitLabClient(gitlabToken.Token)
	if err != nil {
		log.Fatalf("Failed to create GitLab client: %v", err)
	}

	// Return gitClient, gitlabToken, and vaultClient
	return gitClient, gitlabToken, vaultClient, vaultKVPath, nil
}

// Create a Vault client
func CreateVaultClient() (*api.Client, error) {
	config := api.DefaultConfig()
	config.Address = os.Getenv("VAULT_URL")
	config.ConfigureTLS(&api.TLSConfig{Insecure: true})

	client, err := api.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("vault client creation failed: %w", err)
	}
	return client, nil
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

func RotateTokenAndSetExpiry(gitClient *gitlab.Client, vaultClient *api.Client) (*Token, error) {
	expiryDate, err := getTokenExpiryDate()
	log.Printf("Gitlab Token expiry date: %v", expiryDate)
	if err != nil {
		return nil, err
	}

	options := &gitlab.RotatePersonalAccessTokenOptions{ExpiresAt: &expiryDate}

	rotatedPAT, response, err := gitClient.PersonalAccessTokens.RotatePersonalAccessTokenSelf(options)
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("personal access token rotation failed; status=%s: %w", response.Status, err)
		}
		return nil, fmt.Errorf("personal access token rotation failed: %w", err)
	}
	token, err := tokenFromGitLab(rotatedPAT)
	if err != nil {
		return nil, fmt.Errorf(
			"invalid personal access token returned by GitLab: %w",
			err,
		)
	}
	if err := WriteTokenToVault(vaultClient, token); err != nil {
		// Return the rotated token because the previous token has already
		// been revoked and this is the only available replacement.
		return token, fmt.Errorf(
			"PAT was rotated successfully, but Vault update failed: %w",
			err,
		)
	}
	return token, nil
}

func tokenFromGitLab(pat *gitlab.PersonalAccessToken) (*Token, error) {
	if pat == nil {
		return nil, fmt.Errorf("GitLab returned nil PAT")
	}

	if pat.Token == "" {
		return nil, fmt.Errorf("GitLab returned empty PAT value")
	}

	if pat.ExpiresAt == nil {
		return nil, fmt.Errorf("GitLab returned PAT without expiration date")
	}

	return &Token{
		ID:        int(pat.ID),
		ExpiresAt: time.Time(*pat.ExpiresAt).Format("2006-01-02"),
		Active:    pat.Active,
		Token:     pat.Token,
	}, nil
}

// WriteTokenToVault writes the new token to Vault.
func WriteTokenToVault(client *api.Client, rotatedToken *Token) error {
	vaultKVPath := os.Getenv("VAULT_PATH")
	data := map[string]interface{}{
		"id":         rotatedToken.ID,
		"expires_at": rotatedToken.ExpiresAt,
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
	return nil
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

// getTokenExpiryDate processes env GL_TOKEN_EXPIRY
func getTokenExpiryDate() (gitlab.ISOTime, error) {
	const defaultExpiry = "90d"

	value := os.Getenv("GL_TOKEN_EXPIRY")
	if value == "" {
		value = defaultExpiry
	}

	duration, err := parseTokenDuration(value)
	if err != nil {
		return gitlab.ISOTime{}, fmt.Errorf("invalid GL_TOKEN_EXPIRY value %q: %w", value, err)
	}

	expiryTime := time.Now().UTC().Add(duration)

	// GitLab PAT expiry is date-based, so normalize it to a date.
	expiryDate := time.Date(
		expiryTime.Year(),
		expiryTime.Month(),
		expiryTime.Day(),
		0, 0, 0, 0,
		time.UTC,
	)

	return gitlab.ISOTime(expiryDate), nil
}

func parseTokenDuration(value string) (time.Duration, error) {
	if len(value) < 2 {
		return 0, fmt.Errorf("expected a value such as 90d, 34h, or 12w")
	}

	unit := value[len(value)-1:]
	numberText := value[:len(value)-1]

	number, err := strconv.Atoi(numberText)
	if err != nil || number <= 0 {
		return 0, fmt.Errorf("duration must contain a positive whole number")
	}

	switch unit {
	case "h":
		return time.Duration(number) * time.Hour, nil

	case "d":
		return time.Duration(number) * 24 * time.Hour, nil

	case "w":
		return time.Duration(number) * 7 * 24 * time.Hour, nil

	default:
		return 0, fmt.Errorf(
			"unsupported unit %q; supported units are h, d, and w",
			unit,
		)
	}
}
