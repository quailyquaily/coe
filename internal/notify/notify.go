package notify

import (
	"context"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	busName       = "org.freedesktop.Notifications"
	objectPath    = "/org/freedesktop/Notifications"
	interfaceName = "org.freedesktop.Notifications"
	notifyMethod  = interfaceName + ".Notify"
	closeMethod   = interfaceName + ".CloseNotification"
)

const (
	UrgencyLow byte = iota
	UrgencyNormal
	UrgencyCritical
)

type Service interface {
	Summary() string
	Send(context.Context, Message) error
	Close() error
}

type Message struct {
	Title     string
	Body      string
	Urgency   byte
	Timeout   time.Duration
	Transient bool
}

type Disabled struct{}

func (Disabled) Summary() string {
	return "disabled"
}

func (Disabled) Send(context.Context, Message) error {
	return nil
}

func (Disabled) Close() error {
	return nil
}

type Desktop struct {
	conn    *dbus.Conn
	obj     dbus.BusObject
	appName string
}

func ConnectSession(appName string) (*Desktop, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}

	return &Desktop{
		conn:    conn,
		obj:     conn.Object(busName, dbus.ObjectPath(objectPath)),
		appName: appName,
	}, nil
}

func (d *Desktop) Summary() string {
	return "system (org.freedesktop.Notifications)"
}

func (d *Desktop) Send(ctx context.Context, msg Message) error {
	if d == nil || d.obj == nil || msg.Title == "" {
		return nil
	}

	timeout := int32(5000)
	if msg.Timeout > 0 {
		timeout = int32(msg.Timeout / time.Millisecond)
	}

	hints := buildHints(msg)

	var id uint32
	call := d.obj.CallWithContext(
		ctx,
		notifyMethod,
		0,
		d.appName,
		uint32(0),
		"",
		msg.Title,
		msg.Body,
		[]string{},
		hints,
		timeout,
	)
	if call.Err != nil {
		return call.Err
	}
	if err := call.Store(&id); err != nil {
		return err
	}
	if shouldCloseAfterTimeout(msg) && id != 0 {
		scheduleCloseNotification(id, msg.Timeout)
	}
	return nil
}

func buildHints(msg Message) map[string]dbus.Variant {
	hints := map[string]dbus.Variant{
		"urgency": dbus.MakeVariant(msg.Urgency),
	}
	if msg.Transient {
		hints["transient"] = dbus.MakeVariant(true)
	}
	return hints
}

func shouldCloseAfterTimeout(msg Message) bool {
	return msg.Transient && msg.Timeout > 0 && msg.Urgency != UrgencyCritical
}

func scheduleCloseNotification(id uint32, delay time.Duration) {
	time.AfterFunc(delay, func() {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			return
		}
		defer conn.Close()

		_ = conn.Object(busName, dbus.ObjectPath(objectPath)).Call(
			closeMethod,
			0,
			id,
		).Err
	})
}

func (d *Desktop) Close() error {
	if d == nil || d.conn == nil {
		return nil
	}
	return d.conn.Close()
}
