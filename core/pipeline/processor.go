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

package pipeline

type Joint interface {
	Name() string
}

type Processor interface {
	Joint
	Process(s *Context) error
}

type ComplexProcessor interface {
	Processor
}

type Input interface {
	Joint
	Open() error
	Close() error
	Read() ([]byte, error)
}

type Output interface {
	Joint
	Open() error
	Close() error
	Write([]byte) error
}

type Filter interface {
	Joint
	Filter([]byte) error
}
