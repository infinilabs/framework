# // user login
POST $[[env.CONSOLE_ENDPOINT]]/account/login
{"userName": "$[[env.CONSOLE_USERNAME]]","password": "$[[env.CONSOLE_PASSWORD]]","type": "account"}
# register: [
#   {access_token: "_ctx.response.body_json.access_token"}
# ],
# assert: {
#   _ctx.response.status: 200
# }

GET $[[env.CONSOLE_ENDPOINT]]/elasticsearch/status
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

GET $[[env.GATEWAY_ENDPOINT]]/_info
# 200

GET $[[env.AGENT_ENDPOINT]]/_info
# 200

POST $[[env.CONSOLE_ENDPOINT]]/instance/try_connect
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

POST $[[env.CONSOLE_ENDPOINT]]/instance/try_connect
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

POST $[[env.CONSOLE_ENDPOINT]]/instance/try_connect
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

POST $[[env.CONSOLE_ENDPOINT]]/instance
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

POST $[[env.CONSOLE_ENDPOINT]]/instance
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

POST $[[env.CONSOLE_ENDPOINT]]/instance/node/_auto_enroll
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

POST $[[env.CONSOLE_ENDPOINT]]/elasticsearch/infini_default_system_cluster/_proxy?method=GET&path=%2F.infini_instance%2F_search
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

GET $[[env.CONSOLE_ENDPOINT]]/instance/_search?size=20&keyword=&application=agent
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

GET $[[env.CONSOLE_ENDPOINT]]/instance/$[[agent_id]]/node/_discovery
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }