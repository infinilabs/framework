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

func (c *APIV1) StartReplication(followIndex string, body []byte) error {
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
func (c *APIV1) StopReplication(indexName string, body []byte) error {
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
func (c *APIV1) GetReplicationStatus(followIndex string) ([]byte, error) {
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
func (c *APIV1) GetReplicationFollowerStats(followIndex string) ([]byte, error) {
	url := fmt.Sprintf("%s/_replication/all_status", c.GetEndpoint())
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

func (c *APIV1) DeleteAutoFollowReplication(autoFollowPatternName string, body []byte) error {
	url := fmt.Sprintf("%s/_replication/_autofollow", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_DELETE, url, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) GetAutoFollowStats(autoFollowPatternName string) ([]byte, error) {
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

func (c *APIV1) GetUser(username string) ([]byte, error) {
	url := fmt.Sprintf("%s/_security/user/%s", c.GetEndpoint(), username)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) GetUsers() ([]byte, error) {
	url := fmt.Sprintf("%s/_security/user", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) DeleteUser(username string) error {
	url := fmt.Sprintf("%s/_security/user/%s", c.GetEndpoint(), username)
	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) PutUser(username string, body []byte) error {
	url := fmt.Sprintf("%s/_security/user/%s", c.GetEndpoint(), username)
	resp, err := c.Request(nil, util.Verb_PUT, url, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) GetRole(roleName string) ([]byte, error) {
	url := fmt.Sprintf("%s/_security/role/%s", c.GetEndpoint(), roleName)
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) GetRoles() ([]byte, error) {
	url := fmt.Sprintf("%s/_security/role", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
func (c *APIV1) DeleteRole(roleName string) error {
	url := fmt.Sprintf("%s/_security/role/%s", c.GetEndpoint(), roleName)
	resp, err := c.Request(nil, util.Verb_DELETE, url, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) PutRole(roleName string, body []byte) error {
	url := fmt.Sprintf("%s/_security/role/%s", c.GetEndpoint(), roleName)
	resp, err := c.Request(nil, util.Verb_PUT, url, body)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf(string(resp.Body))
	}
	return nil
}
func (c *APIV1) GetPrivileges() ([]byte, error) {
	url := fmt.Sprintf("%s/_security/privilege", c.GetEndpoint())
	resp, err := c.Request(nil, util.Verb_GET, url, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(string(resp.Body))
	}
	return resp.Body, nil
}
