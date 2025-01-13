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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package credential

import (
	"fmt"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/go-ucfg"
	keystore2 "infini.sh/framework/lib/keystore"
)

const SecretKey = "credential_secret"

func GetOrInitSecret() ([]byte, error) {
	ks, err := keystore.GetOrInitKeystore()
	if err != nil {
		return nil, err
	}
	secStr, err := ks.Retrieve(SecretKey)
	if err == keystore2.ErrKeyDoesntExists {
		secBytes, err := util.RandomBytes(32)
		if err != nil {
			return nil, fmt.Errorf("generate credential secret error: %w", err)
		}
		err = InitSecret(ks, secBytes)
		if err != nil {
			return nil, err
		}
		return secBytes, nil
	}
	if err != nil {
		return nil, err
	}
	return secStr.Get()
}

func InitSecret(ks keystore2.Keystore, secret []byte) error {
	var err error
	if ks == nil {
		ks, err = keystore.GetOrInitKeystore()
		if err != nil {
			return err
		}
	}
	ksw, err := keystore2.AsWritableKeystore(ks)
	if err != nil {
		return err
	}

	err = ksw.Store(SecretKey, secret)
	if err != nil {
		return fmt.Errorf("store credential secret error: %w", err)
	}
	err = ksw.Save()
	if err != nil {
		return fmt.Errorf("save credential secret error: %w", err)
	}
	return nil
}

func encodeBasicAuth(cred *Credential) error {
	var (
		params map[string]interface{}
		ok     bool
		pwd    string
	)
	if params, ok = cred.Payload[cred.Type].(map[string]interface{}); !ok {
		return fmt.Errorf("wrong credential parameters for type [%s], expect a map", cred.Type)
	}
	if pwd, ok = params["password"].(string); !ok {
		return fmt.Errorf("wrong credential parameters password for type [%s], expect a string", cred.Type)
	}
	if pwd == "" {
		return fmt.Errorf("credential parameters password can not be empty")
	}
	secret, err := GetOrInitSecret()
	if err != nil {
		return err
	}
	encodeBytes, salt, err := util.AesGcmEncrypt([]byte(pwd), secret)
	if err != nil {
		return fmt.Errorf("encrypt password error: %w", err)
	}
	cred.Encrypt.Type = "AES"
	cred.Encrypt.Params = map[string]interface{}{
		"salt": string(salt),
	}
	params["password"] = string(encodeBytes)
	cred.Payload[cred.Type] = params
	return nil
}

func decodeBasicAuth(cred *Credential) (basicAuth model.BasicAuth, err error) {
	var (
		params map[string]interface{}
		ok     bool
		pwd    string
		salt   string
	)
	if params, ok = cred.Payload[cred.Type].(map[string]interface{}); !ok {
		err = fmt.Errorf("wrong credential parameters for type [%s], expect a map", cred.Type)
		return
	}
	if pwd, ok = params["password"].(string); !ok {
		err = fmt.Errorf("wrong credential parameters password for type [%s], expect a string", cred.Type)
		return
	}
	if pwd == "" {
		err = fmt.Errorf("credential parameters password can not be empty")
		return
	}
	if salt, ok = cred.Encrypt.Params["salt"].(string); !ok {
		err = fmt.Errorf("credential encrypt parameters salt can not be empty")
		return
	}
	var secret = cred.secret
	if secret == nil {
		secret, err = GetOrInitSecret()
		if err != nil {
			return basicAuth, err
		}
	}

	plaintext, err := util.AesGcmDecrypt([]byte(pwd), secret, []byte(salt))
	if err != nil {
		return basicAuth, err
	}
	basicAuth.Username = params["username"].(string)
	basicAuth.Password = ucfg.SecretString(plaintext)
	return
}

type ChangeEvent func(credentials *Credential)

var changeEvents []ChangeEvent

func RegisterChangeEvent(evt ChangeEvent) {
	if evt != nil {
		changeEvents = append(changeEvents, evt)
	}
}
func TriggerChangeEvent(credentials *Credential) {
	for _, cb := range changeEvents {
		if cb != nil {
			cb(credentials)
		}
	}
}
