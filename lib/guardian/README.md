[![GoDoc](https://godoc.org/github.com/shaj13/go-guardian/v2?status.svg)](https://pkg.go.dev/github.com/shaj13/go-guardian/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/shaj13/go-guardian)](https://goreportcard.com/report/github.com/shaj13/go-guardian)
[![Coverage Status](https://coveralls.io/repos/github/shaj13/go-guardian/badge.svg?branch=master)](https://coveralls.io/github/shaj13/go-guardian?branch=master)
[![CircleCI](https://circleci.com/gh/shaj13/go-guardian/tree/master.svg?style=svg)](https://circleci.com/gh/shaj13/go-guardian/tree/master)

| :exclamation:  Cache package has been moved to [libcache](https://github.com/shaj13/libcache) repository |
|----------------------------------------------------------------------------------------------------------|

# Go-Guardian
Go-Guardian is a golang library that provides a simple, clean, and idiomatic way to create powerful modern API and web authentication.

## Overview 
Go-Guardian sole purpose is to authenticate requests, which it does through an extensible set of authentication methods known as strategies.<br>
Go-Guardian does not mount routes or assume any particular database schema, which maximizes flexibility and allows decisions to be made by the developer.<br>
The API is simple: you provide go-guardian a request to authenticate, and go-guardian invoke strategies to authenticate end-user request.<br>
Strategies provide callbacks for controlling what occurs when authentication `should` succeeds or fails.

## Installing 
Using go-guardian is easy. First, use go get to install the latest version of the library.

```sh
go get github.com/shaj13/go-guardian/v2
```
Next, include go-guardian in your application:
```go
import "github.com/shaj13/go-guardian/v2"
```

## Why Go-Guardian?
When building a modern application, you don't want to implement authentication module from scratch;<br>
you want to focus on building awesome software. go-guardian is here to help with that.

Here are a few bullet point reasons you might like to try it out:
* provides simple, clean, and idiomatic API. 
* provides top trends and traditional authentication methods.
* provides two-factor authentication and one-time password as defined in [RFC-4226](https://tools.ietf.org/html/rfc4226) and [RFC-6238](https://tools.ietf.org/html/rfc6238)
* provides a mechanism to customize strategies, even enables writing a custom strategy

## Strategies
> JWT and oauth2 packages provide early access to advanced or experimental
> functionality to get community feedback. Their APIs and functionality may be subject to
> breaking changes in future releases.

* [JWT](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/jwt?tab=doc)
* [Oauth2-JWT](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/oauth2/jwt?tab=doc)
* [Oauth2-Introspection](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/oauth2/introspection?tab=doc)
* [Oauth2-OpenID-userinfo](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/oauth2/userinfo?tab=doc)
* [OpenID-IDToken](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/oauth2/jwt?tab=doc)
* [kubernetes (Token Review)](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/kubernetes?tab=doc)
* [2FA](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/twofactor?tab=doc)
* [Certificate-Based](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/x509?tab=doc)
* [Bearer-Token](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/token?tab=doc)
* [Static-Token](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/token?tab=doc)
* [LDAP](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/ldap?tab=doc)
* [Basic](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/basic?tab=doc)
* [Digest](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/digest?tab=doc)
* [Union](https://pkg.go.dev/github.com/rubyniu105/framework/lib/guardian/auth/strategies/union?tab=doc)

# Examples 
Examples are available on [GoDoc](https://pkg.go.dev/github.com/shaj13/go-guardian/v2) or [Examples Folder](./_examples).

# Documentation
API docs are available on [GoDoc](https://pkg.go.dev/github.com/shaj13/go-guardian/v2).

# Contributing

1. Fork it
2. Download your fork to your PC (`git clone https://github.com/your_username/go-guardian && cd go-guardian`)
3. Create your feature branch (`git checkout -b my-new-feature`)
4. Make changes and add them (`git add .`)
5. Commit your changes (`git commit -m 'Add some feature'`)
6. Push to the branch (`git push origin my-new-feature`)
7. Create new pull request

# License
Go-Guardian is released under the MIT license. See [LICENSE](https://github.com/shaj13/go-guardian/blob/master/LICENSE)
