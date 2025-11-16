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

/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"bufio"
	"errors"
	"fmt"
	log "github.com/cihub/seelog"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// JoinPath return joined file path
func JoinPath(filenames ...string) string {

	hasSlash := false
	result := ""
	for _, str := range filenames {
		currentHasSlash := false
		if len(result) > 0 {
			currentHasSlash = strings.HasPrefix(str, "/")
			if hasSlash && currentHasSlash {
				str = strings.TrimPrefix(str, "/")
			}
			if !(hasSlash || currentHasSlash) {
				str = "/" + str
			}
		}
		hasSlash = strings.HasSuffix(str, "/")
		result += str
	}
	return result
}

func FilesExists(path ...string) bool {
	for _, v := range path {
		if !FileExists(v) {
			return false
		}
	}
	return true
}

// FileExists check if the path are exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

// CopyFile copy file from src to dst
func CopyFile(src, dst string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		log.Error(err.Error())
		return
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)

	if err != nil {
		log.Error(err.Error())
		return
	}

	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

// FileMTime get file modified time
func FileMTime(file string) (int64, error) {
	f, e := os.Stat(file)
	if e != nil {
		return 0, e
	}
	return f.ModTime().Unix(), nil
}

// FileSize get file size as how many bytes
func FileSize(file string) (int64, error) {
	f, e := os.Stat(file)
	if e != nil {
		return 0, e
	}
	return f.Size(), nil
}

// FileDelete delete file
func FileDelete(file string) error {
	return os.Remove(file)
}

// Rename handle file rename
func Rename(file string, to string) error {
	return os.Rename(file, to)
}

// FilePutContent put string to file
func FilePutContent(file string, content string) (int, error) {
	fs, e := os.Create(file)
	if e != nil {
		return 0, e
	}
	defer fs.Close()
	return fs.WriteString(content)
}

// FilePutContentWithByte put string to file
func FilePutContentWithByte(file string, content []byte) (int, error) {
	fs, e := os.Create(file)
	if e != nil {
		return 0, e
	}
	defer fs.Close()
	return fs.Write(content)
}

// FileAppendContentWithByte append bytes to the end of the file
func FileAppendContentWithByte(file string, content []byte) (int, error) {

	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	defer f.Close()
	return f.Write(content)
}

// FileAppendNewLine append new line to the end of the file
func FileAppendNewLine(file string, content string) (int, error) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	defer f.Close()
	return f.WriteString(content + "\n")
}

// FileAppendNewLineWithByte append bytes and break line(\n) to the end of the file
func FileAppendNewLineWithByte(file string, content []byte) (int, error) {
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}

	defer f.Close()
	return f.WriteString(string(content) + "\n")
}

// FileGetContent get string from text file
func FileGetContent(file string) ([]byte, error) {
	if !IsFile(file) {
		return nil, os.ErrNotExist
	}
	b, e := ioutil.ReadFile(file)
	if e != nil {
		return nil, e
	}
	return b, nil
}

func FileLinesWalk(filePath string, f func([]byte)) error {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	for scanner.Scan() {
		f(scanner.Bytes())
	}
	return file.Close()
}

func FileGetLines(filePath string) []string {
	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)

	var text []string
	for scanner.Scan() {
		text = append(text, scanner.Text())
	}

	file.Close()
	return text
}

// IsFile returns false when it's a directory or does not exist.
func IsFile(file string) bool {
	f, e := os.Stat(file)
	if e != nil {
		return false
	}
	return !f.IsDir()
}

// IsExist returns whether a file or directory exists.
func IsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func IsFileWithinFolder(file, path string) bool {

	aPath, err := filepath.Abs(path)
	if err != nil {
		panic(err)
	}
	aFile, err := filepath.Abs(file)
	if err != nil {
		panic(err)
	}

	if strings.HasPrefix(aFile, aPath) {
		return true
	} else {
		return false
	}
}

// CreateFile create file
func CreateFile(dir string, name string) (string, error) {
	src := dir + name + "/"
	if IsExist(src) {
		return src, nil
	}

	if err := os.MkdirAll(src, 0755); err != nil {
		if os.IsPermission(err) {
			fmt.Println("permission denied")
		}
		return "", err
	}

	return src, nil
}

// FileExtension extract file extension from file name
func FileExtension(file string) string {
	ext := filepath.Ext(file)
	return strings.ToLower(strings.TrimSpace(ext))
}

// Smart get file abs path
func TryGetFileAbsPath(filePath string, ignoreMissing bool) string {
	filename, _ := filepath.Abs(filePath)
	if FileExists(filename) {
		return filename
	}

	pwd, _ := os.Getwd()
	if pwd != "" {
		pwd = path.Join(pwd, filePath)
	}

	if FileExists(filename) {
		return filename
	}

	ex, err := os.Executable()
	var exPath string
	if err == nil {
		exPath = filepath.Dir(ex)
	}

	if exPath != "" {
		filename = path.Join(exPath, filePath)
	}

	if FileExists(filename) {
		return filename
	} else {
		if !ignoreMissing {
			panic(errors.New("file not found:" + filename))
		}
		return filePath
	}
}

func ListAllFiles(path string) ([]string, error) {
	output := []string{}
	err := filepath.Walk(path, func(file string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if file != path {
				files, err := ListAllFiles(file)
				if err != nil {
					panic(err)
				}
				for _, v := range files {
					output = append(output, v)
				}
			}
		} else {
			output = append(output, file)
		}
		return nil
	})
	return output, err
}

// path must be start and end with `/`
func NormalizeFolderPath(path string) string {
	if path == "" {
		return "/"
	}
	if !PrefixStr(path, "/") {
		path = "/" + path
	}
	if !SuffixStr(path, "/") {
		path = path + "/"
	}
	return path
}
