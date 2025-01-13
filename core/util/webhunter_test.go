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

/*
Copyright 2016 Medcl (m AT medcl.net)

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

package util

import (
	"fmt"
	"github.com/magiconair/properties/assert"
	"testing"
)

//func TestGet(t *testing.T) {
//	data, err := HttpGet("http://es-guide-preview.elasticsearch.cn")
//	fmt.Println(data)
//	fmt.Println(err)
//	//data1, _ := json.Marshal(data.Body)
//	//fmt.Println("", string(data1))
//	//assert.Equal(t, data.StatusCode, 301)
//}

func TestGetHost(t *testing.T) {

	url := "/index.html"
	host := GetHost(url)
	fmt.Println("", host)
	assert.Equal(t, host, "")

	url = "www.baidu.com/index.html"
	host = GetHost(url)
	fmt.Println("www.baidu.com", host)
	assert.Equal(t, host, "www.baidu.com")

	url = "//www.baidu.com/index.html"
	host = GetHost(url)
	fmt.Println("www.baidu.com", host)
	assert.Equal(t, host, "www.baidu.com")

	url = "http://www.baidu.com/index.html"
	host = GetHost(url)
	fmt.Println("www.baidu.com", host)
	assert.Equal(t, host, "www.baidu.com")

	url = "https://www.baidu.com/index.html"
	host = GetHost(url)
	fmt.Println("www.baidu.com", host)
	assert.Equal(t, host, "www.baidu.com")

	url = "//baidu.com"
	host = GetHost(url)
	fmt.Println("baidu.com", host)
	assert.Equal(t, host, "baidu.com")

	url = "logo.png"
	host = GetHost(url)
	fmt.Println("logo.png", host)
	assert.Equal(t, host, "")

	url = "logo.com"
	host = GetHost(url)
	fmt.Println("logo.com", host)
	assert.Equal(t, host, "logo.com")
}

func BenchmarkGet(b *testing.B) {

	for i := 0; i < b.N; i++ {
		HttpGet("http://es-guide-preview.elasticsearch.cn")
	}

}

//func TestTimeWait(t *testing.T) {
//	var i int64
//	for {
//		_, err := HttpGet("http://localhost:9200/index/_search/?q=company:A")
//		if err != nil {
//			t.Error(err)
//		}
//		i++
//		//fmt.Printf("%8d\r", i)
//	}
//}

//func TestDefaultTimeout(t *testing.T){
//	req := &Request{
//		Method: "GET",
//		Url: "http://localhost:8090/hello", // server time delay is 80s
//	}
//	_, err := ExecuteRequest(req)
//	if !errors.Is(err, context.DeadlineExceeded) {
//		t.Fatal("expect error of context deadline exceeded")
//	}
//}

//func TestTimeoutWithContext(t *testing.T){
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 90)
//	defer cancel()
//	req := &Request{
//		Method: "GET",
//		Url: "http://localhost:8090/hello",
//		Context: ctx,
//	}
//	result, err := ExecuteRequest(req)
//	if err != nil {
//		t.Fatal(err)
//	}
//	assert.Equal(t, string(result.Body), "hello\n")
//}

//func TestFiveSecondsTimeoutWithContext(t *testing.T){
//	ctx, cancel := context.WithTimeout(context.Background(), time.Second * 5)
//	defer cancel()
//	req := &Request{
//		Method: "GET",
//		Url: "http://localhost:8090/hello",
//		Context: ctx,
//	}
//	_, err := ExecuteRequest(req)
//	if !errors.Is(err, context.DeadlineExceeded) {
//		t.Fatal("expect error of context deadline exceeded")
//	}
//}
