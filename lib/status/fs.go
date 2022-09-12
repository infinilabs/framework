
/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */
package status

import (
	log "github.com/cihub/seelog"
	disk2 "github.com/shirou/gopsutil/disk"
)

type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
	Available uint64 `json:"available"` //available = free - reserved filesystem blocks(for root)

}

// disk usage of path/disk
func DiskUsage(path string) (disk DiskStatus) {
	stat, err := disk2.Usage(path)
	if err != nil {
		log.Errorf("status.DiskUsage, err: %v",err)
		return
	}
	disk.All = stat.Total
	disk.Free = stat.Free
	disk.Available = stat.Free
	disk.Used = stat.Used
	return
}
