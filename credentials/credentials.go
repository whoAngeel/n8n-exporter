// Package credentials handles interactive collection and secure storage
// of n8n instance credentials. Credentials are encrypted with AES-256-GCM
// and stored in the user's config directory. Nothing is written to env vars.
package credentials

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
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

// CollectCredentials returns credentials for the session.
//
// If a saved credentials file exists, it prompts only for the passphrase
// and decrypts the stored credentials. On first run (or after --reset),
// it prompts for all fields and offers to save them encrypted to disk.
func CollectCredentials() (Credentials, error) {
	// ── Fast path: saved credentials exist ───────────────────────────────────
	if CredentialsFileExists() {
		return collectFromSaved()
	}
	return collectFromScratch()
}

// collectFromSaved decrypts previously saved credentials using a passphrase.
func collectFromSaved() (Credentials, error) {
	var passphrase string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Passphrase").
				Description("Enter the passphrase that protects your saved credentials").
				EchoMode(huh.EchoModePassword).
				Value(&passphrase).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("passphrase cannot be empty")
					}
					return nil
				}),
		),
	).Run()
	if err != nil {
		return Credentials{}, fmt.Errorf("passphrase prompt: %w", err)
	}
	return LoadCredentials(strings.TrimSpace(passphrase))
}

// collectFromScratch prompts for all credential fields on first run.
func collectFromScratch() (Credentials, error) {
	// ── Step 1: URL + auth type ───────────────────────────────────────────────
	var baseURL string
	var authChoice string

	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("n8n Instance URL").
				Placeholder("https://n8n.example.com").
				Value(&baseURL).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("URL cannot be empty")
					}
					return nil
				}),
			huh.NewSelect[string]().
				Title("Authentication Type").
				Options(
					huh.NewOption("Basic Auth (username + password)", "basic"),
					huh.NewOption("API Token (X-N8N-API-KEY)", "token"),
				).
				Value(&authChoice),
		),
	).Run()
	if err != nil {
		return Credentials{}, fmt.Errorf("connection form: %w", err)
	}

	var creds Credentials
	creds.BaseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")

	// ── Step 2: Auth credentials ──────────────────────────────────────────────
	switch authChoice {
	case "basic":
		var username, password string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Username").
					Value(&username).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("username cannot be empty")
						}
						return nil
					}),
				huh.NewInput().
					Title("Password").
					EchoMode(huh.EchoModePassword).
					Value(&password).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("password cannot be empty")
						}
						return nil
					}),
			),
		).Run()
		if err != nil {
			return Credentials{}, fmt.Errorf("auth form: %w", err)
		}
		creds.AuthType = AuthTypeBasic
		creds.Username = strings.TrimSpace(username)
		creds.Password = password

	case "token":
		var token string
		err = huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("API Token").
					EchoMode(huh.EchoModePassword).
					Value(&token).
					Validate(func(s string) error {
						if strings.TrimSpace(s) == "" {
							return fmt.Errorf("API token cannot be empty")
						}
						return nil
					}),
			),
		).Run()
		if err != nil {
			return Credentials{}, fmt.Errorf("token form: %w", err)
		}
		creds.AuthType = AuthTypeToken
		creds.Token = token
	}

	// ── Step 3: Output directory + save option ────────────────────────────────
	var outputDirInput string
	var saveCredentials bool

	err = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Export Directory").
				Description("Leave empty to export alongside the binary (current directory)").
				Placeholder("~/my-workflows  or  /path/to/dir").
				Value(&outputDirInput),
			huh.NewConfirm().
				Title("Save credentials for future sessions?").
				Description("Encrypted with AES-256-GCM using a passphrase you choose").
				Value(&saveCredentials),
		),
	).Run()
	if err != nil {
		return Credentials{}, fmt.Errorf("options form: %w", err)
	}

	creds.OutputDir = resolveOutputDir(outputDirInput)
	fmt.Printf("  → Export directory: %s\n", creds.OutputDir)

	// ── Step 4: Passphrase for saving ─────────────────────────────────────────
	if saveCredentials {
		if err := promptAndSave(creds); err != nil {
			fmt.Printf("⚠  Could not save credentials: %v\n", err)
		}
	}

	return creds, nil
}

// promptAndSave asks for a passphrase and encrypts credentials to disk.
func promptAndSave(creds Credentials) error {
	var passphrase string
	err := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Choose a Passphrase").
				Description("Used to encrypt your stored credentials").
				EchoMode(huh.EchoModePassword).
				Value(&passphrase).
				Validate(func(s string) error {
					if strings.TrimSpace(s) == "" {
						return fmt.Errorf("passphrase cannot be empty")
					}
					return nil
				}),
		),
	).Run()
	if err != nil {
		return err
	}
	if err := SaveCredentials(creds, strings.TrimSpace(passphrase)); err != nil {
		return err
	}
	fmt.Println("✓  Credentials saved.")
	return nil
}

// resolveOutputDir converts a user-supplied path to an absolute directory.
// Empty input defaults to "#n8n_workflows_original" in the current directory.
func resolveOutputDir(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		return filepath.Join(cwd, "#n8n_workflows_original")
	}
	if strings.HasPrefix(input, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			input = filepath.Join(home, input[1:])
		}
	}
	return filepath.Clean(input)
}
