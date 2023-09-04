/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package elastic

import (
	"encoding/base64"
	"fmt"
	"github.com/bkaradzic/go-lz4"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/common"
	"net/http"
)

type ElasticStore struct {
	Client elastic.API
	Config common.StoreConfig
}

func (store *ElasticStore) Open() error {
	o:=&Blob{}
	err:=orm.RegisterSchemaWithIndexName(o,store.Config.IndexName)
	if err!=nil{
		panic(err)
	}
	if store.Config.IndexName==""{
		store.Config.IndexName=orm.GetIndexName(o)
	}
	log.Trace("store index name:",store.Config.IndexName)
	return nil
}

func (store *ElasticStore) Close() error {
	return nil
}

func (store *ElasticStore) GetCompressedValue(bucket string, key []byte) ([]byte, error) {

	data, err := store.GetValue(bucket, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, errors.New("No data found")
	}
	data, err = lz4.Decode(nil, data)
	if err != nil {
		log.Error("Failed to decode:",bucket,",",key,",", err)
		return nil, err
	}
	return data, nil
}

func (store *ElasticStore) ExistsKey(bucket string, key []byte) (bool,error) {
	response, err := store.Client.Get(store.Config.IndexName,"_doc", getKey(bucket, string(key)))
	if err != nil {
		return false, err
	}
	if response.Found {
		content := response.Source["content"]
		if content != nil {
			return true, nil
		}
	}
	return false, nil
}

func (store *ElasticStore) GetValue(bucket string, key []byte) ([]byte, error) {
	response, err := store.Client.Get(store.Config.IndexName,"_doc", getKey(bucket, string(key)))
	if err != nil {
		return nil, err
	}
	if response.Found {
		content := response.Source["content"]
		if content != nil {
			uDec, err := base64.URLEncoding.DecodeString(content.(string))

			if err != nil {
				return nil, err
			}
			return uDec, nil
		}
	}
	if response.StatusCode != http.StatusNotFound {
		var (
			errStr string
			ok bool
		)
		if errStr, ok = response.ESError.(string); !ok{
			errStr = util.MustToJSON(response.ESError)
		}
		return nil, fmt.Errorf("get value error: %s", errStr)
	}
	return nil,nil
}

func (store *ElasticStore) AddValueCompress(bucket string, key []byte, value []byte) error {
	value, err := lz4.Encode(nil, value)
	if err != nil {
		log.Error("Failed to encode:",bucket,",",key,",", err)
		return err
	}
	return store.AddValue(bucket, key, value)
}

func getKey(bucket, key string) string {
	return util.MD5digest(fmt.Sprintf("%s_%s", bucket, key))
}

func (store *ElasticStore) AddValue(bucket string, key []byte, value []byte) error {
	file := Blob{}
	file.Content = base64.URLEncoding.EncodeToString(value)
	_, err := store.Client.Index(store.Config.IndexName, "_doc", getKey(bucket, string(key)), file, "")
	return err
}

func (store *ElasticStore) DeleteKey(bucket string, key []byte) error {
	_, err := store.Client.Delete(store.Config.IndexName, "_doc", getKey(bucket, string(key)))
	return err
}

func (store *ElasticStore) DeleteBucket(bucket string) error {
	panic(errors.New("not implemented yet"))
}
