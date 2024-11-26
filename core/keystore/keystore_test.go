/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package keystore

import (
	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/elastic"
	"os"
	"path"
	"testing"
)

func TestConfigVariable(t *testing.T){
	wd, _ := os.Getwd()
	err := os.Setenv(PathEnvKey, wd)
	if err != nil {
		t.Fatal(err)
	}
	password := "hello123"
	err = SetValue("ES_PASSWORD", []byte(password))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := GetVariableResolver()
	if err != nil {
		t.Fatal(err)
	}
	config.RegisterOption("keystore", resolver)
	cfg, err := config.LoadFile("test_config.yml")
	if err != nil {
		t.Fatal(err)
	}
	esConfigs := []elastic.ElasticsearchConfig{}
	esCfg, err := cfg.Child("elasticsearch", -1)
	if err != nil {
		t.Fatal(err)
	}
	err = esCfg.Unpack(&esConfigs)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, password, esConfigs[0].BasicAuth.Password.Get())
	os.RemoveAll(path.Join(wd, ".keystore"))
}