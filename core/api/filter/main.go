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

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	Counter.Increase()

	if RequestLimit.IsAvailable() {
		RequestLimit.Increase()
		fmt.Println(RequestLimit.ReqCount)
		io.WriteString(w, "Hello world!\n")
	} else {
		fmt.Println("Reach request limiting!")
		io.WriteString(w, "Reach request limit!\n")
	}
}

func main() {
	fmt.Println("Server Started!")
	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/_stats", getStatsHandler)
	http.ListenAndServe(":8000", nil)
}

var qps []QPSCount

type QPSCount struct {
	Timestamp  int64
	QPS        int
	MaxHistory int
}

type CounterFilter struct {
	QPSCount
	CountAll int
	Lock     sync.Mutex
}

func NewCounterService() *CounterFilter {
	counter := &CounterFilter{}
	go func() {
		ticker := time.NewTicker(time.Second)
		for {
			<-ticker.C
			counter.Lock.Lock()
			counter.Timestamp = time.Now().Unix()

			if counter.QPS > 0 {
				qps = append(qps, QPSCount{counter.Timestamp, counter.QPS, 120})
			}

			counter.QPS = 0

			counter.Lock.Unlock()
		}
	}()
	return counter
}

func (counter *CounterFilter) Increase() {
	counter.Lock.Lock()
	defer counter.Lock.Unlock()

	counter.CountAll++
	counter.QPS++
}

func getStatsHandler(w http.ResponseWriter, r *http.Request) {
	cntStr := "time,qps\n"

	for _, c := range qps {
		cntStr += fmt.Sprintf("%d,%d\n", c.Timestamp, c.QPS)
	}

	cntStr += fmt.Sprintf("total: %d\n", Counter.CountAll)

	io.WriteString(w, cntStr)
}

var Counter = NewCounterService()
