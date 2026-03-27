package tlsconfig

import (
	"testing"

	"github.com/joshuabednaz/go-logger/internal/config"
)

func TestLoadPartialFails(t *testing.T) {
	_, err := Load(config.TLSConfig{CertPEM: "-----BEGIN CERTIFICATE-----\nabc\n-----END CERTIFICATE-----"})
	if err == nil {
		t.Fatal("expected error when only cert provided")
	}
}

func TestMustUseProvidedFailsWhenMissing(t *testing.T) {
	_, err := Load(config.TLSConfig{MustUseProvided: true})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestAutoGen(t *testing.T) {
	res, err := Load(config.TLSConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if !res.AutoGen || res.Fingerprint == "" || len(res.Certificate.Certificate) == 0 {
		t.Fatalf("unexpected result: autogen=%v fp=%q", res.AutoGen, res.Fingerprint)
	}
}
