/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package keystore

import (
	"fmt"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/keystore"
	"os"
	"path/filepath"
	"sync"
)

var (
	defaultKeystore keystore.Keystore
	keystoreOnce sync.Once
)

func GetOrInitKeystore() (keystore.Keystore, error) {
	var err error
	keystoreOnce.Do(func() {
		defaultKeystore, err = initKeystore()
	})
	return defaultKeystore, err
}

func GetWriteableKeystore()(keystore.WritableKeystore, error) {
	ks, err := GetOrInitKeystore()
	if err != nil {
		return nil, err
	}
	return keystore.AsWritableKeystore(ks)
}

func initKeystore() (keystore.Keystore, error){
	keystorePath := filepath.Join(global.Env().GetDataDir(), ".keystore")
	if !util.FileExists(keystorePath){
		err := os.Mkdir(keystorePath, 0750)
		if err != nil {
			return nil, err
		}
	}
	storePath := filepath.Join(keystorePath, "ks")
	keyPath := filepath.Join(keystorePath, "key")
	var password *keystore.SecureString
	if !util.FileExists(keyPath){
		randStr := util.GenerateRandomString(32)
		password = keystore.NewSecureString([]byte(randStr))
		_, err := util.FilePutContent(keyPath, randStr)
		if err != nil {
			return nil, fmt.Errorf("save keystore password to file error: %w", err)
		}
	}else{
		passwordBytes, err := util.FileGetContent(keyPath)
		if err != nil {
			return nil, fmt.Errorf("read keystore password file error: %w", err)
		}
		password = keystore.NewSecureString(passwordBytes)
	}

	return keystore.NewFileKeystoreWithPassword(storePath, password)
}