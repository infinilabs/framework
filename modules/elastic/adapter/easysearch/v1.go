/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package easysearch

import (
	"fmt"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/elastic/adapter/elasticsearch"
	"net/http"
)

type APIV1 struct {
	elasticsearch.ESAPIV7_7
}

func (c *APIV1) StartReplication(followIndex string, body []byte) error{
	url := fmt.Sprintf("%s/_replication/%s/_start", c.GetEndpoint(), followIndex)
	resp, err := c.Request(nil, util.Verb_PUT, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) StopReplication(indexName string, body []byte) error{
	url := fmt.Sprintf("%s/_replication/%s/_stop", c.GetEndpoint(), indexName)
	resp, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) PauseReplication(followIndex string, body []byte) error {
	url := fmt.Sprintf("%s/_replication/%s/_pause", c.GetEndpoint(), followIndex)
	//body must not be empty
	if len(body) == 0 {
		body = []byte(`{}`)
	}
	resp, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) ResumeReplication(followIndex string, body []byte) error {
	url := fmt.Sprintf("%s/_replication/%s/_resume", c.GetEndpoint(), followIndex)
	//body must not be empty
	if len(body) == 0 {
		body = []byte(`{}`)
	}
	resp, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) GetReplicationStatus(followIndex string) ([]byte, error){
	url := fmt.Sprintf("%s/_replication/%s/_status", c.GetEndpoint(), followIndex)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) GetReplicationFollowerStats(followIndex string) ([]byte, error){
	url := fmt.Sprintf("%s/_replication/follower_stats", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) CreateAutoFollowReplication(autoFollowPatternName string, body []byte) error {
	url := fmt.Sprintf("%s/_replication/_autofollow", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_POST, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) GetAutoFollowStats(autoFollowPatternName string)([]byte, error){
	url := fmt.Sprintf("%s/_replication/autofollow_stats", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}