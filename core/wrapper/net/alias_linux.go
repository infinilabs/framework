package net

import (
	"fmt"
	"os/exec"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

const alias = "infini"

// Linux
// sudo /sbin/ifconfig eth0:1 192.168.3.198 netmask 255.255.255.0
// sudo /sbin/ifconfig eth0:1 down
func SetupAlias(device, ip, netmask string) error {
	checkPermission()
	log.Debugf("setup net alias %s, %s, %s", device, ip, netmask)
	setupVIP := exec.Command("/sbin/ifconfig", fmt.Sprintf("%s:%s", device, alias), ip, "netmask", netmask)
	data, err := setupVIP.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to set floating IP on interface: %s, %s", string(data), err))
	}

	ok, err := util.CheckIPBinding(ip)
	if !ok || err != nil {
		return errors.New(fmt.Sprintf("failed to locate interface by alias %s: %s\n", device, err))
	}

	////act as backup, disable alias by default
	//DisableAlias(device,ip,netmask)

	//register global callback to disable alias before shutdown
	global.RegisterShutdownCallback(func() {
		DisableAlias(device, ip, netmask)
	})

	log.Debug("ip alias was successfully setup/enabled")

	return nil
}

// sudo /sbin/ifconfig eth0:1 up
func EnableAlias(device, ip string, netmask string) error {

	checkPermission()

	log.Debugf("enable net alias %s, %s, %s", device, ip, netmask)
	setupVIP := exec.Command("/sbin/ifconfig", fmt.Sprintf("%s:%s", device, alias), "up")
	_, err := setupVIP.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to enable alias on interface: %s", err))
	}

	ok, err := util.CheckIPBinding(ip)
	if !ok || err != nil {
		return errors.New(fmt.Sprintf("failed to get alias on interface %s: %s\n", device, err))
	}

	log.Debug("ip alias was successfully setup/enabled")

	return nil
}

//OSX
//sudo /sbin/ifconfig en0 -alias 192.168.3.213

//Linux
///sbin/ifdown device

// Windows
// netsh interface set interface name="INFINI Ethernet" admin=DISABLED
func DisableAlias(device, ip string, netmask string) error {

	checkPermission()

	log.Debugf("disable net alias %s, %s, %s", device, ip, netmask)
	setupVIP := exec.Command("/sbin/ifconfig", fmt.Sprintf("%s:%s", device, alias), "down")
	_, err := setupVIP.CombinedOutput()
	if err != nil {
		return errors.New(fmt.Sprintf("failed to disable alias on interface: %s", err))
	}

	ok, err := util.CheckIPBinding(ip)
	if ok || err != nil {
		return errors.New(fmt.Sprintf("failed to disable alias on interface %s: %s\n", device, err))
	}

	log.Debug("ip alias was successfully disabled")

	return nil
}
