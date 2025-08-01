---
weight: 80
title: "Release Notes"
---

# Release Notes

Information about release notes of INFINI Framework is provided here.


## Latest (In development)  
### ❌ Breaking changes  
### 🚀 Features  
### 🐛 Bug fix  
### ✈️ Improvements  
- chore: less logging for session store change #180

## 1.2.0 (2025-07-25)
### ❌ Breaking changes  
### 🚀 Features  
- feat: add hooks for ORM data operation #167
- feat: merge term filters to terms filter with same field #173
- feat: add create api for indexing #175

### 🐛 Bug fix  
- fix: HTTP headers config was not applied with plugin `http`
- fix: support underscore '_' in quoted JSON keys

### ✈️ Improvements  
- refactor: refactoring query interface 
- feat: url query support terms query #163

## 1.1.9 (2025-06-29)
### ❌ Breaking changes  
### 🚀 Features  
### 🐛 Bug fix  
- fix: response without a body can't have trailers (#158)
### ✈️ Improvements  
- chore: refactoring error handling #157
- chore: update makefile #156

## 1.1.8 (2025-06-13)
### ❌ Breaking changes  
### 🚀 Features  
### 🐛 Bug fix  
### ✈️ Improvements  
- refactor: refactoring orm query #153

## 1.1.7 (2025-05-16)
### ❌ Breaking changes  
### 🚀 Features  
- feat: fasttemplate add util to support rendering nested variables in template #144
- feat: introduce config to check_capacity retry threshold limit
- feat: support custom TLS minimum version for SMTP server configuration
- feat: add filter to orm query #146
- feat: add util to filter user's input

### 🐛 Bug fix
- fix: wrong method call during ORM update
- fix: add missing score field, fix orm filter #147

### ✈️ Improvements  
- refactor: refactoring orm struct mapping and search api
- chore: check disk capacity when disk queue module start #136
- refactor: refactoring orm module #145


## 1.1.6 (2025-04-27)
### Breaking changes  

### Features  
- feat: add query query_string and prefix support to orm module
- feat: add compression support to HTTP processor
- feat: allow to register callback after setup
- feat: add adaptor for elasticsearch v9

### Bug fix  
- fix: fix WriteHeader to prevent duplicate status code writes
- fix: ensure 200 status code is set before writing response in HTTP handler
- fix: reload and notify when pipeline config changes are detected
- fix: register cluster status loss when default setting is `green`

### Improvements  
- chore: add util to get instance id
- chore: add util to delete session key
- chore: update profile structs
- chore: set service restart policy to always
- chore: add util to write response for record not found
- chore: default to support go modules

## 1.1.5 (2025-03-31)
### Breaking changes  
- refactor: refactoring auth config to security config for web module

### Features  
- feat: general pluggable security feature
- feat: add cors settings to UI handler

### Bug fix  
- fix: cluster init status loss when default setting is green

### Improvements  
- Refactoring elasticsearch error base
- chore: no panic during redis start
- chore: skip Cancel in task context for json serialization
- chore: no newline in logging
- chore: update logging message
- chore: add more options to api
- chore: lower priority filter should execute first
- chore: refactory permission options to array
- chore: add support for converting floats in InterfaceToInt
- chore: add util to access api feature option
- chore: remove unnecessary lock
- chore: update api labels to support interface

## 1.1.4 (2025-03-14)
### Breaking changes  
### Features  
- Add configuration option to disable echo messages during WebSocket connection #96
- Allow to register callback on websocket's connect/disconnect #96
- Add optional login feature flag to api

### Bug fix  
### Improvements  
- Fixed task should be cancel on stop #97
- Close the websocket connection on callback error


## 1.1.3 (2025-02-27)

### Breaking changes

