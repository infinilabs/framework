# // user login
POST $[[env.CONSOLE_ENDPOINT]]/account/login
{"userName": "$[[env.CONSOLE_USERNAME]]","password": "$[[env.CONSOLE_PASSWORD]]","type": "account"}
# register: [
#   {access_token: "_ctx.response.body_json.access_token"}
# ],
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

POST $[[env.CONSOLE_ENDPOINT]]/elasticsearch/infini_default_system_cluster/_proxy?method=GET&path=%2F.infini_metrics%2F_count
{"query":{"bool":{"must":[{"term":{"agent.id":{"value":"$[[agent_id]]"}}},{"term":{"category":{"value":"elasticsearch"}}}]}}}
# request: {
#   headers: [
#     {authorization: "Bearer $[[access_token]]"}
#   ],
#   disable_header_names_normalizing: false
# },
# assert: {
#   _ctx.response.status: 200
# }

POST $[[env.ES_ENDPOINT]]/.infini_metrics/_count
{"query":{"bool":{"must":[{"term":{"agent.id":{"value":"$[[agent_id]]"}}},{"term":{"category":{"value":"elasticsearch"}}}]}}}
# assert: {
#   _ctx.response.status: 200,
#   _ctx.response.body_json.count: >=1
# }