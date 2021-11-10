package progress

import (
	"fmt"
	"github.com/mattn/go-isatty"
	"gopkg.in/cheggaaa/pb.v1"
	"infini.sh/framework/core/stats"
	"os"
	"sync"
)

var statsLock sync.RWMutex
var barsMap map[string]*pb.ProgressBar = map[string]*pb.ProgressBar{}
var statsMap map[string]int = map[string]int{}

func IncreaseWithTotal(category, key string, count, total int) {
	statsKey := fmt.Sprintf("%v-%v", category, key)
	statsLock.Lock()
	defer statsLock.Unlock()
	v, ok := statsMap[statsKey]
	var sumCount = count
	if ok {
		sumCount = count + v
	} else {
		if ShowProgress() {
			var bar *pb.ProgressBar = pb.New(total).Prefix(fmt.Sprintf("[%v][%v]:", category, key))
			barsMap[statsKey] = bar
		}
	}

	statsMap[statsKey] = sumCount
	if ShowProgress() {
		barsMap[statsKey].Set(sumCount)
	}
	stats.Gauge(category,key,int64(sumCount*100/(total)))

}

var pool *pb.Pool
var err error
var started bool

func Start() {
	if ShowProgress()&&!started {
		ar:=[]*pb.ProgressBar{}
		for _,v:=range barsMap {
			ar=append(ar,v)
		}
		pool, err = pb.StartPool(ar...)
		if err != nil {
			panic(err)
		}
		started=true
	}
}

func ShowProgress() bool {

	var showBar bool = false
	if isatty.IsTerminal(os.Stdout.Fd()) {
		showBar = true
	} else if isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		showBar = true
	} else {
		showBar = false
	}
	return showBar
}

func Stop() {
	if ShowProgress() {

		for k,v:=range statsMap {
			x:=barsMap[k]
			if int(x.Total)==v{
				if !x.IsFinished(){
					x.Finish()
				}
			}else{
				return
			}
		}
		pool.Stop()
		barsMap = map[string]*pb.ProgressBar{}
		statsMap = map[string]int{}
		started=false
	}
}