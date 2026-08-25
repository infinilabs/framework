/* Copyright © INFINI Ltd. All rights reserved. */

package model

// Token is a bearer credential carried on registered instances (the
// agent's self-generated API token, stored at registration).
type Token struct {
	Value string `json:"value,omitempty" config:"value"`
}
