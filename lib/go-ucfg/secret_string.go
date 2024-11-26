/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package ucfg

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SecretString string

func EncodeToSecretString(raw string, value string) SecretString {
	return SecretString(fmt.Sprintf("%s%s%s", raw, SecretStringDelimiter, value))
}

//var ErrMalformed = errors.New("secret string malformed")
const (
	SecretStringDelimiter = "-->"
	SecretShadowText       = "******"
)
func (k SecretString) Get() string {
	_, v := k.decode()
	return v
}

func (k SecretString) String() string {
	return k.getRaw()
}
func (k SecretString) getRaw() string {
	raw, _ := k.decode()
	return raw
}
func (k SecretString) GoString() string {
	return k.getRaw()
}

func (k SecretString) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.getRaw())
}
func (k *SecretString) UnmarshalJSON(src []byte) error {
	var input string
	err := json.Unmarshal(src, &input)
	if err != nil {
		return err
	}
	*k = SecretString(input)
	return nil
}

func (k SecretString) MarshalYAML() (interface{}, error) {
	return fmt.Sprintf(`%s`, k.getRaw()), nil
}
func (k SecretString) decode() (string, string) {
	return DecodeSecretString(string(k))
}

func DecodeSecretString(s string) (string, string) {
	parts := strings.Split(s, SecretStringDelimiter)
	//case of plain text, no secret variable metadata
	if len(parts) == 1 {
		return SecretShadowText, parts[0]
	}
	return parts[0], parts[1]
}

func (k *SecretString) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw string
	err := unmarshal(&raw)
	if err != nil {
		return err
	}
	*k = SecretString(raw)
	return nil
}
