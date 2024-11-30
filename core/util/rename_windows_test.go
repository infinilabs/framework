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

// +build windows

/*
[nsq]: https://github.com/nsqio/nsq
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
 [nsq]: https://github.com/nsqio/nsq
*/

package util

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nsqio/nsq/internal/util"
)

const TEST_FILE_COUNT = 500

func TestConcurrentRenames(t *testing.T) {
	var waitGroup util.WaitGroupWrapper

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	trigger := make(chan struct{})
	testDir := filepath.Join(os.TempDir(), fmt.Sprintf("nsqd_TestConcurrentRenames_%d", r.Int()))

	err := os.MkdirAll(testDir, 644)
	if err != nil {
		t.Error(err)
	}

	fis, err := ioutil.ReadDir(testDir)
	if err != nil {
		t.Error(err)
	} else if len(fis) > 0 {
		t.Errorf("Test directory %s unexpectedly has %d items in it!", testDir, len(fis))
		t.FailNow()
	}

	// create a bunch of source files and attempt to concurrently rename them all
	for i := 1; i <= TEST_FILE_COUNT; i++ {
		//First rename doesn't overwrite/replace; no target present
		sourcePath1 := filepath.Join(testDir, fmt.Sprintf("source1_%d.txt", i))
		//Second rename will replace
		sourcePath2 := filepath.Join(testDir, fmt.Sprintf("source2_%d.txt", i))
		targetPath := filepath.Join(testDir, fmt.Sprintf("target_%d.txt", i))
		err = ioutil.WriteFile(sourcePath1, []byte(sourcePath1), 0644)
		if err != nil {
			t.Error(err)
		}
		err = ioutil.WriteFile(sourcePath2, []byte(sourcePath2), 0644)
		if err != nil {
			t.Error(err)
		}

		waitGroup.Wrap(func() {
			_, _ = <-trigger
			err := AtomicFileRename(sourcePath1, targetPath)
			if err != nil {
				t.Error(err)
			}
			err = AtomicFileRename(sourcePath2, targetPath)
			if err != nil {
				t.Error(err)
			}
		})
	}

	// start.. they're off to the races!
	close(trigger)

	// wait for completion...
	waitGroup.Wait()

	// no source files should exist any longer; we should just have 500 target files
	fis, err = ioutil.ReadDir(testDir)
	if err != nil {
		t.Error(err)
	} else if len(fis) != TEST_FILE_COUNT {
		t.Errorf("Test directory %s unexpectedly has %d items in it!", testDir, len(fis))
	} else {
		for _, fi := range fis {
			if !strings.HasPrefix(fi.Name(), "target_") {
				t.Errorf("Test directory file %s is not expected target file!", fi.Name())
			}
		}
	}

	// clean up the test directory
	err = os.RemoveAll(testDir)
	if err != nil {
		t.Error(err)
	}
}
