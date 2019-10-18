package chttp

import (
	"net/http/cookiejar"

	"golang.org/x/net/publicsuffix"
)

// Authenticator is an interface that provides authentication to a server.
type Authenticator interface {
	Authenticate(*Client) error
}

func (a *CookieAuth) setCookieJar(c *Client) {
	// If a jar is already set, just use it
	if c.Jar != nil {
		return
	}
	// cookiejar.New never returns an error
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	c.Jar = jar
}
