package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"

	"github.com/srmdn/foliocms/internal/config"
	"github.com/srmdn/foliocms/internal/db"
)

func runSetup(database *db.DB, cfg *config.Config) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("=== Folio First-Time Setup ===")
	fmt.Println()

	email := cfg.AdminEmail
	if email == "" {
		fmt.Print("Admin email: ")
		line, _ := reader.ReadString('\n')
		email = strings.TrimSpace(line)
	}
	if email == "" {
		return fmt.Errorf("admin email is required")
	}

	passwd := cfg.AdminPasswd
	if passwd == "" {
		var err error
		passwd, err = promptPassword(reader)
		if err != nil {
			return err
		}
	}
	if len(passwd) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(passwd), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	_, err = database.Exec(
		`INSERT INTO users (email, passwd_hash) VALUES (?, ?)
		 ON CONFLICT(email) DO UPDATE SET passwd_hash = excluded.passwd_hash`,
		email, string(hash),
	)
	if err != nil {
		return fmt.Errorf("saving admin user: %w", err)
	}

	fmt.Printf("Admin user created: %s\n", email)
	return nil
}

// promptPassword asks for the password twice without echoing the input. A
// mismatch retries up to three times.
func promptPassword(reader *bufio.Reader) (string, error) {
	for attempt := 0; attempt < 3; attempt++ {
		first, err := readSecret(reader, "Admin password: ")
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		if first == "" {
			return "", fmt.Errorf("password is required")
		}
		second, err := readSecret(reader, "Confirm password: ")
		if err != nil {
			return "", fmt.Errorf("reading password: %w", err)
		}
		if first != second {
			fmt.Println("passwords do not match, try again")
			continue
		}
		return first, nil
	}
	return "", fmt.Errorf("passwords did not match after 3 attempts")
}

// readSecret reads one line without terminal echo. Falls back to a plain line
// read from the shared reader when stdin is not a terminal (CI, piping).
func readSecret(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return "", err
	}
	return strings.TrimSpace(line), nil
}
