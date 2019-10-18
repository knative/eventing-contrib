package chttp

import (
	"context"
	"net/http"

	"github.com/go-kivik/kivik"
)

// CookieAuth provides CouchDB Cookie auth services as described at
// http://docs.couchdb.org/en/2.0.0/api/server/authn.html#cookie-authentication
//
// CookieAuth stores authentication state after use, so should not be re-used.
type CookieAuth struct {
	Username string `json:"name"`
	Password string `json:"password"`

	client *Client
	// transport stores the original transport that is overridden by this auth
	// mechanism
	transport http.RoundTripper
}

var _ Authenticator = &CookieAuth{}

// Authenticate initiates a session with the CouchDB server.
func (a *CookieAuth) Authenticate(c *Client) error {
	a.setCookieJar(c)
	a.client = c
	a.transport = c.Transport
	if a.transport == nil {
		a.transport = http.DefaultTransport
	}
	c.Transport = a
	return nil
}

// Cookie returns the current session cookie if found, or nil if not.
func (a *CookieAuth) Cookie() *http.Cookie {
	if a.client == nil {
		return nil
	}
	for _, cookie := range a.client.Jar.Cookies(a.client.dsn) {
		if cookie.Name == kivik.SessionCookieName {
			return cookie
		}
	}
	return nil
}

var authInProgress = &struct{ name string }{"in progress"}

// RoundTrip fulfills the http.RoundTripper interface. It sets
// (re-)authenticates when the cookie has expired or is not yet set.
func (a *CookieAuth) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := a.authenticate(req); err != nil {
		return nil, err
	}
	return a.transport.RoundTrip(req)
}

func (a *CookieAuth) authenticate(req *http.Request) error {
	ctx := req.Context()
	if inProg, _ := ctx.Value(authInProgress).(bool); inProg {
		return nil
	}
	if _, ok := req.Header["Cookie"]; ok {
		// Request already authenticated
		return nil
	}
	a.client.authMU.Lock()
	defer a.client.authMU.Unlock()
	if c := a.Cookie(); c != nil {
		// In case another simultaneous process authenticated successfully first
		req.AddCookie(c)
		return nil
	}
	ctx = context.WithValue(ctx, authInProgress, true)
	opts := &Options{
		GetBody: BodyEncoder(a),
		Header: http.Header{
			HeaderIdempotencyKey: []string{},
		},
	}
	if _, err := a.client.DoError(ctx, http.MethodPost, "/_session", opts); err != nil {
		return err
	}
	if c := a.Cookie(); c != nil {
		req.AddCookie(c)
	}
	return nil
}
