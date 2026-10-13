package llm

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// SafeClient returns an HTTP client for user-supplied provider URLs. Unless
// allowPrivate is set it refuses to connect to loopback, private, link-local or
// metadata addresses, which stops a tenant from using the base-URL setting to
// make the server probe its own internal network (SSRF). The check runs on the
// resolved address at dial time, so DNS tricks do not bypass it.
func SafeClient(allowPrivate bool, timeout time.Duration) *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second}
	if !allowPrivate {
		d.Control = func(_, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
				return fmt.Errorf("connections to %s are not allowed", host)
			}
			return nil
		}
	}
	return &http.Client{Timeout: timeout, Transport: &http.Transport{DialContext: d.DialContext, Proxy: nil, TLSHandshakeTimeout: 10 * time.Second},
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("redirects are not followed") }}
}

// ValidateBaseURL checks a user-supplied base URL's shape (scheme and host).
func ValidateBaseURL(raw string, allowPrivate bool) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil {
		return errors.New("base URL is not valid")
	}
	if u.Scheme != "https" && !(allowPrivate && u.Scheme == "http") {
		return errors.New("base URL must use https")
	}
	return nil
}
