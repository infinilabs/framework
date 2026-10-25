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

package common

import (
	"strings"

	"infini.sh/framework/core/model"
)

const REGISTER_API = "/instance/_register"
const SYNC_API = "/configs/_sync"

const (
	ManagerTokenKeystoreKey          = "configs_manager_token"
	ManagerBootstrapTokenKeystoreKey = "configs_manager_bootstrap_token"
	InstanceAccessTokenKeystoreKey   = "instance_access_token"
)

type RegisterToken struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value,omitempty"`
}

type InstanceRegisterRequest struct {
	Client      model.Instance `json:"client"`
	AccessToken *RegisterToken `json:"access_token,omitempty"`
}

type ConfigFile struct {
	Name     string `json:"name,omitempty"`
	Location string `json:"location,omitempty"`
	Content  string `json:"content,omitempty"`
	Updated  int64  `json:"updated,omitempty"`
	Version  int64  `json:"version,omitempty"`
	Size     int64  `json:"size,omitempty"`
	Readonly bool   `json:"readonly,omitempty"`
	Managed  bool   `json:"managed"`
	Hash     string `json:"hash,omitempty"`
}

type ConfigList struct {
	Main    ConfigFile            `json:"main,omitempty"`
	Configs map[string]ConfigFile `json:"configs,omitempty"`
}

type ConfigDeleteRequest struct {
	Configs []string `json:"configs"`
}

type ConfigUpdateRequest struct {
	Configs map[string]string `json:"configs"`
}

type ConfigSyncRequest struct {
	ForceSync bool           `json:"force_sync"` //ignore hash check in server
	Hash      string         `json:"hash"`
	Client    model.Instance `json:"client"`
	Configs   ConfigList     `json:"configs"`

	// Runtime reports the instance's live pipeline state (which tasks are
	// running, what config version they loaded). The deployment reconciler
	// reads this to compute DeploymentStatus (desired vs reported).
	Runtime *RuntimeReport `json:"runtime,omitempty"`
}

// RuntimeReport is the pipeline execution state an instance reports on
// every sync — the "reported" half of the reconcile loop.
type RuntimeReport struct {
	// Pipelines maps task name → state + the managed-config version the
	// instance last loaded (0 = unknown/not from managed config).
	Pipelines map[string]PipelineRuntimeState `json:"pipelines,omitempty"`
	// Cordoned mirrors the instance-local maintenance flag (#121/#128):
	// running tasks drain but nothing starts. Reported so the control
	// plane can see WHO is cordoned without a proxy round-trip — the
	// placement side keeps configs flowing (a cordoned instance still
	// syncs; recall-on-cordon would strand its running tasks).
	Cordoned bool `json:"cordoned,omitempty"`
	// StageStats carries the instance's cumulative per-stage counters on
	// the same heartbeat — the same rows the local metrics exporter
	// persists. Instances without a shared datastore (gateway-type, local
	// sqlite) still appear in the manager's cluster aggregation.
	StageStats []StageStat `json:"stage_stats,omitempty"`
}

// StageStat is one per-stage counter row in the sync report — the wire
// shape of the pipeline module's StageStats (cumulative counters since
// process start; the manager persists them as pipeline-stage-stats
// snapshots and the cluster fold takes each instance's latest tick).
type StageStat struct {
	Pipeline   string `json:"pipeline"`
	InstanceID string `json:"instance_id"`
	Stage      string `json:"stage"`   // processor name or lane name
	Carrier    string `json:"carrier"` // "processor" | "lane:<name>"
	Count      int64  `json:"count"`
	Errors     int64  `json:"errors"`
	AvgLatency int64  `json:"avg_latency_ns"`
}

type PipelineRuntimeState struct {
	State         string `json:"state"`                    // STARTED|STOPPED|FAILED|...
	ConfigSource  string `json:"config_source,omitempty"`  // managed config name
	ConfigVersion int64  `json:"config_version,omitempty"` // managed config version loaded
	// ConfigDigest is the running task's content digest (the six fields a
	// deployment renders). The control plane compares it against the digest
	// it pushed to verify rollout readiness — state alone cannot prove the
	// instance runs the NEW content (#125).
	ConfigDigest string `json:"config_digest,omitempty"`
	// Reason explains a FAILED state (e.g. the config references a
	// processor missing from this instance's catalog) — surfaced in the
	// deployment status so operators see why without instance shell access
	Reason string `json:"reason,omitempty"`
}

type ConfigSyncResponse struct {
	Changed bool `json:"changed"`
	Configs struct {
		CreatedConfigs map[string]ConfigFile `json:"created,omitempty"`
		DeletedConfigs map[string]ConfigFile `json:"deleted,omitempty"`
		UpdatedConfigs map[string]ConfigFile `json:"updated,omitempty"`
	} `json:"configs,omitempty"`

	Secrets *Secrets `json:"secrets,omitempty"`
}

type ResourceGroup struct {
	Name string   `json:"name"`
	List []string `json:"list"`
}

type ConfigGroup struct {
	Files []string `config:"files"`
}

type InstanceGroup struct {
	ConfigGroups []string `config:"configs"`
	Instances    []string `config:"instances"`
	Secrets      []string `config:"secrets"`
}

type Secrets struct {
	Keystore map[string]KeystoreValue `json:"keystore,omitempty"`
}

type KeystoreValue struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type ConfigRepo struct {
	ConfigGroups   map[string]ConfigGroup   `config:"configs"`
	InstanceGroups map[string]InstanceGroup `config:"instances"`
	SecretGroups   map[string]Secrets       `config:"secrets"`
}

type InstanceSettings struct {
	ConfigFiles []string `config:"configs"`
	Secrets     []string `config:"secrets"`
}

// SupportsManagedAccessToken reports whether an application joining the
// managed-config flow should hand a keystore-backed access token to the
// console at registration time. Any Framework-based instance (Agent,
// Gateway, or third-party apps) qualifies; empty names are skipped since
// there is nothing meaningful to mint a token for.
func SupportsManagedAccessToken(applicationName string) bool {
	return strings.TrimSpace(applicationName) != ""
}
