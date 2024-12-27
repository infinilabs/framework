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

/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package client

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/configs/common"
	"infini.sh/framework/modules/configs/config"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const bucketName = "instance_registered"
const configRegisterEnvKey = "CONFIG_MANAGED_SUCCESS"

func ConnectToManager() error {

	if !global.Env().SystemConfig.Configs.Managed {
		return nil
	}

	// k8s env setting always_register_after_restart and  pod after restart the ip will change so need register again
	if !global.Env().SystemConfig.Configs.AlwaysRegisterAfterRestart {
		if exists, err := kv.ExistsKey(bucketName, []byte(global.Env().SystemConfig.NodeConfig.ID)); exists && err == nil {
			//already registered skip further process
			log.Info("already registered to config manager")
			global.Register(configRegisterEnvKey, true)
			return nil
		}
	}

	log.Info("register new instance to config manager")

	//register to config manager
	if global.Env().SystemConfig.Configs.Servers == nil || len(global.Env().SystemConfig.Configs.Servers) == 0 {
		return errors.Errorf("no config manager was found")
	}

	info := model.GetInstanceInfo()

	req := util.Request{Method: util.Verb_POST}
	req.ContentType = "application/json"
	req.Path = common.REGISTER_API
	req.Body = util.MustToJSONBytes(info)

	server, res, err := submitRequestToManager(&req)
	if err == nil && server != "" {
		if res.StatusCode == 200 || util.ContainStr(string(res.Body), "exists") {
			log.Infof("success register to config manager: %v", string(server))
			err := kv.AddValue(bucketName, []byte(global.Env().SystemConfig.NodeConfig.ID), []byte(util.GetLowPrecisionCurrentTime().String()))
			if err != nil {
				panic(err)
			}
			global.Register(configRegisterEnvKey, true)
		}
	} else {
		log.Error("failed to register to config manager,", err, ",", server)
	}
	return err
}

func submitRequestToManager(req *util.Request) (string, *util.Result, error) {
	var err error
	var res *util.Result
	for _, server := range global.Env().SystemConfig.Configs.Servers {
		req.Url, err = url.JoinPath(server, req.Path)
		if err != nil {
			continue
		}
		res, err = util.ExecuteRequestWithCatchFlag(mTLSClient, req, true)
		if err != nil {
			continue
		}
		return server, res, nil
	}
	return "", nil, err
}

var clientInitLock = sync.Once{}
var mTLSClient *http.Client

func ListenConfigChanges() error {

	clientInitLock.Do(func() {

		if global.Env().SystemConfig.Configs.Managed {
			cfg := global.Env().GetHTTPClientConfig("configs", "")
			if cfg != nil {
				hClient, err := api.NewHTTPClient(cfg)
				if err != nil {
					panic(err)
				}
				mTLSClient = hClient
			}

			//init config sync listening
			req := common.ConfigSyncRequest{}
			req.Client = model.GetInstanceInfo()

			var syncFunc = func() {
				if global.Env().IsDebug {
					log.Trace("fetch configs from manger")
				}

				cfgs := config.GetConfigs(false, false)
				req.Configs = cfgs
				req.Hash = util.MD5digestString(util.MustToJSONBytes(cfgs))

				//fetch configs from manager
				request := util.Request{Method: util.Verb_POST}
				request.ContentType = "application/json"
				request.Path = common.SYNC_API
				request.Body = util.MustToJSONBytes(req)

				if global.Env().IsDebug {
					log.Debug("config sync request: ", string(util.MustToJSONBytes(req)))
				}

				_, res, err := submitRequestToManager(&request)
				if err != nil {
					log.Error("failed to submit request to config manager,", err)
					return
				}

				if res != nil {
					obj := common.ConfigSyncResponse{}
					err := util.FromJSONBytes(res.Body, &obj)
					if err != nil {
						panic(err)
					}

					if global.Env().IsDebug {
						log.Debug("config sync response: ", string(res.Body))
					}

					if obj.Changed {

						//update secrets //TODO client send salt to manager first, manager encrypt secrets with salt and send back
						if obj.Secrets != nil {
							for k, v := range obj.Secrets.Keystore {
								if v.Type == "plaintext" {
									err := saveKeystore(k, v.Value)
									if err != nil {
										log.Error("error on save keystore:", k, ",", err)
									}
								}
							}

							//TODO maybe we have other secrets
						}

						for _, v := range obj.Configs.DeletedConfigs {
							if v.Managed {
								err := config.DeleteConfig(v.Name)
								if err != nil {
									panic(err)
								}
							} else {
								log.Error("config [", v.Name, "] is not managed by config manager, skip deleting")
							}
						}

						for _, v := range obj.Configs.CreatedConfigs {
							err := config.SaveConfig(v.Name, v)
							if err != nil {
								panic(err)
							}
						}

						for _, v := range obj.Configs.UpdatedConfigs {
							err := config.SaveConfig(v.Name, v)
							if err != nil {
								panic(err)
							}
						}

						var keyValuePairs = []util.KeyValue{}
						//checking backup files, remove old configs, only keep max num of files
						filepath.Walk(global.Env().SystemConfig.PathConfig.Config, func(file string, f os.FileInfo, err error) error {

							//only allow to delete backup files
							if !util.SuffixStr(file, ".bak") {
								return nil
							}

							keyValuePairs = append(keyValuePairs, util.KeyValue{Key: file, Value: f.ModTime().Unix()})
							return nil
						})
						if len(keyValuePairs) > 0 {
							keyValuePairs = util.SortKeyValueArray(keyValuePairs, true)

							if len(keyValuePairs) > global.Env().SystemConfig.Configs.MaxBackupFiles {
								tobeDeleted := keyValuePairs[global.Env().SystemConfig.Configs.MaxBackupFiles:]
								for _, v := range tobeDeleted {
									log.Debug("delete config file: ", v.Key)
									err := util.FileDelete(v.Key)
									if err != nil {
										panic(err)
									}
								}
							}
						}
					}
				}

			}

			syncFunc()

			if !global.Env().SystemConfig.Configs.ScheduledTask {
				log.Debug("register background task for checking configs changes")
				global.RegisterBackgroundCallback(&global.BackgroundTask{
					Tag:      "checking configs changes",
					Interval: util.GetDurationOrDefault(global.Env().SystemConfig.Configs.Interval, time.Duration(30)*time.Second),
					Func:     syncFunc,
				})
			} else {
				log.Debug("register schedule task for checking configs changes")
				task.RegisterScheduleTask(task.ScheduleTask{
					Description: fmt.Sprintf("sync configs from manager"),
					Type:        "interval",
					Interval:    global.Env().SystemConfig.Configs.Interval,
					Task: func(ctx context.Context) {
						syncFunc()
					},
				})
			}
		}

	})

	return nil
}

func saveKeystore(k string, v string) error {

	log.Debug("save keystore:", k)

	ks, err := keystore.GetWriteableKeystore()
	if err != nil {
		return err
	}
	err = ks.Store(k, util.UnsafeStringToBytes(v))
	if err != nil {
		return err
	}
	err = ks.Save()
	return err
}