### Features
- Allow registering functions to execute before application setup (#84)
- Add utility to securely marshal JSON (#85)

### Bug fix
- Fixed comsumer segment without producer (#89)
- Disable default proxy when proxy is not enabled #91


### Improvements
- Structure http error response (#86)
- Introduce system type to echo welcome message in websocket (#87)


## v1.1.2 (2025-02-15)

### Bug fix
- Fixed `[]byte` operator when queue comsumer paic (#77)
- Fixed incorrect interval configuration in index stats collection task (#80)
- Fixed reload file need use privious pos (#79)
- Fixed nil panic by init cluster health default status to green (#81) 
- Fixed path walk warn when tmp rename (#82)

### Improvements

- Refactor loopback address to use const (#73)
- Add debug message for `queue` comsumer (#77)


## v1.1.1 (2025-01-24)

### Features

- Add new search function to orm module, support result item mapper (#65)
- Add new stats api to quickly find the top N keys from a Badger DB (#67)
- Proactively restore dead node's availability (#72)

### Bug fix
- Fixed client sync config panic when config folder not exits (#71)

### Improvements

- Add util to http handler, support to parse bool parameter
- Handle simplified bulk metdata, parse index from url path (#59)
- Improve handling of message read for partially loaded files (#63)

## v1.1.0 (2025-01-11) 

### Breaking changes

- Update WebSocket greeting message header to use `websocket-session-id`

### Features

- Set the metric collection task to singleton mode (#17)
- Record cluster allocation explain to activity after cluster health status changed to `red`
- Add elastic api method `ClusterAllocationExplain`
- Add `min_bucket_size` and `hits_total` to metric configurations (#29)
- Add proxy settings to `http_client` config section (#33)
- Add new condition to check item length (eg: array,string) (#38)
- Fixed issue with console LDAP config with dot key [#46](https://github.com/infinilabs/console/issues/46)

### Breaking changes

- Add util to http handler, support write bytes with status code (#55)

### Bug fix

- Remove the collection of cluster stats metric in node stats collection task (#17)
- Fix the main switch of the cluster metric is not work (#17)
- Fixed the issue that the metadata does not take effect immediately after the cluster changes to available (#23)
- Enable skipping to the next file with multiple gaps (#22)
- Removing the logic of collecting metric per each node (#26)
- Fixed to parse password from basic auth (#31)
- Fixed issue with metric collection task interval not working (#30)
- Fix invalid data folder, remove cluster_config and use appname directly for configuration (#46)
- Fixed incorrect system cluster health status in the health API (#39)

### Improvements

- Add commit hashes for framework and managed vendor dependencies
- Trim spaces from input variables during app initialization
- Auto init the badger db for the first time access (#27)
- Add search response to logging message (#28)

## v1.0.0 (2024-12-13)

### 🚀 Features

- Add option to keep compatible with old consumer config
- Add timeout when push message to disk_queue
- Auto skip missing file for consumer (#491)
- _(queue)_ Skip missing till to latest file (#488)
- _(util)_ Add a function to clear all registered IDs
- Register background job to clean up badger LSM tree (#529)
- Allow to use default auth for agent's auto enroll
- Use http body to pass scroll_id for next scroll fetch
- Add http interceptor
- Add node labels to agent metadata
- Return host info in info api
- Crontab task support multi crontab expression
- Add CGO flag to Makefile
- Add new config field 'MetricCollectionMode'
- Add util to convert string to float
- Use common app setting api to instead of auth setting api
- Add easysearch ccr api
- Add delete autofollow rule api
- Update follower list api
- Add cluster settings query args
- Add gateway config
- Support dynamic app setting
- Check consumer before acquire (#573)
- Provide config to use doc_id as hash factor, use message offset as default hash factor"
- Message level slicing in bulk_indexing
- Add last_access_time to queue stats
- Support record last active timestamp
- Return cpu info in info api
- Add field HeapMax in struct CatNodeResponse
- Support customize event queue
- Support tz draft
- Crontab task support timezone
- Support passing query param level to cluster health api
- Add param context for es api ClusterHealth and ClusterStats
- Add configs param `allow_generated_metrics_tasks`
- Support filter config file (#620)
- Auto issue certificates for domain
- Add es flush api
- Allow to append or override tempates in orm module (#640)
- Support search templates in orm (#643)
- Add util to get queue config with queue config
- Add singleton option to tasks
- Add key_field to indexing_merge processor
- Supports secure display of secret fields (#656)
- Add api to list & delete files(#659)
- feat: add util to validate request (#7)
- feat: validate version branch, add product_name to commit message

### 🐛 Bug Fixes

- Skip load missing wal
- Skip node info missing (#519)
- Panic: kv store handler is not registered (#525)
- Add default generated.go
- Change branch
- Fix offset check across versions
- Change to false by default
- Zstd command
- Incorrect ZSTD compression
- Panic on error while saving keystore, #514
- Prevent close closed channel
- Wrong use of zstd with vfs
- Passed empty scroll request body
- Get latest offset should compare segment first
- Skip submit empty bulk requests
- Remove unnecessary offset reset
- Handle the offset
- Concurrent map read and map write with queue labels
- Consumer should handle slice config
- Handle dirty read when file is still active write
- Check consumer api before use
- Query_string query was ignored (#588)
- Wait group usage in bulk_indexing processor
- Prevent consumer from advancing beyond writer's segment
- Reload when file is in dirty read
- Getting cluster version with timeout
- When file not exits continue delete (#614)
- Make test (#615)
- Rollback for client register (#619)
- Agent labels not work
- Refactoring inflight check
- Queue consumer not skip to next file (#636)
- Build error when github pull error (#637)
- Getting empty node id (#639)
- Panic while init search instance
- Path conflick with :instanceid (#661)
- Fix path blank
- fix: return error after failed to create http request (#10)
- fix: cluster health change callback was not triggered (#9)
- fix: avoid copy atomic value (#8)
- feat: adding field `Request` to record metric request statement (#2)
- feat: allow to prioritize global vendor over managed vendor via environment key

### 🚜 Refactor

- Logging
- Unify tls config for http client
- Refactor func GetFieldCaps
- Fix err log
- Optimize app state checking performance
- Add queue_id when checking inflight consumers
- Refactoring system config
- Refactoring tasks
- Refactoring ui to web
- chore: splitting metric collecting task (#6)

### 🧪 Testing

- Add zstd tests
- Add xxhash tests

### ⚙️ Miscellaneous Tasks

- Add back es monitoring api
- Cleanup logging
- Cleanup unused code
- Remove proxy binary file
- Fix build, keep compatibility with old golang version
- Update logging level
- Update logging message
- No panic on config during init
- Misc fixes (#568)
- Reduce logging impact, improve performance
- Update stats key
- Add queue name to log message
- Update config key
- Update logging message
- Add stats when dirty read
- Update cli naming style (#589)
- Log with ip and agent register when restart aging
- Adjust logging format
- Add util to parse parameter, panic on missing
- Rollback agent meta labels
- Update vendor repo (#626)
- Update default branch for vendor
- Add uuid to websocket session
- Handle session_id for websocket
- Fix typo
- Remove unused config
- Update loggings for autocerts
- Update license header
- Add util to register schema
- Remove unused code
- Update to use main branch (#641)
- Throw error when schema is not valid (#642)
- Prioritize global vendor over managed vendor
- Add some debug log
- Add some debug log with quque
- Add stack dump for log (#657)
- Add stack_trace and improve log messages (#658)
- Add build arm64 for ci (#663)
- Update license (#664)
- chore: optimize ES metric collecting (#4)
- chore: fixed node id of elastic metadata (#3)
- chore: update default repo

### Build

- _(makefile)_ Support multiple `APP_CONFIG` files

## [20231228] - 2023-12-29

### 🧪 Testing

- _(mapstr)_ Fix wrong check logic (#477)

### ⚙️ Miscellaneous Tasks

- Merge code from console (#476)
- No error when temp file was missing (#480)

## [20231214] - 2023-12-13

### ⚙️ Miscellaneous Tasks

- Output welcome message at the very beginning (#462)

## [20231130] - 2023-12-01

### 🚀 Features

- Add http processor (#426)
- Update tenant domain (#448)

### 🐛 Bug Fixes

- Quit consumer when shutting down (#430)
- Fix user's schema, add default API handler (#447)

### ⚙️ Miscellaneous Tasks

- Refactoring (#440)

## [20231116] - 2023-11-15

### 🚀 Features

- Throttle the disk capacity check (#416)
- Multi-tenant (#411)

### 🐛 Bug Fixes

- _(conditions)_ Ignore nil placeholders (#422)

### ⚙️ Miscellaneous Tasks

- Remove log message in service mode (#417)

## [20231102] - 2023-11-02

### 🚀 Features

- Report assertion errors (#397)
- Allow to skip config missing or parse error (#398)
- Expose APIs to render config template (#401)
- Allow config to void being managed (#402)
- Add simple_kv module (#404)
- _(mapstr)_ Support array index (#403)

### ⚙️ Miscellaneous Tasks

- Refactoring module location (#405)
- Remove unused api (#409)

## [20230921] - 2023-09-20

### 🚀 Features

- Refactoring resource limit, add cpu limit (#382)

### 🐛 Bug Fixes

- Cleanup invalid lock file (#380)
- Delete lock on more panic (#381)
- Queue selector by labels, if more than one labels specified, they should all match together neither any match (#383)

### 🚜 Refactor

- Update configs
- Remove unused api

### ⚙️ Miscellaneous Tasks

- _(build)_ Prepare plugins in framework before build (#379)

## [20230629] - 2023-06-29

### ⚙️ Miscellaneous Tasks

- Safety shutdown app (#354)

## [0.3.1] - 2023-01-18

### 🚀 Features

- WriteOkJson func

### Vfs

- Reslove default folder to fetch index.html

## v0.1.0

### breaking changes

1. Refactoring API/Cluster/Pipeline, config section moved out of module
2. Namespace moved to infini.sh

### features

1. Support offline build, `OFFLINE_BUILD=true make build`
2. Add error handler to pipeline
3. Auto generate TLS certs
4. Support Check if PID is running on windows
5. Update VFS
6. Support Setup alias on network interface
7. Support Add callback functions to execute on shutdown

### improvement

1. Add elasticsearch adaptors for major versions
2. Refactor webhunter, add utils
3. Unify elasticsearch configuration, reference by id
4. Support custom header in webhunter
5. Remove static files from framework

### bugfix

1. Fix VFS issue, static was not work with empty local folder
