# runner: {
#   reset_context: true,
#   default_endpoint: "$[[env.CONSOLE_ENDPOINT]]",
# }

GET /_info
# 200

GET /setting/application
# 200
# {"auth_enabled":true}

GET /health
# 200
# {"setup_required":true,"status":"green"}

POST /elasticsearch/try_connect
{"host":"$[[env.ES_ADDR]]","schema":"$[[env.ES_SCHEMA]]","basic_auth":{"username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"}}
# 200

POST /setup/_validate
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"}}
# 200
# {"success": true}

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"template_ilm"}
# 200
# {"log": "initalize template [template_ilm] succeed","success": true} 

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"rollup"}
# 200
# {"log": "initalize template [rollup] succeed","success": true} 

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"insight"}
# 200
# {"log": "initalize template [insight] succeed","success": true}

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"alerting"}
# 200
# {"log": "initalize template [alerting] succeed","success": true}

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"agent"}
# 200
# {"log": "initalize template [agent] succeed","success": true}

POST /setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"view"}
# 200
# {"log": "initalize template [view] succeed","success": true}

POST /setup/_initialize
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"bootstrap_username":"$[[env.CONSOLE_USERNAME]]","bootstrap_password":"$[[env.CONSOLE_PASSWORD]]","credential_secret":"ci_github_actions"}
# 200
# {"secret_mismatch": false,"success": true}

