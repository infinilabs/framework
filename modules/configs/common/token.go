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
	"strings"
	"sync"

	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/util"
	keystore2 "infini.sh/framework/lib/keystore"
)

// These helpers persist transport tokens directly in the local keystore.
// Server-side persisted credentials still use core/credential.
var tokenKeystoreLock sync.Mutex

func LoadTokenFromKeystore(key string) (string, error) {
	value, err := keystore.GetValue(key)
	if err == keystore2.ErrKeyDoesntExists {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(value)), nil
}

func SaveTokenToKeystore(key, value string) error {
	return keystore.SetValue(key, util.UnsafeStringToBytes(strings.TrimSpace(value)))
}

func EnsureTokenInKeystore(key string) (string, error) {
	tokenKeystoreLock.Lock()
	defer tokenKeystoreLock.Unlock()

	value, err := LoadTokenFromKeystore(key)
	if err != nil {
		return "", err
	}
	if value != "" {
		return value, nil
	}
	value = util.GenerateRandomString(48)
	if err := SaveTokenToKeystore(key, value); err != nil {
		return "", err
	}
	return value, nil
}
