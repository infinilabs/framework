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

//go:build darwin

package overall

import (
	"os/exec"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v4/net"
)

// detectNetworkBandwidthPerInterface detects network interface speeds in Mbps for each interface.
// On macOS, it uses ifconfig to get network interface speeds.
// Returns a map of interface name to bandwidth in Mbps.
func detectNetworkBandwidthPerInterface() map[string]float64 {
	result := make(map[string]float64)

	interfaces, err := net.IOCounters(true)
	if err != nil {
		log.Debugf("overall: failed to get network interfaces: %v", err)
		return result
	}

	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if isVirtualInterface(iface.Name) {
			continue
		}

		speed := detectDarwinInterfaceSpeed(iface.Name)
		if speed > 0 {
			log.Debugf("overall: detected interface %s speed: %.0f Mbps", iface.Name, speed)
			result[iface.Name] = speed
		}
	}

	return result
}

// detectDarwinInterfaceSpeed attempts to detect the speed of a single interface on macOS
func detectDarwinInterfaceSpeed(ifaceName string) float64 {
	// Try ifconfig for link speed
	out, err := exec.Command("ifconfig", ifaceName).Output()
	if err != nil {
		return 0
	}

	output := string(out)

	// Look for "media: autoselect (1000baseT <full-duplex>)"
	if strings.Contains(output, "10Gbase") || strings.Contains(output, "10GBASE") {
		return 10000
	}
	if strings.Contains(output, "1000baseT") || strings.Contains(output, "1000BASE-T") {
		return 1000
	}
	if strings.Contains(output, "100baseT") || strings.Contains(output, "100BASE-T") {
		return 100
	}
	if strings.Contains(output, "10baseT") || strings.Contains(output, "10BASE-T") {
		return 10
	}

	return 0
}
