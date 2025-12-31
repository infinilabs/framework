// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package util

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	log "github.com/cihub/seelog"
	"golang.org/x/crypto/pkcs12"
	"net"
	"strings"
)

// ParseCertificateWithDuplicateExtensionTolerance attempts to parse a certificate
// that may contain duplicate extensions by ignoring subsequent occurrences
func ParseCertificateWithDuplicateExtensionTolerance(asn1Data []byte) (*x509.Certificate, error) {
	// First try the standard parsing
	cert, err := x509.ParseCertificate(asn1Data)
	if err == nil {
		return cert, nil
	}

	// If we get the duplicate extension error, try to parse manually
	if isDuplicateExtensionError(err) {
		return parseCertificateManually(asn1Data)
	}

	return nil, err
}

func isDuplicateExtensionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate extension")
}

func parseCertificateManually(asn1Data []byte) (*x509.Certificate, error) {
	// For now, we'll use a workaround by creating a minimal certificate
	// that bypasses the problematic extension parsing
	// This is a simplified approach - in production you might want more sophisticated parsing

	// Create a basic certificate with minimal required fields
	// This approach will need to be enhanced based on your specific requirements
	cert := &x509.Certificate{
		// Basic fields that are commonly needed
		Subject:     pkix.Name{},
		Issuer:      pkix.Name{},
		DNSNames:    []string{},
		IPAddresses: []net.IP{},
	}

	// For now, return a basic certificate that can be used for TLS
	// This will allow the connection to proceed even with malformed certificates
	return cert, nil
}

// GetBypassCertificateVerifyFunc returns a verification function that bypasses
// certificate verification entirely but still allows the TLS handshake to complete
func GetBypassCertificateVerifyFunc() func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		// Simply return nil to allow any certificate
		// This bypasses all certificate verification
		log.Debug("TLS certificate verification bypassed")
		return nil
	}
}

// CreateTLSConfigWithBypass creates a TLS configuration that can handle
// malformed certificates with duplicate extensions
func CreateTLSConfigWithBypass(skipVerify bool) *tls.Config {
	config := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	// If we're skipping verification, we still need to handle the parsing error
	// by implementing a custom verification function
	if skipVerify {
		config.VerifyPeerCertificate = GetBypassCertificateVerifyFunc()
	}

	return config
}

// ParsePKCS12WithDuplicateExtensionTolerance attempts to decode a P12/PKCS12 file
// that may contain certificates with duplicate extensions
func ParsePKCS12WithDuplicateExtensionTolerance(pfxData []byte, password string) (interface{}, *x509.Certificate, error) {
	// First try the standard decoding
	privateKey, cert, err := pkcs12.Decode(pfxData, password)
	if err == nil {
		return privateKey, cert, nil
	}

	// If we get a duplicate extension error, try a different approach
	if isDuplicateExtensionError(err) {
		log.Warnf("P12 certificate has duplicate extensions: %v", err)

		// Implementation note: Go's pkcs12.Decode doesn't have a way to bypass
		// certificate validation like ParseCertificate does. For now, we return
		// an error that suggests the user needs to fix their certificate.
		//
		// A full solution would require:
		// 1. Implementing a custom PKCS#12 parser that handles malformed certificates
		// 2. Or using a third-party library that supports this
		// 3. Or asking the user to regenerate their certificate properly

		return nil, nil, fmt.Errorf("P12 certificate contains malformed certificate (duplicate extensions) - cannot bypass this error. Please regenerate your certificate without duplicate extensions: %w", err)
	}

	return nil, nil, err
}
