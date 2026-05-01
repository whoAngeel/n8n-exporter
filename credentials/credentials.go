// Package credentials handles interactive collection and secure storage
// of n8n instance credentials. Credentials are encrypted with AES-256-GCM
// and stored in the user's config directory. Nothing is written to env vars.
package credentials

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// AuthType distinguishes between Basic Auth (user+password) and API Token.
type AuthType int

const (
	AuthTypeBasic AuthType = iota // username + password
	AuthTypeToken                 // API token (X-N8N-API-KEY)
)

// Credentials holds all authentication data for a single session.
type Credentials struct {
	BaseURL   string // e.g. "https://n8n.example.com" (no trailing slash)
	AuthType  AuthType
	Username  string // only for AuthTypeBasic
	Password  string // only for AuthTypeBasic
	Token     string // only for AuthTypeToken
	OutputDir string // absolute path to the export directory
}

// readMasked reads a sensitive value from stdin without echoing characters.
// Falls back to plain bufio read if the terminal doesn't support raw mode
// (e.g. when piping input in tests).
func readMasked(prompt string) (string, error) {
	fmt.Print(prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println() // move to next line after hidden input
	if err != nil {
		// Fallback for non-TTY environments (pipes, CI, tests).
		reader := bufio.NewReader(os.Stdin)
		line, err2 := reader.ReadString('\n')
		if err2 != nil {
			return "", fmt.Errorf("reading input: %w", err2)
		}
		return strings.TrimSpace(line), nil
	}
	return strings.TrimSpace(string(b)), nil
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
		passphrase, err := readMasked("Passphrase: ")
		if err != nil {
			return Credentials{}, fmt.Errorf("reading passphrase: %w", err)
		}
		if passphrase == "" {
			return Credentials{}, fmt.Errorf("passphrase cannot be empty")
		}

		creds, err := LoadCredentials(passphrase)
		if err != nil {
			return Credentials{}, err
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

		password, err := readMasked("Password: ")
		if err != nil {
			return Credentials{}, fmt.Errorf("reading password: %w", err)
		}
		if password == "" {
			return Credentials{}, fmt.Errorf("password cannot be empty")
		}

		creds.AuthType = AuthTypeBasic
		creds.Username = username
		creds.Password = password

	case "2":
		token, err := readMasked("API Token: ")
		if err != nil {
			return Credentials{}, fmt.Errorf("reading token: %w", err)
		}
		if token == "" {
			return Credentials{}, fmt.Errorf("API token cannot be empty")
		}

		creds.AuthType = AuthTypeToken
		creds.Token = token

	default:
		return Credentials{}, fmt.Errorf("invalid auth type %q: enter 1 or 2", authChoice)
	}

	// ── Offer to save credentials ─────────────────────────────────────────────
	fmt.Print("\nOutput directory for exports\n(leave empty to use current directory): ")
	rawDir, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading output directory: %w", err)
	}
	outputDir := strings.TrimSpace(rawDir)
	if outputDir == "" {
		// Default: current working directory
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		outputDir = filepath.Join(cwd, "#n8n_workflows_original")
	} else {
		// Expand ~ to home directory if present
		if strings.HasPrefix(outputDir, "~") {
			home, err := os.UserHomeDir()
			if err == nil {
				outputDir = filepath.Join(home, outputDir[1:])
			}
		}
		outputDir = filepath.Clean(outputDir)
	}
	creds.OutputDir = outputDir
	fmt.Printf("  → Export directory: %s\n", outputDir)

	fmt.Print("\nSave credentials for future sessions? (y/N): ")
	saveChoice, err := reader.ReadString('\n')
	if err != nil {
		return Credentials{}, fmt.Errorf("reading save choice: %w", err)
	}
	saveChoice = strings.TrimSpace(strings.ToLower(saveChoice))

	if saveChoice == "y" || saveChoice == "yes" {
		passphrase, err := readMasked("Choose a passphrase to protect your credentials: ")
		if err != nil {
			return Credentials{}, fmt.Errorf("reading passphrase: %w", err)
		}
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
