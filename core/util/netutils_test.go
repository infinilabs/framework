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
	"github.com/stretchr/testify/assert"
	"net"
	"strconv"
	"testing"
)

func TestTestPort(t *testing.T) {
	port := 42122
	res := TestPort(port)
	assert.Equal(t, true, res)

	ln, _ := net.Listen("tcp", ":"+strconv.Itoa(port))

	res = TestPort(port)
	assert.Equal(t, false, res)
	ln.Close()
}

func TestGetAvailablePort(t *testing.T) {
	port := 42123
	res := TestPort(port)
	assert.Equal(t, true, res)

	ln, _ := net.Listen("tcp", ":"+strconv.Itoa(port))
	defer ln.Close()

	p1 := GetAvailablePort("", port)
	assert.Equal(t, 42124, p1)
}

func TestGetAvailablePort2(t *testing.T) {
	port := 42123

	for i := 0; i < 1000; i++ {
		p1 := GetAvailablePort("", port)
		assert.Equal(t, 42123, p1)
	}
}

func TestAutoGetAddress(t *testing.T) {
	port := 42123
	res := TestPort(port)
	assert.Equal(t, true, res)

	ln, _ := net.Listen("tcp", ":"+strconv.Itoa(port))

	var p1 string

	p1 = AutoGetAddress(":42123")
	assert.Equal(t, ":42124", p1)
	ln.Close()

	ln, _ = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))

	p1 = AutoGetAddress("127.0.0.1:42123")
	assert.Equal(t, "127.0.0.1:42124", p1)
	ln.Close()

}

func TestGetValidAddress(t *testing.T) {
	addr := ":8001"
	addr = GetValidAddress(addr)
	assert.Equal(t, "127.0.0.1:8001", addr)
}

func TestGetIntranetIP(t *testing.T) {
	ip, _ := GetIntranetIP()
	fmt.Println(ip)
}

func TestGetAutoIP(t *testing.T) {
	ip:= GetSafetyInternalAddress("0.0.0.0:8888")
	fmt.Println(ip)
}


func TestGetAddress(t *testing.T) {
	dev,ip,mask,_:=GetPublishNetworkDeviceInfo("")
	fmt.Println(dev,ip,mask)
}
