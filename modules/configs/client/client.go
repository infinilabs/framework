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
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/api"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/keystore"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	ucfg "infini.sh/framework/lib/go-ucfg"
	"infini.sh/framework/modules/configs/common"
	"infini.sh/framework/modules/configs/config"
)

const bucketName = "instance_registered"
const configRegisterEnvKey = "CONFIG_MANAGED_SUCCESS"
const legacyManagedRegisterCompatMaxVersion = "1.30.4"
const unauthorizedRegisterRetryInterval = 10 * time.Second

var postRegisterHooks []func(server string, res *util.Result) error
var unauthorizedRegisterRetryLock sync.Mutex
var lastUnauthorizedRegisterRetryAt time.Time
var clearManagedRegistrationStateFunc = clearManagedRegistrationState
var loadManagedBootstrapAccessTokenFunc = func() (string, error) {
	return common.LoadTokenFromKeystore(common.ManagerBootstrapTokenKeystoreKey)
}
var restoreManagedBootstrapAccessTokenFunc = func() (string, error) {
	token, err := loadManagedBootstrapAccessTokenFunc()
	if err != nil {
		return "", err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		token = strings.TrimSpace(global.Env().SystemConfig.Configs.ManagerConfig.AccessToken.Get())
	}
	if token == "" {
		token, err = common.LoadTokenFromKeystore(common.ManagerTokenKeystoreKey)
		if err != nil {
			return "", err
		}
		token = strings.TrimSpace(token)
	}
	if token == "" {
		return "", fmt.Errorf("managed bootstrap access token is missing")
	}
	global.Env().SystemConfig.Configs.ManagerConfig.AccessToken = ucfg.SecretString(token)
	return token, nil
}
var reconnectToManagerFunc func() error
var configSyncInProgress atomic.Bool

func init() {
	reconnectToManagerFunc = ConnectToManager
}

// maskURLInError replaces http(s):// URLs in error messages to avoid leaking internal addresses in logs.
func maskURLInError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	for _, scheme := range []string{"https://", "http://"} {
		for {
			idx := strings.Index(msg, scheme)
			if idx < 0 {
				break
			}
			end := strings.IndexAny(msg[idx:], " \"'\n\t")
			if end < 0 {
				msg = msg[:idx] + "***"
				break
			}
			msg = msg[:idx] + "***" + msg[idx+end:]
		}
	}
	return msg
}

func truncateManagerResponseBodyForLog(body []byte) string {
	text := strings.TrimSpace(string(body))
	if len(text) <= 256 {
		return text
	}
	return text[:256] + "...(truncated)"
}

func tryStartManagedConfigSync() bool {
	return configSyncInProgress.CompareAndSwap(false, true)
}

func finishManagedConfigSync() {
	configSyncInProgress.Store(false)
}

