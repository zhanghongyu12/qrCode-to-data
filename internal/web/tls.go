package web

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"time"
)

// 自签证书文件（持久化，避免每次重启换证书导致浏览器重新弹警告）。
const (
	certFile = "qrcd-cert.pem"
	keyFile  = "qrcd-key.pem"
)

// ensureCertFiles 确保 qrcd-cert.pem / qrcd-key.pem 存在；不存在则生成自签证书。
// 证书 SAN 包含 localhost、127.0.0.1 与本机所有非 loopback 网卡 IP，
// 供手机经局域网 IP 访问 HTTPS（浏览器对自签证书会提示一次「继续访问」）。
func ensureCertFiles() error {
	if _, err := tls.LoadX509KeyPair(certFile, keyFile); err == nil {
		return nil
	}
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "qrcd"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}
	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && !ipn.IP.IsLoopback() {
				tmpl.IPAddresses = append(tmpl.IPAddresses, ipn.IP)
			}
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certFile, certPEM, 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return os.WriteFile(keyFile, keyPEM, 0o600)
}
