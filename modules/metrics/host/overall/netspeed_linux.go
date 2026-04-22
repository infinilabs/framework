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

//go:build linux

package overall

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v4/net"
)

// detectNetworkBandwidthMbps detects the maximum network interface speed in Mbps.
// On Linux, it reads from /sys/class/net/<iface>/speed for each interface.
// Returns 0 if detection fails.
func detectNetworkBandwidthMbps() float64 {
	interfaces, err := net.IOCounters(true)
	if err != nil {
		log.Debugf("overall: failed to get network interfaces: %v", err)
		return 0
	}

	var maxSpeed float64
	for _, iface := range interfaces {
		// Skip loopback and virtual interfaces
		if iface.Name == "lo" || strings.HasPrefix(iface.Name, "veth") ||
			strings.HasPrefix(iface.Name, "docker") || strings.HasPrefix(iface.Name, "br-") {
			continue
		}

		speedPath := fmt.Sprintf("/sys/class/net/%s/speed", iface.Name)
		data, err := os.ReadFile(speedPath)
		if err != nil {
			log.Debugf("overall: failed to read speed for %s: %v", iface.Name, err)
			continue
		}

		speedStr := strings.TrimSpace(string(data))
		speed, err := strconv.ParseFloat(speedStr, 64)
		if err != nil || speed <= 0 {
			// Speed might be -1 if link is down or unknown
			continue
		}

		log.Debugf("overall: detected interface %s speed: %.0f Mbps", iface.Name, speed)
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	if maxSpeed > 0 {
		log.Infof("overall: auto-detected network bandwidth: %.0f Mbps", maxSpeed)
	}
	return maxSpeed
}
