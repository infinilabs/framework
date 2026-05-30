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

// detectNetworkBandwidthPerInterface detects network interface speeds in Mbps for each interface.
// On Windows, it uses PowerShell/WMI to query network adapter speeds.
// Returns a map of interface name to bandwidth in Mbps.
func detectNetworkBandwidthPerInterface() map[string]float64 {
	result := make(map[string]float64)

	// Use PowerShell to get network adapter speeds with names
	cmd := exec.Command("powershell", "-Command",
		"Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | Select-Object Name,LinkSpeed | ForEach-Object { $_.Name + '|' + $_.LinkSpeed }")
	out, err := cmd.Output()
	if err != nil {
		log.Debugf("overall: failed to get network adapter speed via PowerShell: %v", err)
		return tryWMICPerInterface()
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		linkSpeed := strings.TrimSpace(parts[1])
		speed := parseWindowsLinkSpeed(linkSpeed)
		if speed > 0 {
			log.Debugf("overall: detected interface %s speed: %.0f Mbps", name, speed)
			result[name] = speed
		}
	}

	return result
}

// tryWMICPerInterface tries to get network speed using wmic (fallback for older Windows)
func tryWMICPerInterface() map[string]float64 {
	result := make(map[string]float64)

	cmd := exec.Command("wmic", "nic", "where", "NetEnabled=true", "get", "Name,Speed")
	out, err := cmd.Output()
	if err != nil {
		log.Debugf("overall: failed to get network adapter speed via wmic: %v", err)
		return result
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Name") {
			continue
		}

		// WMIC output is space-separated, speed is the last field
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		speedStr := fields[len(fields)-1]
		name := strings.Join(fields[:len(fields)-1], " ")

		// Speed from wmic is in bits per second
		bps, err := strconv.ParseFloat(speedStr, 64)
		if err != nil || bps <= 0 {
			continue
		}
		mbps := bps / 1000000.0
		log.Debugf("overall: detected interface %s speed: %.0f Mbps", name, mbps)
		result[name] = mbps
	}

	return result
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
