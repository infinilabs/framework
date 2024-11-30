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