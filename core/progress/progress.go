package progress

import (
	"fmt"
	"gopkg.in/cheggaaa/pb.v1"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/stats"
	"sync"
)

var statsLock sync.RWMutex
var barsMap map[string]*pb.ProgressBar = map[string]*pb.ProgressBar{}
var statsMap map[string]int = map[string]int{}

func RegisterBar(category, key string, total int) {
	if ShowProgress() {
		statsKey := fmt.Sprintf("[%v][%v]:", category, key)
		statsLock.Lock()
		defer statsLock.Unlock()
		statsMap[statsKey]=0
		bar:=pb.New(total).Prefix(statsKey)
		barsMap[statsKey] = bar
	}
}

func IncreaseWithTotal(category, key string, count, total int) {

	if total<=0{
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
		bar,ok:=barsMap[statsKey]
		if !ok{
			bar=pb.New(total).Prefix(fmt.Sprintf("[%v][%v]:", category, key))
			barsMap[statsKey] = bar
		}
		if bar.Total!=int64(total){
			bar.SetTotal(total)
		}

		bar.Set(sumCount)
		bar.Update()
	}
	stats.Gauge(category,key,int64(sumCount*100/(total)))
}

var pool *pb.Pool
var err error
var started bool

func Start() {


	if ShowProgress(){

		statsLock.Lock()
		defer statsLock.Unlock()

		if !started{
			ar:=[]*pb.ProgressBar{}
			for _,v:=range barsMap {
				ar=append(ar,v)
			}
			pool, err = pb.StartPool(ar...)
			if err != nil {
				panic(err)
			}
			started=true
		}else{
			for k,_:=range statsMap{
				_,ok:=barsMap[k]
				if !ok{
					var bar *pb.ProgressBar = pb.New(100).Prefix(k)
					barsMap[k] = bar
					pool.Add(bar)
				}
			}
		}

	}
}

func ShowProgress() bool {

	cfg:= struct {
		Enabled     bool     `config:"enabled" json:"enabled,omitempty"`
	}{}

	exists,_:=env.ParseConfig("progress_bar",&cfg)
	if exists{
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

		for k,v:=range statsMap {
			x:=barsMap[k]
			if int(x.Total)==v{
				if !x.IsFinished(){
					x.Finish()
				}
			}else{
				continue
			}
		}
		if pool!=nil{
			pool.Stop()
		}
		barsMap = map[string]*pb.ProgressBar{}
		statsMap = map[string]int{}
		started=false
	}
}
