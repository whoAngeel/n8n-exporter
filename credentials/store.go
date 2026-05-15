package credentials

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/pbkdf2"
)

const (
	credFileName = "credentials.enc"
	credDirName  = "n8n-exporter"
	pbkdf2Iter   = 100_000
	keyLen       = 32 // AES-256
	saltLen      = 16
)

// storedCredentials is the JSON structure written to disk (encrypted).
type storedCredentials struct {
	BaseURL   string `json:"baseURL"`
	Token     string `json:"token"`
	OutputDir string `json:"outputDir,omitempty"`
}

// credFilePath returns the platform-appropriate path for the encrypted file.
// Uses $XDG_CONFIG_HOME or ~/.config on Unix, %AppData% on Windows.
func credFilePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(configDir, credDirName, credFileName), nil
}

// CredentialsFileExists returns true if the encrypted credentials file exists.
func CredentialsFileExists() bool {
	path, err := credFilePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// SaveCredentials encrypts creds with passphrase and writes them to disk.
func SaveCredentials(creds Credentials, passphrase string) error {
	path, err := credFilePath()
	if err != nil {
		return err
	}

	// Ensure the directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Serialize credentials to JSON.
	stored := storedCredentials{
		BaseURL:   creds.BaseURL,
		Token:     creds.Token,
		OutputDir: creds.OutputDir,
	}
	plaintext, err := json.Marshal(stored)
	if err != nil {
		return fmt.Errorf("serializing credentials: %w", err)
	}

	// Generate a random salt.
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("generating salt: %w", err)
	}

	// Derive AES-256 key from passphrase using PBKDF2-SHA256.
	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, keyLen, sha256.New)

	// Encrypt with AES-256-GCM.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("creating GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// File layout: [16-byte salt][nonce+ciphertext]
	fileData := append(salt, ciphertext...)
	if err := os.WriteFile(path, fileData, 0600); err != nil {
		return fmt.Errorf("writing credentials file: %w", err)
	}

	return nil
}

// LoadCredentials reads and decrypts the credentials file using passphrase.
// Returns an error if the passphrase is wrong or the file is corrupted.
func LoadCredentials(passphrase string) (Credentials, error) {
	path, err := credFilePath()
	if err != nil {
		return Credentials{}, err
	}

	fileData, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("reading credentials file: %w", err)
	}

	if len(fileData) < saltLen {
		return Credentials{}, errors.New("credentials file is corrupted")
	}

	salt := fileData[:saltLen]
	ciphertext := fileData[saltLen:]

	// Re-derive the key from the passphrase and stored salt.
	key := pbkdf2.Key([]byte(passphrase), salt, pbkdf2Iter, keyLen, sha256.New)

	block, err := aes.NewCipher(key)
	if err != nil {
		return Credentials{}, fmt.Errorf("creating cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Credentials{}, fmt.Errorf("creating GCM: %w", err)
	}

	if len(ciphertext) < gcm.NonceSize() {
		return Credentials{}, errors.New("credentials file is corrupted")
	}
	nonce := ciphertext[:gcm.NonceSize()]
	data := ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		// GCM authentication failure = wrong passphrase or tampered file.
		return Credentials{}, errors.New("wrong passphrase or corrupted credentials file")
	}

	var stored storedCredentials
	if err := json.Unmarshal(plaintext, &stored); err != nil {
		return Credentials{}, fmt.Errorf("parsing credentials: %w", err)
	}

	return Credentials{
		BaseURL:   stored.BaseURL,
		Token:     stored.Token,
		OutputDir: stored.OutputDir,
	}, nil
}

// DeleteCredentials removes the stored credentials file.
func DeleteCredentials() error {
	path, err := credFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting credentials file: %w", err)
	}
	return nil
}
