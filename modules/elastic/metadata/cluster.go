/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package metadata

import (
	"errors"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

//get elasticsearch meta from system cluster

func GetClusterInformation(clusterID string) (*elastic.ClusterInformation, error) {
	cfg, err := GetClusterConfig(clusterID)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	info := &elastic.ClusterInformation{}
	info.ClusterName = cfg.RawName
	info.ClusterUUID = cfg.ClusterUUID
	info.Version.Number = cfg.Version
	return info, nil
}

func GetClusterConfig(clusterID string) (*elastic.ElasticsearchConfig, error) {
	q1 := &orm.Query{Size: 100}
	q1.Conds = orm.And(
		orm.Eq("id", clusterID),
	)
	err, result := orm.Search(elastic.ElasticsearchConfig{}, q1)
	if err != nil {
		return nil, err
	}
	if len(result.Result) > 0 {
		bytes := util.MustToJSONBytes(result.Result[0])
		info := &elastic.ElasticsearchConfig{}
		err := util.FromJSONBytes(bytes, info)
		if err != nil {
			return nil, err
		}
		return info, nil
	}
	return nil, errors.New("not found")
}

func GetNodeInformation(clusterID string, nodeUUIDs []string) (map[string]*elastic.NodesInfo, error) {
	res, err := GetNodeConfigs(clusterID, nodeUUIDs)
	if err != nil {
		log.Error(err, len(res))
		return nil, err
	}
	results := map[string]*elastic.NodesInfo{}
	for k, v := range res {
		results[k] = v.Payload.NodeInfo
	}
	return results, nil
}

func GetNodeConfig(clusterID string, nodeUUIDs string) (*elastic.NodeConfig, error) {
	res, err := GetNodeConfigs(clusterID, []string{nodeUUIDs})
	if err != nil {
		return nil, err
	}
	info, ok := res[nodeUUIDs]
	if !ok || info == nil {
		return nil, errors.New("not found")
	}
	return info, nil
}

func GetNodeConfigs(clusterID string, nodeUUIDs []string) (map[string]*elastic.NodeConfig, error) {
	q1 := &orm.Query{Size: 1000}
	q1.Conds = orm.And(
		orm.Eq("metadata.category", "elasticsearch"),
		orm.Eq("metadata.cluster_id", clusterID),
		orm.InStringArray("metadata.node_id", nodeUUIDs),
	)

	err, result := orm.Search(elastic.NodeConfig{}, q1)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	results := map[string]*elastic.NodeConfig{}
	if len(result.Result) > 0 {
		for _, v := range result.Result {
			bytes := util.MustToJSONBytes(v)
			info := &elastic.NodeConfig{}
			err := util.FromJSONBytes(bytes, info)
			if err != nil {
				log.Error(err)
				continue
			}
			results[info.Metadata.NodeID] = info
		}
		return results, nil
	}
	return nil, errors.New("not found")
}
