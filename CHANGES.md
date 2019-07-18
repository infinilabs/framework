# An Golang Framework #

## CHANGES


#### v0.1.0

##### breaking changes
1. Refactoring API/Cluster, config section moved out of module

##### features
1. Support offline build,  `OFFLINE_BUILD=true make build`
2. Add error handler to pipeline
3. Auto generate TLS certs

##### improvement
1. Add elasticsearch adaptors for major versions 
2. Refactor webhunter, add utils
3. Unify elasticsearch configuration, reference by id
4. Support custom header in webhunter

##### bugfix
