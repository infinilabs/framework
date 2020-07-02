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

package dag

import "infini.sh/crawler/pipeline/codec"

type Context struct {
	Channel chan []byte
	Codec   Codec

	//IntMap map[string]int
	//ByteMap map[string]byte
	//StringMap map[string]byte
	//ObjMap map[string]interface{}
}

func GetContext() *Context {

	context := Context{}
	context.Channel = make(chan []byte)
	context.Codec = &codec.JSONCodec{}

	return &context
}

type Codec interface {
	Encode(in []byte) []byte
	Decode(in []byte) []byte
}

type Input interface {
	Read()
}

type Processor interface {
}

type Output interface {
	Write()
}

type FetchPipeline struct {
	Inputs []Input

	Processors []Processor

	Outputs []Output
}
