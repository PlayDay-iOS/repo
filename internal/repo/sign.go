package repo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/ProtonMail/gopenpgp/v3/crypto"
)

// SignRelease produces Release.gpg (detached) and InRelease (clearsign)
// for the Release file in dir. No-op if armoredKey is empty.
func SignRelease(ctx context.Context, dir, armoredKey, passphrase string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

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

	detachedSigner, err := pgp.Sign().SigningKey(key).Detached().New()
	if err != nil {
		return fmt.Errorf("creating detached signer: %w", err)
	}
	sig, err := detachedSigner.Sign(releaseData, crypto.Armor)
	if err != nil {
		return fmt.Errorf("detached sign: %w", err)
	}

	clearSigner, err := pgp.Sign().SigningKey(key).New()
	if err != nil {
		return fmt.Errorf("creating clearsigner: %w", err)
	}
	clearSig, err := clearSigner.SignCleartext(releaseData)
	if err != nil {
		return fmt.Errorf("clearsign: %w", err)
	}

	if err := fileutil.WriteAtomicBytes(releaseGPG, 0644, sig); err != nil {
		return fmt.Errorf("writing Release.gpg: %w", err)
	}
	if err := fileutil.WriteAtomicBytes(inRelease, 0644, clearSig); err != nil {
		// Roll back the already-written Release.gpg to avoid an inconsistent
		// signed state where a detached signature exists without a clearsigned
		// counterpart.
		os.Remove(releaseGPG)
		return fmt.Errorf("writing InRelease: %w", err)
	}

	return nil
}
