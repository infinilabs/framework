/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rpc

type RPCConfig struct {
	BindingAddr           string `json:"binding_addr,omitempty" config:"binding_addr"`
	TLSEnabled            bool   `config:"tls_enabled"`
	TLSCertFile           string `config:"tls_cert_file"`
	TLSKeyFile            string `config:"tls_key_file"`
	TLSInsecureSkipVerify bool   `config:"tls_skip_insecure_verify"`
}
