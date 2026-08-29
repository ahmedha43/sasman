package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	pkiMu     sync.RWMutex
	pkiDir    = "data/pki"
	caCert    *x509.Certificate
	caKey     *ecdsa.PrivateKey
	caCertPEM []byte
	caKeyPEM  []byte

	serverCert    *tls.Certificate
	serverCertPEM []byte
	serverKeyPEM  []byte
)

// CertBundle holds client certificate and key in PEM format
type CertBundle struct {
	CommonName   string    `json:"common_name"`
	SerialNumber string    `json:"serial_number"`
	CertPEM      string    `json:"cert_pem"`
	KeyPEM       string    `json:"key_pem"`
	CAPEM        string    `json:"ca_pem"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// InitPKI initializes or loads the Root CA and Server Certificates
func InitPKI() error {
	pkiMu.Lock()
	defer pkiMu.Unlock()

	if err := os.MkdirAll(pkiDir, 0700); err != nil {
		return fmt.Errorf("failed to create PKI directory: %w", err)
	}

	caCertPath := filepath.Join(pkiDir, "ca.crt")
	caKeyPath := filepath.Join(pkiDir, "ca.key")

	// 1. Check if Root CA already exists, or generate a new one
	if fileExists(caCertPath) && fileExists(caKeyPath) {
		certBytes, err := os.ReadFile(caCertPath)
		if err != nil {
			return fmt.Errorf("failed to read CA cert: %w", err)
		}
		keyBytes, err := os.ReadFile(caKeyPath)
		if err != nil {
			return fmt.Errorf("failed to read CA key: %w", err)
		}

		block, _ := pem.Decode(certBytes)
		if block == nil {
			return fmt.Errorf("failed to decode CA cert PEM")
		}
		parsedCert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse CA cert: %w", err)
		}

		keyBlock, _ := pem.Decode(keyBytes)
		if keyBlock == nil {
			return fmt.Errorf("failed to decode CA key PEM")
		}
		parsedKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse CA key: %w", err)
		}

		caCert = parsedCert
		caKey = parsedKey
		caCertPEM = certBytes
		caKeyPEM = keyBytes
		log.Printf("[pki] Loaded existing Root CA: CN=%s, Expires=%s", caCert.Subject.CommonName, caCert.NotAfter.Format("2006-01-02"))
	} else {
		// Generate new Root CA (valid for 10 years)
		if err := generateRootCA(); err != nil {
			return fmt.Errorf("failed to generate Root CA: %w", err)
		}
	}

	// 2. Ensure Server Certificate exists
	serverCertPath := filepath.Join(pkiDir, "server.crt")
	serverKeyPath := filepath.Join(pkiDir, "server.key")

	if fileExists(serverCertPath) && fileExists(serverKeyPath) {
		pair, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
		if err != nil {
			log.Printf("[pki] Server cert invalid, regenerating: %v", err)
			if err := generateServerCert(); err != nil {
				return err
			}
		} else {
			serverCert = &pair
			serverCertPEM, _ = os.ReadFile(serverCertPath)
			serverKeyPEM, _ = os.ReadFile(serverKeyPath)
			log.Printf("[pki] Loaded RadSec Server certificate successfully")
		}
	} else {
		if err := generateServerCert(); err != nil {
			return fmt.Errorf("failed to generate Server Certificate: %w", err)
		}
	}

	return nil
}

func generateRootCA() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"SASMAN Network Systems"},
			CommonName:   "SASMAN Internal Root CA",
		},
		NotBefore:             time.Now().Add(-10 * time.Minute),
		NotAfter:              time.Now().AddDate(10, 0, 0), // 10 years
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	caCert, _ = x509.ParseCertificate(certDER)
	caKey = priv

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	caCertPEM = certPEMBlock
	caKeyPEM = keyPEMBlock

	_ = os.WriteFile(filepath.Join(pkiDir, "ca.crt"), certPEMBlock, 0644)
	_ = os.WriteFile(filepath.Join(pkiDir, "ca.key"), keyPEMBlock, 0600)

	log.Printf("[pki] Generated new Root CA successfully (Valid for 10 years)")
	return nil
}

func generateServerCert() error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"SASMAN Network Systems"},
			CommonName:   "SASMAN RadSec Server",
		},
		NotBefore:   time.Now().Add(-10 * time.Minute),
		NotAfter:    time.Now().AddDate(5, 0, 0), // 5 years
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("0.0.0.0"),
			net.ParseIP("::1"),
		},
		DNSNames: []string{
			"localhost",
			"radsec.sasman.local",
			"radius.sasman.local",
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return err
	}

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	serverCertPEM = certPEMBlock
	serverKeyPEM = keyPEMBlock

	pair, err := tls.X509KeyPair(certPEMBlock, keyPEMBlock)
	if err != nil {
		return err
	}
	serverCert = &pair

	_ = os.WriteFile(filepath.Join(pkiDir, "server.crt"), certPEMBlock, 0644)
	_ = os.WriteFile(filepath.Join(pkiDir, "server.key"), keyPEMBlock, 0600)

	log.Printf("[pki] Generated RadSec Server certificate successfully (Valid for 5 years)")
	return nil
}

// GenerateClientCertificate creates a new client certificate signed by the Root CA
func GenerateClientCertificate(commonName string, validityDays int) (*CertBundle, error) {
	pkiMu.RLock()
	defer pkiMu.RUnlock()

	if caCert == nil || caKey == nil {
		return nil, fmt.Errorf("Root CA is not initialized")
	}

	if validityDays <= 0 {
		validityDays = 365 * 2 // Default 2 years
	}

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	now := time.Now()
	expiresAt := now.AddDate(0, 0, validityDays)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"SASMAN Remote Agents"},
			CommonName:   commonName,
		},
		NotBefore:   now.Add(-10 * time.Minute),
		NotAfter:    expiresAt,
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{commonName},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &priv.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign client certificate: %w", err)
	}

	certPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	privDER, _ := x509.MarshalECPrivateKey(priv)
	keyPEMBlock := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	bundle := &CertBundle{
		CommonName:   commonName,
		SerialNumber: serialNumber.Text(16),
		CertPEM:      string(certPEMBlock),
		KeyPEM:       string(keyPEMBlock),
		CAPEM:        string(caCertPEM),
		CreatedAt:    now,
		ExpiresAt:    expiresAt,
	}

	return bundle, nil
}

// GetServerTLSConfig returns a tls.Config with mutual TLS (mTLS) enabled
func GetServerTLSConfig(verifyPeerFunc func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error) (*tls.Config, error) {
	pkiMu.RLock()
	defer pkiMu.RUnlock()

	if serverCert == nil || len(caCertPEM) == 0 {
		return nil, fmt.Errorf("PKI server certificate not ready")
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCertPEM) {
		return nil, fmt.Errorf("failed to append Root CA to cert pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{*serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		},
	}

	if verifyPeerFunc != nil {
		tlsConfig.VerifyPeerCertificate = verifyPeerFunc
	}

	return tlsConfig, nil
}

// GetClientTLSConfig constructs a *tls.Config for remote agent from PEM strings
func GetClientTLSConfig(certPEM, keyPEM, caPEM string) (*tls.Config, error) {
	cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
	if err != nil {
		return nil, fmt.Errorf("invalid client cert/key pair: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM([]byte(caPEM)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates:       []tls.Certificate{cert},
		RootCAs:            caPool,
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}, nil
}

// GetCACertPEM returns the Root CA certificate in PEM format
func GetCACertPEM() []byte {
	pkiMu.RLock()
	defer pkiMu.RUnlock()
	return caCertPEM
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return true
}
