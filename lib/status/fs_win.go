// +build windows

package status
import (
	"syscall"
	"unsafe"
)
type DiskStatus struct {
	All  uint64 `json:"all"`
	Used uint64 `json:"used"`
	Free uint64 `json:"free"`
	Available uint64 `json:"available"` //available = free - reserved filesystem blocks(for root)
}

// disk usage of path/disk
func DiskUsage(path string) (disk DiskStatus) {

	h := syscall.MustLoadDLL("kernel32.dll")
	c := h.MustFindProc("GetDiskFreeSpaceExW")
	lpFreeBytesAvailable := uint64(0)
	lpTotalNumberOfBytes := uint64(0)
	lpTotalNumberOfFreeBytes := uint64(0)
	_, _, err := c.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("F:"))),
		uintptr(unsafe.Pointer(&lpFreeBytesAvailable)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfBytes)),
		uintptr(unsafe.Pointer(&lpTotalNumberOfFreeBytes)))

	if err != nil {
		return
	}
	disk.All = lpTotalNumberOfBytes
	disk.Available = lpFreeBytesAvailable
	disk.Free = lpTotalNumberOfFreeBytes
	disk.Used = disk.All - disk.Free
	return
}
