package index

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/index"
	"github.com/infinitbyte/framework/core/queue"
	"runtime"
)

type IndexModule struct {
}

//TODO naming
const IndexChannel string = "index"

func (this IndexModule) Name() string {
	return "Index"
}

var signalChannel chan bool

type IndexConfig struct {
	Elasticsearch *index.ElasticsearchConfig `config:"elasticsearch"`
}

var (
	defaultConfig = IndexConfig{
		Elasticsearch: &index.ElasticsearchConfig{
			Endpoint:    "http://localhost:9200",
			IndexPrefix: "app-",
		},
	}
)

func (module IndexModule) Start(cfg *Config) {

	indexConfig := defaultConfig
	cfg.Unpack(&indexConfig)

	signalChannel = make(chan bool, 1)
	client := index.ElasticsearchClient{Config: defaultConfig.Elasticsearch}

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
				er, v := queue.Pop(IndexChannel)
				log.Trace("got index signal, ", string(v))
				if er != nil {
					log.Error(er)
					continue
				}
				//indexing to es or blevesearch
				doc := index.IndexDocument{}
				err := json.Unmarshal(v, &doc)
				if err != nil {
					panic(err)
				}

				client.Index(doc.Index, doc.ID, doc.Source)
			}

		}
	}()
}

func (module IndexModule) Stop() error {
	signalChannel <- true
	return nil
}
