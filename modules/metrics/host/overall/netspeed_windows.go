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

//go:build windows

package overall

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	log "github.com/cihub/seelog"
)

// detectNetworkBandwidthMbps detects the maximum network interface speed in Mbps.
// On Windows, it uses PowerShell/WMI to query network adapter speeds.
// Returns 0 if detection fails.
func detectNetworkBandwidthMbps() float64 {
	// Use PowerShell to get network adapter speeds
	cmd := exec.Command("powershell", "-Command",
		"Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | Select-Object -ExpandProperty LinkSpeed")
	out, err := cmd.Output()
	if err != nil {
		log.Debugf("overall: failed to get network adapter speed via PowerShell: %v", err)
		return tryWMIC()
	}

	var maxSpeed float64
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		speed := parseWindowsLinkSpeed(line)
		if speed > maxSpeed {
			maxSpeed = speed
		}
	}

	if maxSpeed > 0 {
		log.Infof("overall: auto-detected network bandwidth: %.0f Mbps", maxSpeed)
	}
	return maxSpeed
}

// tryWMIC tries to get network speed using wmic (fallback for older Windows)
func tryWMIC() float64 {
	cmd := exec.Command("wmic", "nic", "where", "NetEnabled=true", "get", "Speed")
	out, err := cmd.Output()
	if err != nil {
		log.Debugf("overall: failed to get network adapter speed via wmic: %v", err)
		return 0
	}

	var maxSpeed float64
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || line == "Speed" {
			continue
		}
		// Speed from wmic is in bits per second
		bps, err := strconv.ParseFloat(line, 64)
		if err != nil || bps <= 0 {
			continue
		}
		mbps := bps / 1000000.0
		if mbps > maxSpeed {
			maxSpeed = mbps
		}
	}

	if maxSpeed > 0 {
		log.Infof("overall: auto-detected network bandwidth: %.0f Mbps", maxSpeed)
	}
	return maxSpeed
}

// parseWindowsLinkSpeed parses Windows link speed strings like "1 Gbps", "100 Mbps"
func parseWindowsLinkSpeed(linkSpeed string) float64 {
	linkSpeed = strings.TrimSpace(linkSpeed)

	// Match patterns like "1 Gbps", "100 Mbps", "10 Gbps"
	gbpsRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[Gg]bps`)
	mbpsRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[Mm]bps`)
	kbpsRegex := regexp.MustCompile(`(\d+(?:\.\d+)?)\s*[Kk]bps`)

	if matches := gbpsRegex.FindStringSubmatch(linkSpeed); len(matches) > 1 {
		if speed, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return speed * 1000 // Convert Gbps to Mbps
		}
	}
	if matches := mbpsRegex.FindStringSubmatch(linkSpeed); len(matches) > 1 {
		if speed, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return speed
		}
	}
	if matches := kbpsRegex.FindStringSubmatch(linkSpeed); len(matches) > 1 {
		if speed, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return speed / 1000 // Convert Kbps to Mbps
		}
	}

	return 0
}
