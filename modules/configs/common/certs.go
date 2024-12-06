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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package common

import (
	"crypto/x509"
	"encoding/pem"
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/util"
	"os"
	"path"
)

func GetOrInitDefaultCaCerts() (string, string, error) {
	dataDir := global.Env().GetDataDir()
	caFile := path.Join(dataDir, "certs/ca.crt")
	caKey := path.Join(dataDir, "certs/ca.key")
	if !(util.FileExists(caFile) && util.FileExists(caKey)) {
		err := os.MkdirAll(path.Join(dataDir, "certs"), 0775)
		if err != nil {
			return "", "", err
		}
		log.Info("auto generating cert files")
		_, rootKey, rootCertPEM := util.GetRootCert()

		caKeyPEM := pem.EncodeToMemory(&pem.Block{
			Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rootKey),
		})
		_, err = util.FilePutContentWithByte(caKey, caKeyPEM)
		if err != nil {
			return "", "", err
		}
		_, err = util.FilePutContentWithByte(caFile, rootCertPEM)
		if err != nil {
			return "", "", err
		}
	}
	return caFile, caKey, nil
}
