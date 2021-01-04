// +build windows freebsd netbsd openbsd

package net

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

	return nil
}

func EnableAlias(device, ip string, netmask string) error {


	return nil
}

//OSX
//sudo /sbin/ifconfig en0 -alias 192.168.3.213

//Linux
///sbin/ifdown device

//Windows
//netsh interface set interface name="INFINI Ethernet" admin=DISABLED
func DisableAlias(device, ip string, netmask string) error {

	return nil
}
