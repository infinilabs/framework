DELETE $[[env.ES_ENDPOINT]]/.infini*

DELETE $[[env.ES_ENDPOINT]]/_template/.infini*

GET $[[env.CONSOLE_ENDPOINT]]/_info
# 200

GET $[[env.GATEWAY_ENDPOINT]]/_info
# 200

GET $[[env.AGENT_ENDPOINT]]/_info
# 200

GET $[[env.CONSOLE_ENDPOINT]]/setting/application
# 200
# {"auth_enabled":true}

GET $[[env.CONSOLE_ENDPOINT]]/health
# 200
# {"setup_required":true,"status":"green"}

POST $[[env.CONSOLE_ENDPOINT]]/elasticsearch/try_connect
{"host":"$[[env.ES_ADDR]]","schema":"$[[env.ES_SCHEMA]]","basic_auth":{"username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"}}
# 200

POST $[[env.CONSOLE_ENDPOINT]]/setup/_validate
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"}}
# 200
# {"success": true}

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"template_ilm"}
# 200
# {"log": "initalize template [template_ilm] succeed","success": true} 

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"rollup"}
# 200
# {"log": "initalize template [rollup] succeed","success": true} 

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"insight"}
# 200
# {"log": "initalize template [insight] succeed","success": true}

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"alerting"}
# 200
# {"log": "initalize template [alerting] succeed","success": true}

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"agent"}
# 200
# {"log": "initalize template [agent] succeed","success": true}

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize_template
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"initialize_template":"view"}
# 200
# {"log": "initalize template [view] succeed","success": true}

POST $[[env.CONSOLE_ENDPOINT]]/setup/_initialize
{"cluster":{"endpoint":"$[[env.ES_ENDPOINT]]","username":"$[[env.ES_USERNAME]]","password":"$[[env.ES_PASSWORD]]"},"bootstrap_username":"$[[env.CONSOLE_USERNAME]]","bootstrap_password":"$[[env.CONSOLE_PASSWORD]]","credential_secret":"ci_github_actions"}
# 200
# {"secret_mismatch": false,"success": true}

