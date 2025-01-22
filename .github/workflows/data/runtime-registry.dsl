# runner: {
#   reset_context: true,
#   default_endpoint: "$[[env.CONSOLE_ENDPOINT]]",
# }

# // user login
POST /account/login
{"userName": "$[[env.CONSOLE_USERNAME]]","password": "$[[env.CONSOLE_PASSWORD]]","type": "account"}
# register: [
#   {access_token: "_ctx.response.body_json.access_token"}
# ],
# assert: {
#   _ctx.response.status: 200
# }

GET /elasticsearch/status
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }


POST /collection/cluster/_search
{"query":{"bool":{"must":[],"filter":[],"should":[],"must_not":[]}}}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# register: [
#   {cluster_uuid: "_ctx.response.body_json.hits.hits.0._source.cluster_uuid"},
#   {credential_id: "_ctx.response.body_json.hits.hits.0._source.credential_id"},
#   {cluster_name: "_ctx.response.body_json.hits.hits.0._source.name"},
#   {cluster_host: "_ctx.response.body_json.hits.hits.0._source.host"},
#   {cluster_schema: "_ctx.response.body_json.hits.hits.0._source.schema"},
#   {cluster_version: "_ctx.response.body_json.hits.hits.0._source.version"},
#   {cluster_distribution: "_ctx.response.body_json.hits.hits.0._source.distribution"}
# ],
# assert: {
#   _ctx.response.status: 200
# }

GET /credential/_search
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

GET /_info
# 200

GET $[[env.GATEWAY_ENDPOINT]]/_info
# 200

GET $[[env.AGENT_ENDPOINT]]/_info
# 200

POST /instance/try_connect
{"endpoint":"$[[env.CONSOLE_ENDPOINT]]","isTLS":false}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance/try_connect
{"endpoint":"$[[env.GATEWAY_ENDPOINT]]","isTLS":false}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance/try_connect
{"endpoint":"$[[env.AGENT_ENDPOINT]]","isTLS":false}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance
{"endpoint":"$[[env.CONSOLE_ENDPOINT]]","isTLS":false,"instance_id":"console","name":"Conaole","version":{"number":"0.0.1","framework_hash":"N/A","vendor_hash":"N/A","build_hash":"N/A","build_date":"N/A","build_number":"001","eol_date":"N/A"},"status":"Online","tags":["default"],"description":""}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /elasticsearch/try_connect
{"name":"$[[cluster_name]]","host":"$[[cluster_host]]","schema":"$[[cluster_schema]]","credential_id":"$[[credential_id]]","basic_auth":{}}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

PUT /elasticsearch/infini_default_system_cluster
{"name":"$[[cluster_name]]","host":"$[[cluster_host]]","credential_id":"$[[credential_id]]","basic_auth":{},"agent_credential_id":"$[[credential_id]]","agent_basic_auth":{},"monitored":true,"monitor_configs":{"cluster_health":{"enabled":true,"interval":"10s"},"cluster_stats":{"enabled":true,"interval":"10s"},"node_stats":{"enabled":false,"interval":"10s"},"index_stats":{"enabled":false,"interval":"10s"}},"metadata_configs":{"health_check":{"enabled":true,"interval":"10s"},"node_availability_check":{"enabled":true,"interval":"10s"},"metadata_refresh":{"enabled":true,"interval":"10s"},"cluster_settings_check":{"enabled":true,"interval":"10s"}},"discovery":{"enabled":false},"version":"$[[cluster_version]]","schema":"$[[cluster_schema]]","distribution":"$[[cluster_distribution]]","location":{},"cluster_uuid":"$[[cluster_uuid]]"}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance
{"endpoint":"$[[env.GATEWAY_ENDPOINT]]","isTLS":false,"instance_id":"gateway","name":"Gateway","version":{"number":"0.0.1","framework_hash":"N/A","vendor_hash":"N/A","build_hash":"N/A","build_date":"N/A","build_number":"001","eol_date":"N/A"},"status":"Online","tags":["default"],"description":""}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

GET /instance/_search?size=20&keyword=&application=agent
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# register: [
#   {agent_id: "_ctx.response.body_json.hits.hits.0._id"}
# ],
# assert: {
#   _ctx.response.status: 200
# }

GET /instance/$[[agent_id]]/node/_discovery
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance/node/_auto_enroll
{"cluster_id":["infini_default_system_cluster"]}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /elasticsearch/infini_default_system_cluster/_proxy?method=GET&path=%2F.infini_instance%2F_search
{"_source": "_id","query": {"term": {"application.name": {"value": "agent"}}}}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST /instance/$[[agent_id]]/node/_discovery
{"cluster_id":["infini_default_system_cluster"]}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

GET /instance/$[[agent_id]]/node/_discovery
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }