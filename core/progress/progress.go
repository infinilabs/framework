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

package progress

import (
	"fmt"
	"sync"

	"github.com/rubyniu105/framework/core/env"
	"gopkg.in/cheggaaa/pb.v1"
)

var statsLock sync.RWMutex
var barsMap map[string]*pb.ProgressBar = map[string]*pb.ProgressBar{}
var statsMap map[string]int = map[string]int{}

func RegisterBar(category, key string, total int) {
	if ShowProgress() {
		statsKey := fmt.Sprintf("[%v][%v]:", category, key)
		statsLock.Lock()
		defer statsLock.Unlock()
		statsMap[statsKey] = 0
		bar := pb.New(total).Prefix(statsKey)
		barsMap[statsKey] = bar
	}
}

func IncreaseWithTotal(category, key string, count, total int) {

	if total <= 0 {
		return
	}

	statsKey := fmt.Sprintf("[%v][%v]:", category, key)
	statsLock.Lock()
	defer statsLock.Unlock()
	v, ok := statsMap[statsKey]
	var sumCount = count
	if ok {
		sumCount = count + v
	}

	statsMap[statsKey] = sumCount
	if ShowProgress() {
		bar, ok := barsMap[statsKey]
		if !ok {
			bar = pb.New(total).Prefix(fmt.Sprintf("[%v][%v]:", category, key))
			barsMap[statsKey] = bar
		}
		if bar.Total != int64(total) {
			bar.SetTotal(total)
		}

		bar.Set(sumCount)
		bar.Update()
	}
}

var pool *pb.Pool
var err error
var started bool

func Start() {

	if ShowProgress() {

		statsLock.Lock()
		defer statsLock.Unlock()

		if !started {
			ar := []*pb.ProgressBar{}
			for _, v := range barsMap {
				ar = append(ar, v)
			}
			pool, err = pb.StartPool(ar...)
			if err != nil {
				panic(err)
			}
			started = true
		} else {
			for k, _ := range statsMap {
				_, ok := barsMap[k]
				if !ok {
					var bar *pb.ProgressBar = pb.New(100).Prefix(k)
					barsMap[k] = bar
					pool.Add(bar)
				}
			}
		}

	}
}

func ShowProgress() bool {

	cfg := struct {
		Enabled bool `config:"enabled" json:"enabled,omitempty"`
	}{}

	exists, _ := env.ParseConfig("progress_bar", &cfg)
	if exists {
		return cfg.Enabled
	}

	return false
	//var showBar bool = cfg.Enabled
	//if isatty.IsTerminal(os.Stdout.Fd()) {
	//	showBar = true
	//} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
	//	showBar = true
	//} else {
	//	showBar = false
	//}
	//return showBar
}

func Stop() {
	if ShowProgress() {

		for k, v := range statsMap {
			x := barsMap[k]
			if int(x.Total) == v {
				if !x.IsFinished() {
					x.Finish()
				}
			} else {
				continue
			}
		}
		if pool != nil {
			pool.Stop()
		}
		barsMap = map[string]*pb.ProgressBar{}
		statsMap = map[string]int{}
		started = false
	}
}
