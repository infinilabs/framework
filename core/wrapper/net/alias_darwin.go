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

package net

import (
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	"os/exec"
)

//OSX
//sudo /sbin/ifconfig en0 alias 192.168.3.213  255.255.255.0
//sudo /sbin/ifconfig en0 -alias 192.168.3.213

//Linux
//sudo /sbin/ifconfig eth0:1 192.168.3.198 netmask 255.255.255.0
//sudo /sbin/ifconfig eth0:1 down

//Windows
//netsh -c Interface ip add address name="INFINI Ethernet" addr=10.10.0.21 mask=255.255.0.0
//netsh -c Interface ip delete address name="INFINI Ethernet" addr=10.10.0.21
func SetupAlias(device, ip, netmask string) error {

	err:=EnableAlias(device, ip, netmask)
	if err != nil {
		return err
	}

	//register global callback to disable alias before shutdown
	global.RegisterShutdownCallback(func() {
		DisableAlias(device, ip, netmask)
	})

	return nil
}

func EnableAlias(device, ip string, netmask string) error {

	checkPermission()

	if !util.FilesExists("/sbin/ifconfig"){
		return errors.New("net alias not supported on your platform.")
	}

	log.Debugf("setup net alias %s, %s, %s", device, ip, netmask)
	setupVIP := exec.Command("/sbin/ifconfig", device, "alias", ip, netmask)
	data, err := setupVIP.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to set alias on interface: %s, %s",string(data), err))
	}

	ok, err := util.CheckIPBinding(ip)
	if !ok || err != nil {
		return errors.New(fmt.Sprintf("failed to get interface by alias %s: %s\n", device, err))
	}

	log.Debug("net alias was successfully setup/enabled")

	return nil
}

//OSX
//sudo /sbin/ifconfig en0 -alias 192.168.3.213

//Linux
///sbin/ifdown device

//Windows
//netsh interface set interface name="INFINI Ethernet" admin=DISABLED
func DisableAlias(device, ip string, netmask string) error {
	checkPermission()

	if !util.FilesExists("/sbin/ifconfig"){
		return errors.New("net alias not supported on your platform.")
	}

	setupVIP := exec.Command( "/sbin/ifconfig", device, "-alias", ip)
	_, err := setupVIP.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to disable alias on interface: %s", err))
	}

	ok, err := util.CheckIPBinding(ip)
	if ok || err != nil {
		return errors.New(fmt.Sprintf("failed to disable alias on interface %s: %s\n", device, err))
	}

	log.Debug("net alias was successfully disabled")

	return nil
}
