# INFINI Golang Framework

## CHANGES


#### v0.1.0

##### breaking changes
1. Refactoring API/Cluster/Pipeline, config section moved out of module
2. Namespace moved to infini.sh

##### features
1. Support offline build,  `OFFLINE_BUILD=true make build`
2. Add error handler to pipeline
3. Auto generate TLS certs
4. Support Check if PID is running on windows
5. Update VFS
6. Support Setup alias on network interface
7. Support Add callback functions to execute on shutdown

##### improvement
1. Add elasticsearch adaptors for major versions 
2. Refactor webhunter, add utils
3. Unify elasticsearch configuration, reference by id
4. Support custom header in webhunter
5. Remove static files from framework

##### bugfix
1. Fix VFS issue, static was not work with empty local folder
