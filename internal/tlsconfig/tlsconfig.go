package tlsconfig

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	"github.com/joshuabednaz/go-logger/internal/config"
)

// Result holds a TLS certificate and whether it was auto-generated.
type Result struct {
	Certificate tls.Certificate
	AutoGen     bool
	Fingerprint string
	Leaf        *x509.Certificate
}

// Load resolves TLS material per spec: path beats PEM for each artifact; both required
// or both missing; partial config errors; auto-generate when both missing unless MustUseProvided.
func Load(cfg config.TLSConfig) (Result, error) {
	certSrc := resolveSource(cfg.CertPath, cfg.CertPEM)
	keySrc := resolveSource(cfg.KeyPath, cfg.KeyPEM)

	if certSrc.present != keySrc.present {
		return Result{}, fmt.Errorf("tls: provide both certificate and private key, or neither")
	}
	if !certSrc.present {
		if cfg.MustUseProvided {
			return Result{}, fmt.Errorf("tls: TLS_MUST_USE_PROVIDED_CERT set but cert/key not provided")
		}
		return generateSelfSigned(cfg.ExtraSANHosts)
	}

	certPEM, err := certSrc.loadPEM()
	if err != nil {
		return Result{}, fmt.Errorf("tls cert: %w", err)
	}
	keyPEM, err := keySrc.loadPEM()
	if err != nil {
		return Result{}, fmt.Errorf("tls key: %w", err)
	}
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Result{}, err
	}
	leaf, err := leafFromTLSCert(tlsCert)
	if err != nil {
		return Result{}, err
	}
	fp := fingerprintLeaf(leaf)
	return Result{Certificate: tlsCert, AutoGen: false, Fingerprint: fp, Leaf: leaf}, nil
}

type source struct {
	path    string
	inline  string
	present bool
}

func resolveSource(path, inline string) source {
	if strings.TrimSpace(path) != "" {
		return source{path: path, present: true}
	}
	if strings.TrimSpace(inline) != "" {
		return source{inline: inline, present: true}
	}
	return source{}
}

func (s source) loadPEM() ([]byte, error) {
	if s.path != "" {
		return os.ReadFile(s.path)
	}
	return []byte(s.inline), nil
}

func generateSelfSigned(extraHosts []string) (Result, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return Result{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return Result{}, err
	}
	notBefore := time.Now().Add(-time.Hour)
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	dns := []string{"localhost"}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, h := range extraHosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
		} else {
			dns = append(dns, h)
		}
	}

	tpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization: []string{"go-logger"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              dns,
		IPAddresses:           ips,
	}

	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &key.PublicKey, key)
	if err != nil {
		return Result{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return Result{}, err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return Result{}, err
	}
	fp := fingerprintLeaf(leaf)
	slog.Warn("tls: using auto-generated self-signed certificate",
		"sha256_fingerprint", fp,
		"hint", "use grpcurl -insecure, curl -k, or pin this fingerprint")
	return Result{Certificate: tlsCert, AutoGen: true, Fingerprint: fp, Leaf: leaf}, nil
}

func leafFromTLSCert(c tls.Certificate) (*x509.Certificate, error) {
	if len(c.Certificate) == 0 {
		return nil, fmt.Errorf("no certificate in tls.Certificate")
	}
	return x509.ParseCertificate(c.Certificate[0])
}

func fingerprintLeaf(leaf *x509.Certificate) string {
	sum := sha256.Sum256(leaf.Raw)
	return hex.EncodeToString(sum[:])
}
