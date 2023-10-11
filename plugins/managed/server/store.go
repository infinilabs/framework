/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package server

//save configs to database
//
//func  SaveIngestConfig(ctx context.Context, agentBaseURL string) error {
//	ingestCfg, basicAuth, err := common.GetAgentIngestConfig()
//	if err != nil {
//		return err
//	}
//	if basicAuth != nil && basicAuth.Password != "" {
//		err = SetKeystoreValue(ctx, agentBaseURL, "ingest_cluster_password", basicAuth.Password)
//		if err != nil {
//			return fmt.Errorf("set keystore value to agent error: %w", err)
//		}
//	}
//	err = SaveDynamicConfig(context.Background(), agentBaseURL, "ingest_variables.yml", ingestCfg )
//	if err != nil {
//		return fmt.Errorf("save dynamic config to agent error: %w", err)
//	}
//	return nil
//}


//func  SetKeystoreValue(ctx context.Context, agentBaseURL string, key, value string) error{
//	body := util.MapStr{
//		"key": key,
//		"value": value,
//	}
//	req := &util.Request{
//		Method: http.MethodPost,
//		Path:     "/keystore",
//		Context: ctx,
//		Body: util.MustToJSONBytes(body),
//	}
//	return DoRequest(req, nil)
//}

//func  SaveDynamicConfig(ctx context.Context, agentBaseURL string, filename, content string) error{
//	body := util.MapStr{
//		"configs": util.MapStr{
//			filename: content,
//		},
//	}
//	req := &util.Request{
//		Method: http.MethodPost,
//		Path:    "/config/_update",
//		Context: ctx,
//		Body: util.MustToJSONBytes(body),
//	}
//	return DoRequest(req, nil)
//}

