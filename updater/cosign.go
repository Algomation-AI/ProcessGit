// Cosign wrapper. Shells out to the cosign binary that's installed in the
// updater image. Verification only — the updater never signs anything.
//
// Two operations:
//
//   VerifyImage  — checks that the image at <ref-or-digest> was signed by the
//                  expected GitHub Actions OIDC identity (constructed from
//                  the manifest's signing fields).
//
//   VerifyBlob   — checks that release.json itself was signed by the same
//                  identity. Called BEFORE we trust the contents of the
//                  manifest. Without this, an attacker who can publish a
//                  malicious release.json could redirect the updater to
//                  pull an arbitrary image.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

type Cosign struct {
	Bin string // path to cosign binary; default "cosign"
}

func NewCosign() *Cosign {
	bin := os.Getenv("COSIGN_BIN")
	if bin == "" {
		bin = "cosign"
	}
	return &Cosign{Bin: bin}
}

// VerifyImage runs `cosign verify` against the image ref+digest.
func (c *Cosign) VerifyImage(ctx context.Context, m *Manifest) error {
	ref := m.Image.DigestRef() // ref + digest is canonical
	args := []string{
		"verify",
		"--certificate-identity-regexp", m.Signing.IdentityRegex,
		"--certificate-oidc-issuer", m.Signing.Issuer,
		ref,
	}
	return c.run(ctx, "verify image "+ref, args, nil)
}

// VerifyBlob runs `cosign verify-blob` against release.json using the
// release.json.sig + release.json.crt that we downloaded alongside it.
// Inputs are byte slices so callers don't need to persist them to disk
// outside the updater's control.
func (c *Cosign) VerifyBlob(ctx context.Context, m *Manifest, jsonBytes, sig, cert []byte) error {
	// cosign verify-blob requires file paths, so we write to short-lived temp
	// files (cleaned up regardless of outcome).
	jsonPath, err := writeTemp("release-*.json", jsonBytes)
	if err != nil {
		return err
	}
	defer os.Remove(jsonPath)
	sigPath, err := writeTemp("release-*.sig", sig)
	if err != nil {
		return err
	}
	defer os.Remove(sigPath)
	certPath, err := writeTemp("release-*.crt", cert)
	if err != nil {
		return err
	}
	defer os.Remove(certPath)

	args := []string{
		"verify-blob",
		"--signature", sigPath,
		"--certificate", certPath,
		"--certificate-identity-regexp", m.Signing.IdentityRegex,
		"--certificate-oidc-issuer", m.Signing.Issuer,
		jsonPath,
	}
	return c.run(ctx, "verify-blob release.json", args, nil)
}

func writeTemp(pattern string, data []byte) (string, error) {
	f, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create temp %s: %w", pattern, err)
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", fmt.Errorf("write temp %s: %w", pattern, err)
	}
	return f.Name(), nil
}

// run executes cosign and returns a wrapped error including the output on
// failure. stdin is optional.
func (c *Cosign) run(ctx context.Context, what string, args []string, stdin []byte) error {
	cmd := exec.CommandContext(ctx, c.Bin, args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cosign %s failed: %w\nstdout: %s\nstderr: %s",
			what, err, out.String(), errb.String())
	}
	return nil
}
