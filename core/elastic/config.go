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

package elastic

import "fmt"

var c = map[string]API{}

func RegisterInstance(elastic string, handler API) {
	if c == nil {
		c = map[string]API{}
	}
	c[elastic] = handler
}

func GetClient(k string) API {
	v, ok := c[k]
	if !ok {
		panic(fmt.Sprintf("elasticsearch client %v was not found", k))
	}
	return v
}
