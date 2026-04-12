package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

// SignRelease produces Release.gpg (detached) and InRelease (clearsign)
// for the Release file in dir. No-op if armoredKey is empty.
func SignRelease(dir, armoredKey, passphrase string) error {
	armoredKey = strings.TrimSpace(armoredKey)
	if armoredKey == "" {
		return nil
	}

	releaseFile := filepath.Join(dir, "Release")
	releaseGPG := filepath.Join(dir, "Release.gpg")
	inRelease := filepath.Join(dir, "InRelease")

	releaseData, err := os.ReadFile(releaseFile)
	if err != nil {
		return fmt.Errorf("Release file not found: %w", err)
	}

	var key *crypto.Key
	if passphrase != "" {
		key, err = crypto.NewPrivateKeyFromArmored(armoredKey, []byte(passphrase))
	} else {
		key, err = crypto.NewKeyFromArmored(armoredKey)
	}
	if err != nil {
		return fmt.Errorf("loading GPG key: %w", err)
	}
	defer key.ClearPrivateParams()

	pgp := crypto.PGP()

	// Detached signature
	detachedSigner, err := pgp.Sign().SigningKey(key).Detached().New()
	if err != nil {
		return fmt.Errorf("creating detached signer: %w", err)
	}
	sig, err := detachedSigner.Sign(releaseData, crypto.Armor)
	if err != nil {
		return fmt.Errorf("detached sign: %w", err)
	}

	// Clearsign
	clearSigner, err := pgp.Sign().SigningKey(key).New()
	if err != nil {
		return fmt.Errorf("creating clearsigner: %w", err)
	}
	clearSig, err := clearSigner.SignCleartext(releaseData)
	if err != nil {
		return fmt.Errorf("clearsign: %w", err)
	}

	// Write both files atomically via temp+rename to avoid partial state
	releaseGPGTmp := releaseGPG + ".tmp"
	inReleaseTmp := inRelease + ".tmp"

	if err := os.WriteFile(releaseGPGTmp, sig, 0644); err != nil {
		return fmt.Errorf("writing Release.gpg: %w", err)
	}
	if err := os.WriteFile(inReleaseTmp, clearSig, 0644); err != nil {
		os.Remove(releaseGPGTmp)
		return fmt.Errorf("writing InRelease: %w", err)
	}

	if err := os.Rename(releaseGPGTmp, releaseGPG); err != nil {
		os.Remove(releaseGPGTmp)
		os.Remove(inReleaseTmp)
		return fmt.Errorf("finalizing Release.gpg: %w", err)
	}
	if err := os.Rename(inReleaseTmp, inRelease); err != nil {
		os.Remove(inReleaseTmp)
		os.Remove(releaseGPG) // rollback the already-renamed Release.gpg
		return fmt.Errorf("finalizing InRelease: %w", err)
	}

	return nil
}
