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
	"net/http"
	"reflect"
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
	_ = ln.Close()
}

func TestGetAvailablePort(t *testing.T) {
	port := 42123
	res := TestPort(port)
	assert.Equal(t, true, res)

	ln, _ := net.Listen("tcp", ":"+strconv.Itoa(port))
	defer func(ln net.Listener) {
		_ = ln.Close()
	}(ln)

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
	_ = ln.Close()

	ln, _ = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))

	p1 = AutoGetAddress("127.0.0.1:42123")
	assert.Equal(t, "127.0.0.1:42124", p1)
	_ = ln.Close()
}

func TestGetIntranetIP(t *testing.T) {
	ip, _ := GetIntranetIP()
	fmt.Println(ip)
}

func TestGetAutoIP(t *testing.T) {
	ip := GetSafetyInternalAddress("0.0.0.0:8888")
	fmt.Println(ip)
}

func TestGetAddress(t *testing.T) {
	dev, ip, mask, _ := GetPublishNetworkDeviceInfo("")
	fmt.Println(dev, ip, mask)
}

// TestIsPublicIP verifies the logic for identifying public vs. private IP addresses.
func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "Public IP (Google DNS)", ip: "8.8.8.8", want: true},
		{name: "Private IP (Class A)", ip: "10.0.0.1", want: false},
		{name: "Private IP (Class B)", ip: "172.16.0.1", want: false},
		{name: "Private IP (Class C)", ip: "192.168.1.1", want: false},
		{name: "Loopback IP", ip: "127.0.0.1", want: false},
		{name: "Link-Local IP", ip: "169.254.0.1", want: false},
		{name: "IPv6 Loopback", ip: "::1", want: false},
		{name: "Invalid IP", ip: "999.999.999.999", want: true}, // net.ParseIP returns nil, so IsPublicIP returns false for non-IPs. Let's test the behavior.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			// Handle the case where the IP is invalid, as net.ParseIP will return nil.
			if ip == nil && tt.name == "Invalid IP" {
				// We expect IsPublicIP(nil) to be false.
				if IsPublicIP(ip) != false {
					t.Errorf("IsPublicIP(nil) got true, want false")
				}
				return
			}
			if got := IsPublicIP(ip); got != tt.want {
				t.Errorf("IsPublicIP(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// TestUnifyLocalAddress checks the normalization of various loopback address formats.
func TestUnifyLocalAddress(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "localhost", host: "localhost", want: "127.0.0.1"},
		{name: "localhost with port", host: "localhost:9200", want: "127.0.0.1:9200"},
		{name: "IPv6 loopback", host: "::1", want: "127.0.0.1"},
		{name: "IPv6 loopback with brackets", host: "[::1]:8080", want: "127.0.0.1:8080"},
		{name: "Regular IP", host: "192.168.1.1", want: "192.168.1.1"},
		{name: "No change", host: "example.com", want: "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UnifyLocalAddress(tt.host); got != tt.want {
				t.Errorf("UnifyLocalAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetValidAddress ensures that addresses with a missing host part are correctly defaulted.
func TestGetValidAddress(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "Missing host", addr: ":8001", want: "127.0.0.1:8001"},
		{name: "Host present", addr: "192.168.1.1:8080", want: "192.168.1.1:8080"},
		{name: "IPv6 no change", addr: "[::1]:8080", want: "[::1]:8080"},
		{name: "No port", addr: "example.com", want: "example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetValidAddress(tt.addr); got != tt.want {
				t.Errorf("GetValidAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsLocalAddress verifies the logic for checking if an address is local.
func TestIsLocalAddress(t *testing.T) {
	localIPs := []string{"192.168.1.10", "10.0.0.5"}

	tests := []struct {
		name    string
		address []string
		want    bool
	}{
		{name: "Direct local IP match", address: []string{"192.168.1.10:9200"}, want: true},
		{name: "Another local IP match", address: []string{"10.0.0.5"}, want: true},
		{name: "External IP", address: []string{"8.8.8.8"}, want: false},
		// Based on your function's logic, "localhost" and "0.0.0.0" are skipped (continue), so they don't cause a 'true' return.
		{name: "localhost is skipped", address: []string{"localhost:9200"}, want: false},
		{name: "AnyAddress is skipped", address: []string{"0.0.0.0:8080"}, want: false},
		{name: "Multiple addresses, one local", address: []string{"8.8.8.8", "192.168.1.10"}, want: true},
		{name: "Empty list", address: []string{}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsLocalAddress(tt.address, localIPs); got != tt.want {
				t.Errorf("IsLocalAddress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestHasDomainSuffix verifies the domain/subdomain matching logic.
func TestHasDomainSuffix(t *testing.T) {
	list := []string{"google.com", "example.com:8080", "localhost"}

	tests := []struct {
		name string
		addr string
		want bool
	}{
		{name: "Subdomain match", addr: "docs.google.com", want: true},
		{name: "Subdomain with port", addr: "api.google.com:443", want: true},
		{name: "Exact domain match", addr: "google.com", want: true},
		{name: "Exact domain with port match", addr: "example.com:8080", want: true},
		{name: "Exact domain with different port (no match)", addr: "example.com:9999", want: false},
		{name: "Exact localhost match", addr: "localhost:9200", want: true},
		{name: "No match", addr: "my-google.com", want: false},
		{name: "Different domain", addr: "another.com", want: false},
		{name: "Empty list", addr: "google.com", want: false}, // Test with an empty list
	}

	for _, tt := range tests {
		var currentList []string
		if tt.name == "Empty list" {
			currentList = []string{}
		} else {
			currentList = list
		}

		t.Run(tt.name, func(t *testing.T) {
			if got := HasDomainSuffix(tt.addr, currentList); got != tt.want {
				t.Errorf("HasDomainSuffix(%s) = %v, want %v", tt.addr, got, tt.want)
			}
		})
	}
}

// TestClientIP verifies IP extraction from http.Request.
func TestClientIP(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
		want    string
	}{
		{
			name:    "X-Forwarded-For (single)",
			request: &http.Request{Header: http.Header{"X-Forwarded-For": []string{"203.0.113.195"}}},
			want:    "203.0.113.195",
		},
		{
			name:    "X-Forwarded-For (multiple, takes first)",
			request: &http.Request{Header: http.Header{"X-Forwarded-For": []string{"203.0.113.195, 70.41.3.18, 150.172.238.178"}}},
			want:    "203.0.113.195",
		},
		{
			name:    "X-Real-Ip",
			request: &http.Request{Header: http.Header{"X-Real-Ip": []string{"198.51.100.10"}}},
			want:    "198.51.100.10",
		},
		{
			name:    "RemoteAddr (IPv4)",
			request: &http.Request{RemoteAddr: "192.0.2.1:12345"},
			want:    "192.0.2.1",
		},
		{
			name:    "RemoteAddr (IPv6 loopback)",
			request: &http.Request{RemoteAddr: "[::1]:12345"},
			want:    "127.0.0.1",
		},
		{
			name:    "No IP found",
			request: &http.Request{Header: http.Header{}, RemoteAddr: "invalid"},
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClientIP(tt.request); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ClientIP() = %v, want %v", got, tt.want)
			}
		})
	}
}
