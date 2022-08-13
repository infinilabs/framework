/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"context"
	"fmt"
	"infini.sh/framework/core/util"
	"net/http"
)

type Client struct {
}

func (client *Client) EnableTaskToNode(ctx context.Context, agentBaseURL string, nodeUUID string) error {
	req := &util.Request{
		Method: http.MethodGet,
		Url: fmt.Sprintf("%s/task/%s/_enable", agentBaseURL, nodeUUID),
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
		Method: http.MethodGet,
		Url: fmt.Sprintf("%s/task/%s/_disable", agentBaseURL, nodeUUID),
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
		Method: http.MethodDelete,
		Url: fmt.Sprintf("%s/manage/%s", agentBaseURL, agentID),
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

func (client *Client) doRequest(req *util.Request, respObj interface{}) error{
	result, err := util.ExecuteRequest(req)
	if err != nil {
		return err
	}
	return util.FromJSONBytes(result.Body, respObj)
}
