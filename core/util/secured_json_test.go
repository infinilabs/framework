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

package util

import (
	"encoding/json"
	"strings"
	"testing"
)

type User struct {
	Name   string `json:"name"`
	Secret string `json:"secret,secured"`
}

func TestMaskFields(t *testing.T) {
	data := `{"name": "John Doe", "secret": "supersecretpassword"}`
	var user User

	// Deserialize JSON into the struct
	err := json.Unmarshal([]byte(data), &user)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Apply masking
	MaskFields(&user)

	//fmt.Println(util.MustToJSON(user))

	// Check if the secret field is masked correctly
	expectedMask := strings.Repeat("*", len("supersecretpassword"))
	if user.Secret != expectedMask {
		t.Errorf("Expected Secret to be masked as '%s', got '%s'", expectedMask, user.Secret)
	}

	// Check if the name field is unaffected
	if user.Name != "John Doe" {
		t.Errorf("Expected Name to remain 'John Doe', got '%s'", user.Name)
	}
}

func TestMaskAndMarshal(t *testing.T) {
	user := &User{
		Name:   "John Doe",
		Secret: "supersecretpassword",
	}

	// Apply masking and marshal
	result, err := MaskAndMarshal(user)
	if err != nil {
		t.Fatalf("MaskAndMarshal failed: %v", err)
	}

	//fmt.Println(string(result))

	// Check the output
	expectedJSON := `{"name":"John Doe","secret":"*******************"}`
	if string(result) != expectedJSON {
		t.Errorf("Expected JSON '%s', got '%s'", expectedJSON, string(result))
	}
}

// User struct with a custom MarshalJSON implementation
type CustomUser struct {
	Name   string `json:"name"`
	Secret string `json:"secret,secured"`
}

// MarshalJSON masks the "secured" fields during JSON serialization
func (u *CustomUser) MarshalJSON() ([]byte, error) {
	type Alias CustomUser // Avoid recursion by using an alias
	alias := &struct {
		Secret string `json:"secret"`
		*Alias
	}{
		Alias: (*Alias)(u),
	}

	// Mask the secured field
	alias.Secret = strings.Repeat("*", len(u.Secret))

	return json.Marshal(alias)
}

func TestMaskAndMarshalWithoutCopy(t *testing.T) {
	user := &CustomUser{
		Name:   "John Doe",
		Secret: "supersecretpassword",
	}

	// Marshal directly with masking
	result, err := json.Marshal(user)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Check that the output contains the expected masked secret
	expectedSnippet := `"secret":"*******************"`
	if !strings.Contains(string(result), expectedSnippet) {
		t.Errorf("Expected JSON to contain '%s', but got '%s'", expectedSnippet, string(result))
	}

	// Optionally, verify that the "Name" field exists
	if !strings.Contains(string(result), `"name":"John Doe"`) {
		t.Errorf("Expected JSON to contain '\"name\":\"John Doe\"', but got '%s'", string(result))
	}
}
