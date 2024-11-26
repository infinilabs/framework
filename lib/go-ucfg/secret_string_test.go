/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package ucfg

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"testing"
)

func TestMarshalSecretString(t *testing.T){
	var (
		raw = "$[[secret.test]]"
		value = "test123456"
	)
	secStr := EncodeToSecretString(raw, value)
	assert.Equal(t, raw, fmt.Sprintf("%s", secStr))
	buf, err := json.Marshal(secStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, fmt.Sprintf(`"%s"`, raw),  string(buf))
	buf, err = yaml.Marshal(secStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, raw+"\n",  string(buf))
	v := secStr.Get()
	assert.Equal(t, value, v)
}

func TestMarshalPlainTextSecretString(t *testing.T){
	const v = "test123456"
	secStr := SecretString(v)
	assert.Equal(t, SecretShadowText, fmt.Sprintf("%s", secStr))
	buf, err := json.Marshal(secStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, fmt.Sprintf(`"%s"`, SecretShadowText),  string(buf))
	buf, err = yaml.Marshal(secStr)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, fmt.Sprintf("'%s'\n", SecretShadowText),  string(buf))
	assert.Equal(t, v, secStr.Get())
}

func TestUnmarshalSecretString(t *testing.T){
	payload := struct {
		Password SecretString `json:"password" yaml:"password"`
	}{}
	buf := []byte(fmt.Sprintf(`"password": "$[[secret.test]]%stest123456"`, SecretStringDelimiter))
	err := yaml.Unmarshal(buf, &payload)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "$[[secret.test]]", fmt.Sprintf("%s", payload.Password))
}
