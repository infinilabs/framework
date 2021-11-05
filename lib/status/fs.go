// +build !windows

/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */
package status

import (
	"syscall"
)

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
	Available uint64 `json:"available"` //available = free - reserved filesystem blocks(for root)

}

// disk usage of path/disk
func DiskUsage(path string) (disk DiskStatus) {
	sf := syscall.Statfs_t{}
	err := syscall.Statfs(path, &sf)
	if err != nil {
		return
	}
	disk.All = sf.Blocks * uint64(sf.Bsize)
	disk.Free = sf.Bfree * uint64(sf.Bsize)
	disk.Available = sf.Bavail * uint64(sf.Bsize)
	disk.Used = disk.All - disk.Free
	return
}