func ConnectToManager() error {
	cfg := global.Env().SystemConfig.Configs
	if !cfg.Managed {
		return nil
	}
	if cfg.Servers == nil || len(cfg.Servers) == 0 {
		return errors.Errorf("no config manager was found")
	}

	// k8s env setting always_register_after_restart and  pod after restart the ip will change so need register again
	if !cfg.AlwaysRegisterAfterRestart {
		if exists, err := kv.ExistsKey(bucketName, []byte(global.Env().SystemConfig.NodeConfig.ID)); exists && err == nil {
			//already registered skip further process
			log.Infof("skip config manager registration for instance %v: local registration marker exists", global.Env().SystemConfig.NodeConfig.ID)
			global.Register(configRegisterEnvKey, true)
			return nil
		}
	}

	info := model.GetInstanceInfo()
	log.Infof("start config manager registration for instance %v against %d server(s)", info.ID, len(cfg.Servers))
	registerReq := common.InstanceRegisterRequest{
		Client: info,
	}
	registerAccessToken, err := buildManagedRegisterAccessToken(info)
	if err != nil {
		return err
	}
	if registerAccessToken != nil {
		registerReq.AccessToken = registerAccessToken
	}

	req := util.Request{Method: util.Verb_POST}
	req.ContentType = "application/json"
	req.Path = common.REGISTER_API
	req.Body = util.MustToJSONBytes(registerReq)

	server, res, err := submitRequestToManager(&req)
	if err == nil && server != "" {
		if res.StatusCode == 200 || util.ContainStr(string(res.Body), "exists") {
			if err := execPostRegisterHooks(server, res); err != nil {
				return err
			}
			log.Infof("config manager registration succeeded for instance %v via %v: status=%d", info.ID, server, res.StatusCode)
			err := kv.AddValue(bucketName, []byte(global.Env().SystemConfig.NodeConfig.ID), []byte(util.GetLowPrecisionCurrentTime().String()))
			if err != nil {
				panic(err)
			}
			global.Register(configRegisterEnvKey, true)
		} else {
			if res.StatusCode == http.StatusUnauthorized {
				if !claimUnauthorizedRegisterRetrySlot() {
					return fmt.Errorf("unauthorized config manager registration")
				}
				return recoverManagedRegistrationWithBootstrap()
			}
			log.Warnf("config manager registration failed for instance %v via %v: status=%d, body=%s", info.ID, server, res.StatusCode, truncateManagerResponseBodyForLog(res.Body))
			return fmt.Errorf("failed to register to config manager: status=%d, body=%s", res.StatusCode, strings.TrimSpace(string(res.Body)))
		}
	} else {
		log.Errorf("config manager registration request failed for instance %v via %v: %v", info.ID, server, err)
	}
	return err
}

func buildManagedRegisterAccessToken(info model.Instance) (*common.RegisterToken, error) {
	if !common.SupportsManagedAccessToken(info.Application.Name) {
		return nil, nil
	}
	if shouldSkipManagedRegisterAccessToken(info.Application.Version.VersionNumber) {
		return nil, nil
	}
	accessToken, err := common.EnsureTokenInKeystore(common.AgentAccessTokenKeystoreKey)
	if err != nil {
		return nil, err
	}
	productName := strings.TrimSpace(info.Application.Name)
	if productName == "" {
		productName = "instance"
	}
	return &common.RegisterToken{
		Name:        fmt.Sprintf("%s access token", info.ID),
		Description: fmt.Sprintf("Console to %s access token for instance %s", productName, info.ID),
		Value:       accessToken,
	}, nil
}

func shouldSkipManagedRegisterAccessToken(version string) bool {
	version = strings.TrimSpace(version)
	if version == "" {
		return false
	}
	parsed, err := util.ParseSemantic(version)
	if err != nil {
		parsed, err = util.ParseGeneric(version)
		if err != nil {
			return false
		}
	}
	cmp, err := parsed.Compare(legacyManagedRegisterCompatMaxVersion)
	if err != nil {
		return false
	}
	return cmp <= 0
}

func AddPostRegisterHook(hook func(server string, res *util.Result) error) {
	if hook != nil {
		postRegisterHooks = append(postRegisterHooks, hook)
	}
}

func clearManagedRegistrationState() error {
	global.Register(configRegisterEnvKey, false)
	instanceID := strings.TrimSpace(global.Env().SystemConfig.NodeConfig.ID)
	if instanceID == "" {
		return nil
	}
	return kv.DeleteKey(bucketName, []byte(instanceID))
}

func handleUnauthorizedConfigSyncResponse(res *util.Result) bool {
	if res == nil || res.StatusCode != http.StatusUnauthorized {
		return false
	}

	if !claimUnauthorizedRegisterRetrySlot() {
		return true
	}

	log.Warn("config sync unauthorized, clearing local registration state and retrying registration")
	if err := recoverManagedRegistrationWithBootstrap(); err != nil {
		log.Warnf("failed to re-register to config manager after unauthorized config sync: %v", err)
		return true
	}
	log.Info("re-registered to config manager after unauthorized config sync")
	return true
}

func claimUnauthorizedRegisterRetrySlot() bool {
	unauthorizedRegisterRetryLock.Lock()
	defer unauthorizedRegisterRetryLock.Unlock()
	if !lastUnauthorizedRegisterRetryAt.IsZero() && time.Since(lastUnauthorizedRegisterRetryAt) < unauthorizedRegisterRetryInterval {
		return false
	}
	lastUnauthorizedRegisterRetryAt = time.Now()
	return true
}

