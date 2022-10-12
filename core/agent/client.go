/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"infini.sh/framework/core/host"
)

type ClientAPI interface {
	EnableTaskToNode(ctx context.Context, agentBaseURL string, nodeUUID string) error
	DisableTaskToNode(ctx context.Context, agentBaseURL string, nodeUUID string) error
	DeleteInstance(ctx context.Context, agentBaseURL string, agentID string) error
	EnrollInstance(ctx context.Context, agentBaseURL string, agentID string, body interface{}) error
	GetHostInfo(ctx context.Context, agentBaseURL string, agentID string) (*host.HostInfo, error)
	SetNodesMetricTask(ctx context.Context, agentBaseURL string, body interface{}) error
	DiscoveredHost(ctx context.Context, agentBaseURL string, body interface{}) error
	GetElasticProcess(ctx context.Context, agentBaseURL string, agentID string)(interface{}, error)
	GetElasticLogFiles(ctx context.Context, agentBaseURL string, agentID string, nodeID string)(interface{}, error)
	GetElasticLogFileContent(ctx context.Context, agentBaseURL string, agentID string, body interface{})(interface{}, error)
	GetInstanceBasicInfo(ctx context.Context, agentBaseURL string) (map[string]interface{}, error)
}
