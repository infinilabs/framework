/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

func connect() {

	//// TLS证书解析验证
	//if _, err = tls.LoadX509KeyPair(G_config.ServerPem, G_config.ServerKey); err != nil {
	//	//return common.ERR_CERT_INVALID
	//}
	//
	//transport := &http.Transport{
	//	TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // 不校验服务端证书
	//	MaxIdleConns:        G_config.GatewayMaxConnection,
	//	MaxIdleConnsPerHost: G_config.GatewayMaxConnection,
	//	IdleConnTimeout:     time.Duration(G_config.GatewayIdleTimeout) * time.Second, // 连接空闲超时
	//}
	//
	//// 启动HTTP/2协议
	//http2.ConfigureTransport(transport)
	//
	//// HTTP/2 客户端
	//gateConn.client = &http.Client{
	//	Transport: transport,
	//	Timeout:   time.Duration(G_config.GatewayTimeout) * time.Millisecond, // 请求超时
	//}
}
