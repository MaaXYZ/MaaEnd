//go:build linux

package ziplineimport

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerateRootCAValidity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ca.crt")
	if _, err := generateRootCA(path); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("no PEM block")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	nowUTC := time.Now().UTC()
	notBeforeUTC := cert.NotBefore.UTC()
	notAfterUTC := cert.NotAfter.UTC()

	if !(nowUTC.After(notBeforeUTC) && nowUTC.Before(notAfterUTC)) {
		t.Fatalf("CA 当前不在有效期内: now=%v NotBefore=%v NotAfter=%v",
			nowUTC, notBeforeUTC, notAfterUTC)
	}
	if notAfterUTC.Sub(nowUTC) < 30*time.Minute {
		t.Fatalf("CA 剩余有效期不足: NotAfter=%v", notAfterUTC)
	}
	if !cert.IsCA {
		t.Fatal("not a CA cert")
	}
	t.Logf("ok: now=%v valid=[%v,%v]", nowUTC, notBeforeUTC, notAfterUTC)
}
