package elastic

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/elastic"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/queue"
	"runtime"
)

type ElasticIndexer struct {
	client       elastic.API
	indexChannel string
}

var signalChannel chan bool

func (this ElasticIndexer) Start() error {

	log.Trace("starting ElasticIndexer")

	signalChannel = make(chan bool, 1)

	go func() {
		defer func() {

			if !global.Env().IsDebug {
				if r := recover(); r != nil {

					if r == nil {
						return
					}
					var v string
					switch r.(type) {
					case error:
						v = r.(error).Error()
					case runtime.Error:
						v = r.(runtime.Error).Error()
					case string:
						v = r.(string)
					}
					log.Error("error in indexer,", v)
				}
			}
		}()

		for {
			select {
			case <-signalChannel:
				log.Trace("indexer exited")
				return
			default:
				log.Trace("waiting index signal")
				v, er := queue.Pop(this.indexChannel)
				log.Trace("got index signal, ", string(v))
				if er != nil {
					log.Error(er)
					continue
				}
				doc := elastic.IndexDocument{}
				err := json.Unmarshal(v, &doc)
				if err != nil {
					panic(err)
				}

				this.client.Index(doc.Index, doc.ID, doc.Source)
			}

		}
	}()

	log.Trace("started ElasticIndexer")

	return nil
}

func (this ElasticIndexer) Stop() error {
	log.Trace("stopping ElasticIndexer")
	signalChannel <- true
	log.Trace("stopped ElasticIndexer")
	return nil
}
