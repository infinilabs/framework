/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"fmt"
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/util"
	"net/http"
)

type Client struct {
}

func (client *Client) EnableTaskToNode(ctx context.Context, agentBaseURL string, nodeUUID string) error {
	req := &util.Request{
		Method:  http.MethodGet,
		Url:     fmt.Sprintf("%s/task/%s/_enable", agentBaseURL, nodeUUID),
		Context: ctx,
	}
	resBody := map[string]interface{}{}
	err := client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["result"] != "success" {
		return fmt.Errorf("%v", resBody["error"])
	}
	return nil
}

func (client *Client) DisableTaskToNode(ctx context.Context, agentBaseURL string, nodeUUID string) error {
	req := &util.Request{
		Method:  http.MethodGet,
		Url:     fmt.Sprintf("%s/task/%s/_disable", agentBaseURL, nodeUUID),
		Context: ctx,
	}
	resBody := map[string]interface{}{}
	err := client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["result"] != "success" {
		return fmt.Errorf("%v", resBody["error"])
	}
	return nil
}

func (client *Client) DeleteInstance(ctx context.Context, agentBaseURL string, agentID string) error {
	req := &util.Request{
		Method:  http.MethodDelete,
		Url:     fmt.Sprintf("%s/manage/%s", agentBaseURL, agentID),
		Context: ctx,
	}
	resBody := map[string]interface{}{}
	err := client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["result"] != "deleted" {
		return fmt.Errorf("%v", resBody["error"])
	}
	return nil
}

func (client *Client) EnrollInstance(ctx context.Context, agentBaseURL string, agentID string, body interface{}) error {
	req := &util.Request{
		Method:  http.MethodPost,
		Url:     fmt.Sprintf("%s/manage/register/%s", agentBaseURL, agentID),
		Context: ctx,
	}
	reqBody, err := util.ToJSONBytes(body)
	if err != nil {
		return err
	}
	req.Body = reqBody
	resBody := map[string]interface{}{}
	err = client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["result"] != "updated" {
		return fmt.Errorf("enroll error from client: %v", resBody["error"])
	}
	return nil
}

func (client *Client) GetHostInfo(ctx context.Context, agentBaseURL string, agentID string) (*host.HostInfo, error) {
	req := &util.Request{
		Method:  http.MethodGet,
		Url:     fmt.Sprintf("%s/agent/%s/host/_basic", agentBaseURL, agentID),
		Context: ctx,
	}
	resBody := struct {
		Success bool `json:"success"`
		Error string `json:"error"`
		HostInfo *host.HostInfo `json:"result"`
	}{}
	err := client.doRequest(req, &resBody)
	if err != nil {
		return nil, err
	}
	if resBody.Success != true {
		return nil, fmt.Errorf("enroll error from client: %v", resBody.Error)
	}
	return resBody.HostInfo, nil
}

func (client *Client) SetNodesMetricTask(ctx context.Context, agentBaseURL string, body interface{}) error {
	req := &util.Request{
		Method:  http.MethodPost,
		Url:     fmt.Sprintf("%s/task/_extra", agentBaseURL),
		Context: ctx,
	}
	reqBody, err := util.ToJSONBytes(body)
	if err != nil {
		return err
	}
	req.Body = reqBody
	resBody := map[string]interface{}{}
	err = client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["success"] != true {
		return fmt.Errorf("set nodes metric task error from client: %v", resBody["error"])
	}
	return nil
}
func (client *Client) DiscoveredHost(ctx context.Context, agentBaseURL string, body interface{}) error {
	req := &util.Request{
		Method:  http.MethodPut,
		Url:     fmt.Sprintf("%s/host/discover", agentBaseURL),
		Context: ctx,
	}
	reqBody, err := util.ToJSONBytes(body)
	if err != nil {
		return err
	}
	req.Body = reqBody
	resBody := map[string]interface{}{}
	err = client.doRequest(req, &resBody)
	if err != nil {
		return err
	}
	if resBody["success"] != true {
		return fmt.Errorf("discover host callback error: %v", resBody["error"])
	}
	return nil
}

func (client *Client) GetElasticProcess(ctx context.Context, agentBaseURL string, agentID string)(interface{}, error) {
	req := &util.Request{
		Method:  http.MethodGet,
		Url:     fmt.Sprintf("%s/agent/%s/process/_elastic", agentBaseURL, agentID),
		Context: ctx,
	}
	resBody := map[string]interface{}{}
	err := client.doRequest(req, &resBody)
	if err != nil {
		return nil, err
	}
	if resBody["success"] != true {
		return nil, fmt.Errorf("discover host callback error: %v", resBody["error"])
	}
	return resBody["elastic_process"], nil
}

func (client *Client) doRequest(req *util.Request, respObj interface{}) error {
	result, err := util.ExecuteRequest(req)
	if err != nil {
		return err
	}
	return util.FromJSONBytes(result.Body, respObj)
}
