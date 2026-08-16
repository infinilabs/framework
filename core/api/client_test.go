package api

import (
	"crypto/tls"
	"net"
	"os"
	"path/filepath"
	"testing"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
)

func TestGetClientTLSConfigSkipDomainVerifyAllowsHostnameMismatch(t *testing.T) {
	rootCert, rootKey, rootCertPEM := util.GetRootCert()
	serverCertPEM, serverKeyPEM, err := util.GenerateServerCert(rootCert, rootKey, rootCertPEM, nil)
	if err != nil {
		t.Fatalf("generate server cert: %v", err)
	}

	dir := t.TempDir()
	caFile := filepath.Join(dir, "ca.crt")
	serverCertFile := filepath.Join(dir, "server.crt")
	serverKeyFile := filepath.Join(dir, "server.key")

	if err := os.WriteFile(caFile, rootCertPEM, 0600); err != nil {
		t.Fatalf("write ca cert: %v", err)
	}
	if err := os.WriteFile(serverCertFile, serverCertPEM, 0600); err != nil {
		t.Fatalf("write server cert: %v", err)
	}
	if err := os.WriteFile(serverKeyFile, serverKeyPEM, 0600); err != nil {
		t.Fatalf("write server key: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("create listener: %v", err)
	}
	defer ln.Close()

	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		t.Fatalf("load server cert: %v", err)
	}
	ln = tls.NewListener(ln, &tls.Config{Certificates: []tls.Certificate{serverCert}})

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		if tlsConn, ok := conn.(*tls.Conn); ok {
			_ = tlsConn.Handshake()
		}
		_ = conn.Close()
	}()

	cfg, err := GetClientTLSConfig(&config.TLSConfig{
		TLSCACertFile:         caFile,
		SkipDomainVerify:      true,
		TLSInsecureSkipVerify: false,
	})
	if err != nil {
		t.Fatalf("get client tls config: %v", err)
	}

	conn, err := tls.Dial("tcp", ln.Addr().String(), cfg)
	if err != nil {
		t.Fatalf("tls dial: %v", err)
	}
	_ = conn.Close()
	<-done
}
