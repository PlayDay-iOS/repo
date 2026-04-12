package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

func TestSignRelease_NoopWhenKeyEmpty(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "Release"), []byte("test"), 0644)

	if err := SignRelease(dir, "", ""); err != nil {
		t.Fatalf("expected no-op, got error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "Release.gpg")); !os.IsNotExist(err) {
		t.Error("Release.gpg should not exist when key is empty")
	}
	if _, err := os.Stat(filepath.Join(dir, "InRelease")); !os.IsNotExist(err) {
		t.Error("InRelease should not exist when key is empty")
	}
}

func TestSignRelease_ErrorWhenReleaseFileMissing(t *testing.T) {
	dir := t.TempDir()
	err := SignRelease(dir, "-----BEGIN PGP PRIVATE KEY BLOCK-----\nfake\n-----END PGP PRIVATE KEY BLOCK-----", "")
	if err == nil {
		t.Fatal("expected error when Release file is missing")
	}
}

func TestSignRelease_WithTestKey(t *testing.T) {
	dir := t.TempDir()
	releaseContent := []byte("Suite: stable\nCodename: stable\n")
	os.WriteFile(filepath.Join(dir, "Release"), releaseContent, 0644)

	// Generate a test key
	pgp := crypto.PGP()
	key, err := pgp.KeyGeneration().AddUserId("Test", "test@example.com").New().GenerateKey()
	if err != nil {
		t.Fatalf("generating test key: %v", err)
	}
	armored, err := key.Armor()
	if err != nil {
		t.Fatalf("armoring key: %v", err)
	}

	if err := SignRelease(dir, armored, ""); err != nil {
		t.Fatalf("SignRelease failed: %v", err)
	}

	// Verify Release.gpg exists and is armored PGP
	gpgData, err := os.ReadFile(filepath.Join(dir, "Release.gpg"))
	if err != nil {
		t.Fatal("Release.gpg should exist")
	}
	if !strings.Contains(string(gpgData), "BEGIN PGP SIGNATURE") {
		t.Error("Release.gpg should contain armored PGP signature")
	}

	// Verify InRelease exists and contains the original content
	inReleaseData, err := os.ReadFile(filepath.Join(dir, "InRelease"))
	if err != nil {
		t.Fatal("InRelease should exist")
	}
	if !strings.Contains(string(inReleaseData), "Suite: stable") {
		t.Error("InRelease should contain original Release content")
	}
	if !strings.Contains(string(inReleaseData), "BEGIN PGP SIGNED MESSAGE") {
		t.Error("InRelease should be a cleartext signed message")
	}
}