func recoverManagedRegistrationWithBootstrap() error {
	if _, err := restoreManagedBootstrapAccessTokenFunc(); err != nil {
		return err
	}
	if err := clearManagedRegistrationStateFunc(); err != nil {
		return err
	}
	return reconnectToManagerFunc()
}

func execPostRegisterHooks(server string, res *util.Result) error {
	for _, hook := range postRegisterHooks {
		if err := hook(server, res); err != nil {
			return err
		}
	}
	return nil
}

func submitRequestToManager(req *util.Request) (string, *util.Result, error) {
	return DoManagerRequest(req)
}

func DoManagerRequest(req *util.Request) (string, *util.Result, error) {
	var err error
	var res *util.Result
	cfg := global.Env().SystemConfig.Configs
	if err = applyManagerRequestAuth(req); err != nil {
		return "", nil, err
	}
	for _, server := range cfg.Servers {
		req.Url, err = url.JoinPath(server, req.Path)
		if err != nil {
			continue
		}
		res, err = util.ExecuteRequestWithCatchFlag(getManagerHTTPClient(), req, true)
		if err != nil {
			continue
		}
		return server, res, nil
	}
	return "", nil, err
}

func applyManagerRequestAuth(req *util.Request) error {
	cfg := global.Env().SystemConfig.Configs
	if token := cfg.ManagerConfig.AccessToken.Get(); token != "" {
		req.AddHeader(model.API_TOKEN, token)
		return nil
	}
	token, err := common.LoadTokenFromKeystore(common.ManagerTokenKeystoreKey)
	if err != nil {
		return err
	}
	if token != "" {
		req.AddHeader("Authorization", "Bearer "+token)
		return nil
	}
	if cfg.ManagerConfig.BasicAuth.Username != "" {
		req.SetBasicAuth(cfg.ManagerConfig.BasicAuth.Username, cfg.ManagerConfig.BasicAuth.Password.Get())
	}
	return nil
}

var managerHTTPClientInitLock = sync.Once{}
var configSyncInitLock = sync.Once{}
var mTLSClient *http.Client

func initManagerHTTPClient() {
	managerHTTPClientInitLock.Do(func() {
		if !global.Env().SystemConfig.Configs.Managed {
			return
		}
		cfg := global.Env().GetHTTPClientConfig("configs", "")
		if cfg != nil {
			hClient, err := api.NewHTTPClient(cfg)
			if err != nil {
				panic(err)
			}
			mTLSClient = hClient
		}
	})
}

func getManagerHTTPClient() *http.Client {
	initManagerHTTPClient()
	return mTLSClient
}

func ListenConfigChanges() error {
	configSyncInitLock.Do(func() {

		if global.Env().SystemConfig.Configs.Managed {
			initManagerHTTPClient()

			var syncFunc = func() {
				if !tryStartManagedConfigSync() {
					if global.Env().IsDebug {
						log.Trace("skip overlapping config sync")
					}
					return
				}
				defer finishManagedConfigSync()

				if global.Env().IsDebug {
					log.Trace("fetch configs from manger")
				}

				req := common.ConfigSyncRequest{}
				req.Client = model.GetInstanceInfo()
				cfgs := config.GetConfigs(false, false)
				req.Configs = cfgs
				req.Hash = util.MD5digestString(util.MustToJSONBytes(cfgs))

				//fetch configs from manager
				request := util.Request{Method: util.Verb_POST}
				request.ContentType = "application/json"
				request.Path = common.SYNC_API
				requestBody := util.MustToJSONBytes(req)
				request.Body = requestBody

				if global.Env().IsDebug {
					log.Debug("config sync request: ", string(requestBody))
				}

				_, res, err := DoManagerRequest(&request)
				if err != nil {
					log.Error("failed to submit request to config manager,", maskURLInError(err))
					return
				}

				if res != nil {
					if handleUnauthorizedConfigSyncResponse(res) {
						return
					}

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
									log.Errorf("error on delete config [%s]: %v", v.Name, err)
									panic(err)
								}
							} else {
								log.Debugf("config [%s] is not managed by config manager, skip deleting", v.Name)
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
