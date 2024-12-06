/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package status

import (
	"github.com/rubyniu105/framework/core/errors"
	"github.com/rubyniu105/framework/core/global"
	"github.com/rubyniu105/framework/core/util"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DiskFsType represents the type of a file system.
type DiskFsType int

const (
	FileFs DiskFsType = iota
	DirectoryFs
)

// DiskFs represents a file or directory.
type DiskFs struct {
	Name       string     `json:"name"`
	Type       DiskFsType `json:"type"`
	Size       uint64     `json:"size"`
	CreateTime time.Time  `json:"create_time"`
	Children   []*DiskFs  `json:"children"`
}

func DirSize(path string) (uint64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})
	return uint64(size), err
}

func getDiskFsType(info os.FileInfo) DiskFsType {
	if info.IsDir() {
		return DirectoryFs
	}
	return FileFs
}

func getDiskFs(path string) ([]*DiskFs, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	diskFs := &DiskFs{
		Name:       info.Name(),
		Type:       getDiskFsType(info),
		Size:       uint64(info.Size()),
		CreateTime: info.ModTime(),
		Children:   nil,
	}

	if info.IsDir() {
		// 获取目录下的直接子目录
		files, err := os.ReadDir(path)
		if err != nil {
			return nil, err
		}

		for _, file := range files {
			childPath := filepath.Join(path, file.Name())
			childDiskFs, err := getDiskFs(childPath)
			if err != nil {
				return nil, err
			}
			diskFs.Children = append(diskFs.Children, childDiskFs...)
		}
		diskFs.Size, err = DirSize(path)
	}

	return []*DiskFs{diskFs}, nil
}

func ListDiskFs(path string) ([]*DiskFs, error) {
	cfgDir := global.Env().GetDataDir()
	filePath := cfgDir
	if len(strings.TrimSpace(path)) > 0 {
		filePath = filepath.Join(cfgDir, path)
	}
	// 检测操作路径是否在 Data 目录下
	if !util.IsFileWithinFolder(filePath, cfgDir) {
		return []*DiskFs{}, errors.Errorf("invalid operator file: %s, outside of path: %v", path, cfgDir)
	}
	return getDiskFs(filePath)
}

func DeleteDataFile(path string) error {
	// 1. 检查路径是否为空
	if len(strings.TrimSpace(path)) == 0 {
		return errors.New("File path cannot be empty.")
	}

	// 2. 获取数据目录的绝对路径
	cfgDir := global.Env().GetDataDir()
	absParentDirectory, err := filepath.Abs(cfgDir)
	if err != nil {
		return err
	}

	// 3. 构建目标文件的绝对路径
	absTargetPath := filepath.Join(absParentDirectory, filepath.Clean(path))

	// 3. 检查是否在 queue 目录下
	if !strings.HasPrefix(absTargetPath, "queue") {
		return errors.New("File must be in the queue directory.")
	}

	// 4. 检查文件扩展名是否为 .dat
	if filepath.Ext(absTargetPath) != ".dat" {
		return errors.New("File must have dat extension.")
	}

	// 5. 获取文件信息，检查是否为文件
	fileInfo, err := os.Stat(absTargetPath)
	if err != nil {
		return err
	}
	if fileInfo.IsDir() {
		return errors.New("Cannot delete a directory.")
	}

	// 6. 检测操作路径是否在 Data 目录下
	if !util.IsFileWithinFolder(absTargetPath, cfgDir) {
		return errors.Errorf("invalid operator file: %s, outside of path: %v", path, cfgDir)
	}

	// 7. 删除文件
	err = os.Remove(absTargetPath)
	if err != nil {
		return err
	}

	return nil
}
