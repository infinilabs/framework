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
	"regexp"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v4/net"
)

// detectNetworkBandwidthMbps detects the maximum network interface speed in Mbps.
// On macOS, it uses system_profiler to get network interface speeds.
// Returns 0 if detection fails.
func detectNetworkBandwidthMbps() float64 {
	interfaces, err := net.IOCounters(true)
	if err != nil {
		log.Debugf("overall: failed to get network interfaces: %v", err)
		return 0
	}

	var maxSpeed float64
	speedRegex := regexp.MustCompile(`(\d+)-baseT|(\d+)BASE-T|Speed:\s*(\d+)\s*Mbps`)

	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if iface.Name == "lo0" || strings.HasPrefix(iface.Name, "utun") ||
			strings.HasPrefix(iface.Name, "bridge") || strings.HasPrefix(iface.Name, "awdl") {
			continue
		}

		// Try using networksetup to get link speed
		out, err := exec.Command("networksetup", "-getMedia", iface.Name).Output()
		if err == nil {
			output := string(out)
			matches := speedRegex.FindStringSubmatch(output)
			for _, match := range matches {
				if match != "" {
					speed, err := strconv.ParseFloat(match, 64)
					if err == nil && speed > 0 && speed > maxSpeed {
						maxSpeed = speed
						log.Debugf("overall: detected interface %s speed: %.0f Mbps", iface.Name, speed)
					}
				}
			}
		}

		// Try ifconfig for link speed
		out, err = exec.Command("ifconfig", iface.Name).Output()
		if err == nil {
			output := string(out)
			// Look for "media: autoselect (1000baseT <full-duplex>)"
			if strings.Contains(output, "1000baseT") || strings.Contains(output, "1000BASE-T") {
				if maxSpeed < 1000 {
					maxSpeed = 1000
					log.Debugf("overall: detected interface %s speed: 1000 Mbps (from ifconfig)", iface.Name)
				}
			} else if strings.Contains(output, "100baseT") || strings.Contains(output, "100BASE-T") {
				if maxSpeed < 100 {
					maxSpeed = 100
					log.Debugf("overall: detected interface %s speed: 100 Mbps (from ifconfig)", iface.Name)
				}
			} else if strings.Contains(output, "10baseT") || strings.Contains(output, "10BASE-T") {
				if maxSpeed < 10 {
					maxSpeed = 10
					log.Debugf("overall: detected interface %s speed: 10 Mbps (from ifconfig)", iface.Name)
				}
			} else if strings.Contains(output, "10Gbase") || strings.Contains(output, "10GBASE") {
				if maxSpeed < 10000 {
					maxSpeed = 10000
					log.Debugf("overall: detected interface %s speed: 10000 Mbps (from ifconfig)", iface.Name)
				}
			}
		}
	}

	if maxSpeed > 0 {
		log.Infof("overall: auto-detected network bandwidth: %.0f Mbps", maxSpeed)
	}
	return maxSpeed
}
