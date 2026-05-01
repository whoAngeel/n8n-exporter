// Package credentials handles interactive collection and secure storage
// of n8n instance credentials. Credentials are encrypted with AES-256-GCM
// and stored in the user's config directory. Nothing is written to env vars.
package credentials

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AuthType distinguishes between Basic Auth (user+password) and API Token.
type AuthType int

const (
	AuthTypeBasic AuthType = iota // username + password
	AuthTypeToken                 // API token (X-N8N-API-KEY)
)

// Credentials holds all authentication data for a single session.
type Credentials struct {
	BaseURL  string // e.g. "https://n8n.example.com" (no trailing slash)
	AuthType AuthType
	Username string // only for AuthTypeBasic
	Password string // only for AuthTypeBasic
	Token    string // only for AuthTypeToken
}

// CollectCredentials returns credentials for the session.
//
// If a saved credentials file exists, it prompts only for the passphrase
// and decrypts the stored credentials. On first run (or after --reset),
// it prompts for all fields and offers to save them encrypted to disk.
func CollectCredentials() (Credentials, error) {
	reader := bufio.NewReader(os.Stdin)

	// ── Fast path: saved credentials exist ───────────────────────────────────
	if CredentialsFileExists() {
		fmt.Print("Passphrase: ")
		passphrase, err := reader.ReadString('\n')
		if err != nil {
			return Credentials{}, fmt.Errorf("reading passphrase: %w", err)
		}
		passphrase = strings.TrimSpace(passphrase)
		if passphrase == "" {
			return Credentials{}, fmt.Errorf("passphrase cannot be empty")
		}

		creds, err := LoadCredentials(passphrase)
		if err != nil {
			return Credentials{}, err // "wrong passphrase or corrupted file"
		}
		return creds, nil
	}

	// ── First run: collect all fields ────────────────────────────────────────
	fmt.Print("n8n instance URL (e.g. https://n8n.example.com): ")
	rawURL, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading URL: %w", err)
	}
	baseURL := strings.TrimRight(strings.TrimSpace(rawURL), "/")
	if baseURL == "" {
		return Credentials{}, fmt.Errorf("instance URL cannot be empty")
	}

	fmt.Print("Auth type — enter 1 for Basic (user/password) or 2 for API Token: ")
	authChoice, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading auth type: %w", err)
	}
	authChoice = strings.TrimSpace(authChoice)

	var creds Credentials
	creds.BaseURL = baseURL

	switch authChoice {
	case "1":
		fmt.Print("Username: ")
		username, err := reader.ReadString('\n')
		if err != nil {
			return Credentials{}, fmt.Errorf("reading username: %w", err)
		}
		username = strings.TrimSpace(username)
		if username == "" {
			return Credentials{}, fmt.Errorf("username cannot be empty")
		}

		fmt.Print("Password: ")
		password, err := reader.ReadString('\n')
		if err != nil {
			return Credentials{}, fmt.Errorf("reading password: %w", err)
		}
		password = strings.TrimSpace(password)
		if password == "" {
			return Credentials{}, fmt.Errorf("password cannot be empty")
		}

		creds.AuthType = AuthTypeBasic
		creds.Username = username
		creds.Password = password

	case "2":
		fmt.Print("API Token: ")
		token, err := reader.ReadString('\n')
		if err != nil {
			return Credentials{}, fmt.Errorf("reading token: %w", err)
		}
		token = strings.TrimSpace(token)
		if token == "" {
			return Credentials{}, fmt.Errorf("API token cannot be empty")
		}

		creds.AuthType = AuthTypeToken
		creds.Token = token

	default:
		return Credentials{}, fmt.Errorf("invalid auth type %q: enter 1 or 2", authChoice)
	}

	// ── Offer to save credentials ─────────────────────────────────────────────
	fmt.Print("\nSave credentials for future sessions? (y/N): ")
	saveChoice, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading save choice: %w", err)
	}
	saveChoice = strings.TrimSpace(strings.ToLower(saveChoice))

	if saveChoice == "y" || saveChoice == "yes" {
		fmt.Print("Choose a passphrase to protect your credentials: ")
		passphrase, err := reader.ReadString('\n')
		if err != nil {
			return Credentials{}, fmt.Errorf("reading passphrase: %w", err)
		}
		passphrase = strings.TrimSpace(passphrase)
		if passphrase == "" {
			fmt.Println("⚠  Passphrase cannot be empty — credentials not saved.")
		} else {
			if err := SaveCredentials(creds, passphrase); err != nil {
				fmt.Printf("⚠  Could not save credentials: %v\n", err)
			} else {
				fmt.Println("✓  Credentials saved.")
			}
		}
	}

	return creds, nil
}
