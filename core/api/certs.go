/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/caddyserver/certmagic"
	"github.com/cihub/seelog"
	"github.com/libdns/tencentcloud"
	"go.uber.org/zap"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"io/ioutil"
	"os"
	"path"
	log "github.com/cihub/seelog"
	"time"
)

func GetServerTLSConfig(tlsCfg *config.TLSConfig) (*tls.Config, error) {
	var err error
	cfg := &tls.Config{
		MinVersion: tls.VersionTLS12,
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.CurveP384,
			tls.X25519, // Go 1.8 only
		},
		PreferServerCipherSuites: true,
		InsecureSkipVerify:       tlsCfg.TLSInsecureSkipVerify,
		SessionTicketsDisabled:   false,
		ClientSessionCache:       tls.NewLRUClientSessionCache(128),
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Go 1.8 only
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Go 1.8 only
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		},
		NextProtos: []string{"h2", "http/1.1"},
	}

	if tlsCfg.AutoIssue.Enabled && (tlsCfg.TLSCertFile == "" && tlsCfg.TLSKeyFile == "") {
		AutoIssueTLSCertificates(tlsCfg,cfg)
	} else {
		//try self-signed certs
		if tlsCfg.TLSCertFile == "" && tlsCfg.TLSKeyFile == "" {
			dataDir := global.Env().GetDataDir()
			tlsCfg.TLSCertFile = path.Join(dataDir, "certs/instance.crt")
			tlsCfg.TLSKeyFile = path.Join(dataDir, "certs/instance.key")
			tlsCfg.TLSCACertFile = path.Join(dataDir, "certs/ca.crt")
			caKey := path.Join(dataDir, "certs/ca.key")
			if !(util.FileExists(tlsCfg.TLSCACertFile) && util.FileExists(tlsCfg.TLSCertFile) && util.FileExists(tlsCfg.TLSKeyFile)) {
				err = os.MkdirAll(path.Join(dataDir, "certs"), 0775)
				if err != nil {
					return nil, err
				}
				log.Info("auto generating cert files")
				rootCert, rootKey, rootCertPEM = util.GetRootCert()
				if tlsCfg.DefaultDomain == "" {
					tlsCfg.DefaultDomain = "localhost"
				}
				instanceCertPEM, instanceKeyPEM, err := util.GenerateServerCert(rootCert, rootKey, rootCertPEM, []string{tlsCfg.DefaultDomain})
				if err != nil {
					return nil, err
				}
				caKeyPEM := pem.EncodeToMemory(&pem.Block{
					Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
				})

				_, err = util.FilePutContentWithByte(caKey, caKeyPEM)
				if err != nil {
					return nil, err
				}

				util.FilePutContentWithByte(tlsCfg.TLSCACertFile, rootCertPEM)
				util.FilePutContentWithByte(tlsCfg.TLSCertFile, instanceCertPEM)
				util.FilePutContentWithByte(tlsCfg.TLSKeyFile, instanceKeyPEM)
			}
		}


		//load cert files
		cfg.Certificates = make([]tls.Certificate, 1)
		cfg.Certificates[0], err = tls.LoadX509KeyPair(tlsCfg.TLSCertFile, tlsCfg.TLSKeyFile)

		//setup if need verify certs
		if !tlsCfg.TLSInsecureSkipVerify {
			if certPool == nil {
				certPool = x509.NewCertPool()
			}
			if len(rootCertPEM) == 0 && util.FileExists(tlsCfg.TLSCACertFile) {
				rootCertPEM, err = ioutil.ReadFile(tlsCfg.TLSCACertFile)
				if err != nil {
					return nil, err
				}
			}
			certPool.AppendCertsFromPEM(rootCertPEM)
			cfg.ClientCAs = certPool
			cfg.ClientAuth = tls.RequireAndVerifyClientCert
			log.Info("TLS client required to verify client cert")
		}
	}
	return cfg, err
}

func AutoIssueTLSCertificates(tlsCfg *config.TLSConfig,cfg *tls.Config) {
	zapCfg := zap.NewProductionConfig()
	zapCfg.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	logger := zap.Must(zapCfg.Build())
	certmagic.Default.Logger = logger

	// Configure CertMagic to use the DNS-01 challenge
	certmagic.DefaultACME.DNS01Solver = &certmagic.DNS01Solver{
		DNSManager: certmagic.DNSManager{
			PropagationDelay: 30 * time.Second,
			DNSProvider: &tencentcloud.Provider{
				SecretId:  tlsCfg.AutoIssue.Provider.TencentDNS.SecretID,
				SecretKey: tlsCfg.AutoIssue.Provider.TencentDNS.SecretKey,
			},
		},
	}

	// Set up the domains you want to obtain a certificate for
	domains := tlsCfg.AutoIssue.Domains
	if tlsCfg.AutoIssue.IncludeDefaultDomain && tlsCfg.DefaultDomain != "" {
		domains = append(domains, tlsCfg.DefaultDomain)
	}

	//TODO skip_invalid_domain

	if len(domains) == 0 {
		panic(errors.Errorf("please setup domain for auto issue"))
	}

	// Use certmagic.NewDefault to create a new CertMagic config
	magic := certmagic.NewDefault()
	myACME := certmagic.NewACMEIssuer(magic, certmagic.ACMEIssuer{
		CA:     certmagic.LetsEncryptProductionCA,
		Email:  tlsCfg.AutoIssue.Email,
		Agreed: true,
		Logger: logger,
	})

	certPath := tlsCfg.AutoIssue.Path
	if certPath == "" {
		certPath = util.JoinPath(global.Env().GetDataDir(), ".auto_issued_certs")
	}
	magic.Storage = &certmagic.FileStorage{Path: certPath}
	magic.Issuers = []certmagic.Issuer{myACME}
	magic.Logger = logger

	seelog.Infof("start issuing certs for domain: [%v]", util.JoinArray(domains, ", "))
	// Manage certificates with CertMagic
	err := magic.ManageSync(context.Background(), domains)
	if err != nil {
		panic(err)
	}

	seelog.Infof("issued certs for domain: [%v]", util.JoinArray(domains, ","))

	cfg.GetCertificate = magic.GetCertificate
	cfg.NextProtos = append([]string{"h2", "http/1.1"}, cfg.NextProtos...)
}
