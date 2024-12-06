// Package ldap provides authentication strategy,
// to authenticate HTTP requests and builds, extracts user informations from LDAP Server.
package ldap

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"github.com/go-ldap/ldap/v3"
	"github.com/rubyniu105/framework/core/util"
	"github.com/rubyniu105/framework/lib/fasthttp"
	"github.com/rubyniu105/framework/lib/guardian/auth"
	"github.com/rubyniu105/framework/lib/guardian/auth/strategies/basic"
	"strings"
)

// ErrEntries is returned by ldap authenticate function,
// When search result return user DN does not exist or too many entries returned.
var ErrEntries = errors.New("strategies/ldap: Search user DN does not exist or too many entries returned")

type conn interface {
	Bind(username, password string) error
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	StartTLS(config *tls.Config) error
	UnauthenticatedBind(username string) error
	Close()
}

// Config define the configuration to connect to LDAP.
type Config struct {
	// Port LDAP server port.
	Port int
	// Host LDAP server host.
	Host string
	// TLS configuration, if nil connect without TLS.
	TLS *tls.Config
	// BindDN represents LDAP DN for searching for the user DN.
	// Typically read only user DN.
	BindDN string
	// BindPassword LDAP password for searching for the user DN.
	// Typically read only user password.
	BindPassword string
	// Attributes used for users.
	Attributes []string
	// BaseDN LDAP domain to use for users.
	BaseDN string
	// Filter for the User Object Filter.
	// if username nedded more than once use fmt index pattern (%[1]s).
	// Otherwise %s.
	UserFilter     string
	GroupFilter    string
	UIDAttribute   string
	GroupAttribute string
}

func dial(cfg *Config) (conn, error) {
	scheme := "ldap"
	opts := []ldap.DialOpt{}

	if cfg.TLS != nil {
		scheme = "ldaps"
		opts = append(opts, ldap.DialWithTLSConfig(cfg.TLS))
	}

	addr := fmt.Sprintf("%s://%s:%v", scheme, cfg.Host, cfg.Port)
	return ldap.DialURL(addr, opts...)
}

type client struct {
	dial func(cfg *Config) (conn, error)
	cfg  *Config
}

func (c client) authenticate(ctx context.Context, r *fasthttp.Request, userName, password []byte) (auth.Info, error) { //nolint:lll
	l, err := c.dial(c.cfg)

	if err != nil {
		return nil, err
	}

	defer l.Close()

	if c.cfg.BindPassword != "" {
		err = l.Bind(c.cfg.BindDN, c.cfg.BindPassword)
	} else {
		err = l.UnauthenticatedBind(c.cfg.BindDN)
	}

	if err != nil {
		return nil, err
	}

	result, err := l.Search(&ldap.SearchRequest{
		BaseDN:     c.cfg.BaseDN,
		Scope:      ldap.ScopeWholeSubtree,
		Filter:     fmt.Sprintf(c.cfg.UserFilter, userName),
		Attributes: c.cfg.Attributes,
	})

	if err != nil {
		return nil, err
	}

	if len(result.Entries) != 1 {
		return nil, ErrEntries
	}

	err = l.Bind(result.Entries[0].DN, util.UnsafeBytesToString(password))

	if err != nil {
		return nil, err
	}

	id := ""
	ext := map[string][]string{}
	groups := []string{}

	for _, attr := range result.Entries[0].Attributes {
		name := attr.Name
		values := attr.Values

		if name == c.cfg.UIDAttribute {
			id = values[0]
			continue
		}

		if name == c.cfg.GroupAttribute {
			groups = append(groups, values...)
		}

		ext[name] = values
	}
	if len(groups) == 0 {
		//try to extract first group from dn
		if grp := extractFirstGroupFromDN(result.Entries[0].DN, c.cfg.GroupAttribute); grp != "" {
			groups = append(groups, grp)
		}
	}

	//
	//result, err = l.Search(ldap.NewSearchRequest(
	//	c.cfg.BaseDN,
	//	ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
	//	fmt.Sprintf(c.cfg.GroupFilter, userName),
	//	[]string{c.cfg.GroupAttribute}, // can it be something else than "cn"?
	//	nil,
	//))
	//
	//fmt.Println(result,err)
	//
	//for _, entry := range result.Entries {
	//	groups = append(groups, entry.GetAttributeValue(c.cfg.GroupAttribute))
	//}

	return auth.NewUserInfo(util.UnsafeBytesToString(userName), id, groups, ext), nil
}

// GetAuthenticateFunc return function to authenticate request using LDAP.
// The returned function typically used with the basic strategy.
func GetAuthenticateFunc(cfg *Config, opts ...auth.Option) basic.AuthenticateFunc {
	cl := new(client)
	cl.dial = dial
	cl.cfg = cfg
	return cl.authenticate
}

// New return strategy authenticate request using LDAP.
// New is similar to Basic.New().
func New(cfg *Config, opts ...auth.Option) auth.Strategy {
	fn := GetAuthenticateFunc(cfg, opts...)
	return basic.New(fn, opts...)
}

// NewCached return strategy authenticate request using LDAP.
// The returned strategy, caches the authentication decision.
// New is similar to Basic.NewCached().
func NewCached(cfg *Config, c auth.Cache, opts ...auth.Option) auth.Strategy {
	fn := GetAuthenticateFunc(cfg, opts...)
	return basic.NewCached(fn, c, opts...)
}

// extractFirstGroupFromDN try to extract first group from dn
func extractFirstGroupFromDN(dn string, groupAttr string) string {
	dnParts := strings.Split(dn, ",")
	for _, part := range dnParts {
		kvs := strings.Split(part, "=")
		if kvs[0] == groupAttr && len(kvs) == 2 {
			return kvs[1]
		}
	}
	return ""
}
