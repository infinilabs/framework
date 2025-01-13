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
Copyright Medcl (m AT medcl.net)

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

package filter

//
//import (
//	"fmt"
//	"infini.sh/framework/core/api/router"
//	"io"
//	"net/http"
//	"sync"
//	"time"
//)
//
//type RateLimitFilter struct {
//	Interval time.Duration
//	MaxCount int
//	Lock     sync.Mutex
//	ReqCount int
//}
//
//func NewRequestLimitService(interval time.Duration, maxCnt int) *RateLimitFilter {
//	reqLimit := &RateLimitFilter{
//		Interval: interval,
//		MaxCount: maxCnt,
//	}
//
//	go func() {
//		ticker := time.NewTicker(interval)
//		defer ticker.Stop()
//		for {
//			<-ticker.C
//			reqLimit.Lock.Lock()
//			reqLimit.ReqCount = 0
//			reqLimit.Lock.Unlock()
//		}
//	}()
//
//	return reqLimit
//}
//
//func (filter *RateLimitFilter) Increase() {
//	filter.Lock.Lock()
//	defer filter.Lock.Unlock()
//
//	filter.ReqCount += 1
//}
//
//func (filter *RateLimitFilter) IsAvailable() bool {
//	filter.Lock.Lock()
//	defer filter.Lock.Unlock()
//
//	return filter.ReqCount < filter.MaxCount
//}
//
//var RequestLimit = NewRequestLimitService(10*time.Second, 5)
//
//func (filter RateLimitFilter) FilterHttpRouter(pattern string, h httprouter.Handle) httprouter.Handle {
//
//	Counter.Increase()
//
//	if RequestLimit.IsAvailable() {
//		RequestLimit.Increase()
//		return h
//
//	} else {
//		return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
//			fmt.Println("Reach request limiting!")
//			io.WriteString(w, "Reach request limit!\n")
//		}
//	}
//
//}
//
//func (filter RateLimitFilter) FilterHttpHandlerFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
//
//	Counter.Increase()
//
//	return func(w http.ResponseWriter, r *http.Request) {
//		if RequestLimit.IsAvailable() {
//			RequestLimit.Increase()
//			handler(w, r)
//		} else {
//			fmt.Println("Reach request limiting!")
//			io.WriteString(w, "Reach request limit!\n")
//		}
//	}
//
//}
