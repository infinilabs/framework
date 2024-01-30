/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package locker

import (
	"fmt"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
	log "github.com/cihub/seelog"
	"strings"
	"time"
)

const parentBucket = "dis_locker"

type AllocateInfo struct {
	ClientID  string
	Bucket    string
	Name      string
	Timestamp time.Time
}

func GetKey(bucket, name string) []byte {
	return []byte(bucket + ":" + name)
}

func placeLock(bucket, name string, clientID string) (bool, error) {
	v := fmt.Sprintf("%s/%v", clientID, util.GetLowPrecisionCurrentTime().Unix())
	err := kv.AddValue(parentBucket, GetKey(bucket, name), []byte(v))
	return true, err
}

func GetAllocateInfo(bucket, name string) (bool, *AllocateInfo, error) {
	v1, err := kv.GetValue(parentBucket, GetKey(bucket, name))
	if err!=nil{
		panic(err)
	}

	inf := &AllocateInfo{}
	if v1 == nil {
		//not found
		return false, nil, nil
	} else {
		arr := strings.Split(string(v1), "/")
		if len(arr) != 2 {
			return false, nil, errors.Errorf("invalid locker info: %v", string(v1))
		}
		unix, err := util.ToInt64(arr[1])
		if err != nil {
			return false, nil, err
		}
		inf.ClientID = arr[0]
		inf.Timestamp = util.FromUnixTimestamp(unix)
		inf.Bucket = bucket
		inf.Name = name
		return true,inf,nil
	}
}

func Hold(bucket, name string, clientID string, expireTimeout time.Duration, allocateIfNot bool) (bool, error) {
	ok, info, err := GetAllocateInfo(bucket, name)
	if err != nil {
		panic(err)
	}

	if ok {
		if expireTimeout.Seconds() <= 0 {
			expireTimeout = time.Duration(30) * time.Second
		}

		if info == nil {
			panic("allocate info can't be nil")
		}

		if info.ClientID != clientID {
			if time.Since(info.Timestamp) > expireTimeout {
				if allocateIfNot {
					if global.Env().IsDebug {
						log.Infof("lost someone, taking over: %v, client_id: %v, local_id:%v, duration: %v", string(GetKey(bucket, name)), info.ClientID, clientID, time.Since(info.Timestamp))
					}
					return placeLock(bucket, name, clientID)
				} else {
					return false, nil
				}
			} else {
				if global.Env().IsDebug {
					log.Infof("someone already taken this: %v, client_id: %v, local_id:%v, duration: %v", string(GetKey(bucket, name)), info.ClientID, clientID, time.Since(info.Timestamp))
				}
				return false, nil
			}
		}else{

			if global.Env().IsDebug{
				log.Debug("it's me, let's hold the lock again, bucket:",bucket,", name:", name,", client_id:",info.ClientID)
			}
			//update timestamp to extend the lease
			return placeLock(bucket, name, clientID)
		}
	}else{
		if global.Env().IsDebug {
			log.Debug("no one hold this lock, let's hold the lock, client_id:",bucket, name)
		}
		//not exists
		return placeLock(bucket, name, clientID)
	}
	return false, nil
}

func Release(bucket, name string,clientID string) error {

	ok, info, err := GetAllocateInfo(bucket, name)
	if err != nil {
		return err
	}

	if ok{
		if info.ClientID != clientID{
			//not your business
			return errors.Errorf("not your business anymore, client_id: %v, local_id:%v", info.ClientID, clientID)
		}
		err := kv.DeleteKey(parentBucket, GetKey(bucket, name))
		if err != nil {
			return err
		}
	}
	return nil
}
