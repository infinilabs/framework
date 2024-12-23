---
weight: 80
title: "Release Notes"
---

# Release Notes

Information about release notes of INFINI Framework is provided here.

## Latest (In development)
### Features
- Set the metric collection task to singleton mode (#17)
- Record cluster allocation explain to activity after cluster health status changed to `red`
- Add elastic api method `ClusterAllocationExplain`

### Breaking changes

### Bug fix
- Remove the collection of cluster stats metric in node stats collection task (#17)
- Fix the main switch of the cluster metric is not work (#17)
- Update elastic metadata safely (#20)
- Fixed the issue that the metadata does not take effect immediately after the cluster changes to available (#23)

### Improvements
- chore: add commit hashes for framework and managed vendor dependencies
- chore: trim spaces from input variables during app initialization


## v1.0.0

### 🚀 Features

- Add option to keep compatible with old consumer config
- Add timeout  when push message to disk_queue
- Auto skip missing file for consumer (#491)
- *(queue)* Skip missing till to latest file (#488)
- *(util)* Add a function to clear all registered IDs
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
- Change  to false by default
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

- *(makefile)* Support multiple `APP_CONFIG` files

## [20231228] - 2023-12-29

### 🧪 Testing

- *(mapstr)* Fix wrong check logic (#477)

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

- *(conditions)* Ignore nil placeholders (#422)

### ⚙️ Miscellaneous Tasks

- Remove log message in service mode (#417)

## [20231102] - 2023-11-02

### 🚀 Features

- Report assertion errors (#397)
- Allow to skip config missing or parse error (#398)
- Expose APIs to render config template (#401)
- Allow config to void being managed (#402)
- Add simple_kv module (#404)
- *(mapstr)* Support array index (#403)

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

- *(build)* Prepare plugins in framework before build (#379)

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
1. Support offline build,  `OFFLINE_BUILD=true make build`
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
