package portal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	RequestInterface      = "org.freedesktop.portal.Request"
	RequestResponseSignal = RequestInterface + ".Response"
	SessionInterface      = "org.freedesktop.portal.Session"
	SessionCloseMethod    = SessionInterface + ".Close"
)

type RequestResponse struct {
	Code    uint32
	Results map[string]dbus.Variant
}

func (c *Client) uniqueName() (string, error) {
	for _, name := range c.conn.Names() {
		if strings.HasPrefix(name, ":") {
			return name, nil
		}
	}
	return "", fmt.Errorf("session bus unique name is unavailable")
}

func requestPathNamespace(uniqueName string) dbus.ObjectPath {
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/request/" + sanitizeUniqueName(uniqueName))
}

func requestPath(uniqueName, token string) dbus.ObjectPath {
	return dbus.ObjectPath(string(requestPathNamespace(uniqueName)) + "/" + token)
}

func sessionPath(uniqueName, token string) dbus.ObjectPath {
	return dbus.ObjectPath("/org/freedesktop/portal/desktop/session/" + sanitizeUniqueName(uniqueName) + "/" + token)
}

func sanitizeUniqueName(name string) string {
	name = strings.TrimPrefix(name, ":")
	return strings.ReplaceAll(name, ".", "_")
}

func makeToken(prefix string) string {
	var buf [6]byte
	if _, err := rand.Read(buf[:]); err == nil {
		return prefix + "_" + hex.EncodeToString(buf[:])
	}
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func (c *Client) watchRequestResponses(ctx context.Context, uniqueName string) (chan *dbus.Signal, func(), error) {
	match := []dbus.MatchOption{
		dbus.WithMatchSender(DesktopBusName),
		dbus.WithMatchInterface(RequestInterface),
		dbus.WithMatchMember("Response"),
		dbus.WithMatchPathNamespace(requestPathNamespace(uniqueName)),
	}
	if err := c.conn.AddMatchSignalContext(ctx, match...); err != nil {
		return nil, nil, err
	}

	ch := make(chan *dbus.Signal, 8)
	c.conn.Signal(ch)

	cleanup := func() {
		c.conn.RemoveSignal(ch)
		_ = c.conn.RemoveMatchSignal(match...)
	}

	return ch, cleanup, nil
}

func awaitRequestResponse(ctx context.Context, ch <-chan *dbus.Signal, handle dbus.ObjectPath) (RequestResponse, error) {
	for {
		select {
		case <-ctx.Done():
			return RequestResponse{}, ctx.Err()
		case sig, ok := <-ch:
			if !ok {
				return RequestResponse{}, fmt.Errorf("portal request listener closed")
			}
			if sig == nil || sig.Name != RequestResponseSignal || sig.Path != handle {
				continue
			}
			if len(sig.Body) != 2 {
				return RequestResponse{}, fmt.Errorf("portal request %s returned malformed response body", handle)
			}

			code, ok := sig.Body[0].(uint32)
			if !ok {
				return RequestResponse{}, fmt.Errorf("portal request %s returned malformed response code", handle)
			}
			results, ok := sig.Body[1].(map[string]dbus.Variant)
			if !ok {
				return RequestResponse{}, fmt.Errorf("portal request %s returned malformed results", handle)
			}

			response := RequestResponse{Code: code, Results: results}
			if code != 0 {
				return response, fmt.Errorf("portal request %s failed with response %d", handle, code)
			}
			return response, nil
		}
	}
}

func variantObjectPath(results map[string]dbus.Variant, key string) (dbus.ObjectPath, error) {
	value, ok := results[key]
	if !ok {
		return "", fmt.Errorf("portal result %q is missing", key)
	}

	switch path := value.Value().(type) {
	case dbus.ObjectPath:
		return path, nil
	case string:
		return dbus.ObjectPath(path), nil
	default:
		return "", fmt.Errorf("portal result %q is not an object path", key)
	}
}

func variantBool(results map[string]dbus.Variant, key string) (bool, bool) {
	value, ok := results[key]
	if !ok {
		return false, false
	}
	flag, ok := value.Value().(bool)
	return flag, ok
}

func variantUint32(results map[string]dbus.Variant, key string) (uint32, bool) {
	value, ok := results[key]
	if !ok {
		return 0, false
	}
	number, ok := value.Value().(uint32)
	return number, ok
}
