package portal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/godbus/dbus/v5"
)

const (
	RemoteDesktopCreateSessionMethod         = RemoteDesktopInterface + ".CreateSession"
	RemoteDesktopSelectDevicesMethod         = RemoteDesktopInterface + ".SelectDevices"
	RemoteDesktopStartMethod                 = RemoteDesktopInterface + ".Start"
	RemoteDesktopNotifyKeyboardKeysym        = RemoteDesktopInterface + ".NotifyKeyboardKeysym"
	ClipboardRequestClipboardMethod          = ClipboardInterface + ".RequestClipboard"
	ClipboardSetSelectionMethod              = ClipboardInterface + ".SetSelection"
	ClipboardSelectionWriteMethod            = ClipboardInterface + ".SelectionWrite"
	ClipboardSelectionWriteDoneMethod        = ClipboardInterface + ".SelectionWriteDone"
	ClipboardSelectionTransferSignal         = ClipboardInterface + ".SelectionTransfer"
	keyboardDeviceType                uint32 = 1
	keyStateReleased                  uint32 = 0
	keyStatePressed                   uint32 = 1
	leftCtrlKeysym                           = 0xffe3
	vKeysym                                  = 0x0076
)

type RemoteDesktopOutputSession struct {
	client *Client

	sessionHandle    dbus.ObjectPath
	keyboardGranted  bool
	clipboardEnabled bool

	signalCh   chan *dbus.Signal
	closedCh   chan struct{}
	closeOnce  sync.Once
	clipboardM sync.RWMutex
	clipboard  string
}

func (c *Client) CreateRemoteDesktopOutputSession(ctx context.Context, wantClipboard, wantKeyboard bool) (*RemoteDesktopOutputSession, error) {
	uniqueName, err := c.uniqueName()
	if err != nil {
		return nil, err
	}

	sessionHandle, err := c.createRemoteDesktopSession(ctx, uniqueName)
	if err != nil {
		return nil, err
	}

	if wantKeyboard {
		if err := c.selectRemoteDesktopDevices(ctx, uniqueName, sessionHandle, keyboardDeviceType); err != nil {
			return nil, errors.Join(err, c.closeSession(sessionHandle))
		}
	}

	if wantClipboard {
		if err := c.requestClipboard(ctx, sessionHandle); err != nil {
			return nil, errors.Join(err, c.closeSession(sessionHandle))
		}
	}

	devices, clipboardEnabled, err := c.startRemoteDesktop(ctx, uniqueName, sessionHandle)
	if err != nil {
		return nil, errors.Join(err, c.closeSession(sessionHandle))
	}
	if wantKeyboard && devices&keyboardDeviceType == 0 {
		return nil, errors.Join(fmt.Errorf("portal session started without keyboard permission"), c.closeSession(sessionHandle))
	}
	if wantClipboard && !clipboardEnabled {
		return nil, errors.Join(fmt.Errorf("portal session started without clipboard permission"), c.closeSession(sessionHandle))
	}

	session := &RemoteDesktopOutputSession{
		client:           c,
		sessionHandle:    sessionHandle,
		keyboardGranted:  devices&keyboardDeviceType != 0,
		clipboardEnabled: clipboardEnabled,
		closedCh:         make(chan struct{}),
	}

	if wantClipboard {
		match := []dbus.MatchOption{
			dbus.WithMatchSender(DesktopBusName),
			dbus.WithMatchInterface(ClipboardInterface),
			dbus.WithMatchMember("SelectionTransfer"),
		}
		if err := c.conn.AddMatchSignalContext(ctx, match...); err != nil {
			_ = c.closeSession(sessionHandle)
			_ = c.Close()
			return nil, err
		}

		session.signalCh = make(chan *dbus.Signal, 16)
		c.conn.Signal(session.signalCh)

		go session.handleClipboardTransfers(match)
	}

	return session, nil
}

func (s *RemoteDesktopOutputSession) SetClipboard(ctx context.Context, text string) error {
	if !s.clipboardEnabled {
		return fmt.Errorf("portal clipboard is not active")
	}

	s.clipboardM.Lock()
	s.clipboard = text
	s.clipboardM.Unlock()

	options := map[string]dbus.Variant{
		"mime_types": dbus.MakeVariant([]string{"text/plain;charset=utf-8", "text/plain"}),
	}
	return s.client.obj.CallWithContext(ctx, ClipboardSetSelectionMethod, 0, s.sessionHandle, options).Store()
}

func (s *RemoteDesktopOutputSession) SendPaste(ctx context.Context) error {
	if !s.keyboardGranted {
		return fmt.Errorf("portal keyboard access is not active")
	}

	events := []struct {
		Keysym int32
		State  uint32
	}{
		{Keysym: leftCtrlKeysym, State: keyStatePressed},
		{Keysym: vKeysym, State: keyStatePressed},
		{Keysym: vKeysym, State: keyStateReleased},
		{Keysym: leftCtrlKeysym, State: keyStateReleased},
	}

	for _, event := range events {
		if err := s.client.obj.CallWithContext(
			ctx,
			RemoteDesktopNotifyKeyboardKeysym,
			0,
			s.sessionHandle,
			map[string]dbus.Variant{},
			event.Keysym,
			event.State,
		).Store(); err != nil {
			return err
		}
	}

	return nil
}

func (s *RemoteDesktopOutputSession) Close() error {
	var err error
	s.closeOnce.Do(func() {
		close(s.closedCh)
		if s.signalCh != nil {
			s.client.conn.RemoveSignal(s.signalCh)
		}
		err = errors.Join(s.client.closeSession(s.sessionHandle), s.client.Close())
	})
	return err
}

func (s *RemoteDesktopOutputSession) handleClipboardTransfers(match []dbus.MatchOption) {
	defer func() {
		_ = s.client.conn.RemoveMatchSignal(match...)
	}()

	for {
		select {
		case <-s.closedCh:
			return
		case sig, ok := <-s.signalCh:
			if !ok {
				return
			}
			s.handleClipboardTransfer(sig)
		}
	}
}

func (s *RemoteDesktopOutputSession) handleClipboardTransfer(sig *dbus.Signal) {
	if sig == nil || sig.Name != ClipboardSelectionTransferSignal || len(sig.Body) != 3 {
		return
	}

	sessionHandle, ok := sig.Body[0].(dbus.ObjectPath)
	if !ok || sessionHandle != s.sessionHandle {
		return
	}

	mimeType, ok := sig.Body[1].(string)
	if !ok {
		return
	}

	serial, ok := sig.Body[2].(uint32)
	if !ok {
		return
	}

	text, ok := s.clipboardPayload(mimeType)
	if !ok {
		_ = s.selectionWriteDone(serial, false)
		return
	}

	var fd dbus.UnixFD
	if err := s.client.obj.Call(ClipboardSelectionWriteMethod, 0, s.sessionHandle, serial).Store(&fd); err != nil {
		_ = s.selectionWriteDone(serial, false)
		return
	}

	file := os.NewFile(uintptr(fd), "portal-clipboard-selection")
	if file == nil {
		_ = s.selectionWriteDone(serial, false)
		return
	}

	success := true
	if _, err := io.Copy(file, strings.NewReader(text)); err != nil {
		success = false
	}
	if err := file.Close(); err != nil {
		success = false
	}

	_ = s.selectionWriteDone(serial, success)
}

func (s *RemoteDesktopOutputSession) clipboardPayload(mimeType string) (string, bool) {
	switch mimeType {
	case "text/plain", "text/plain;charset=utf-8":
	default:
		return "", false
	}

	s.clipboardM.RLock()
	defer s.clipboardM.RUnlock()
	if s.clipboard == "" {
		return "", false
	}
	return s.clipboard, true
}

func (s *RemoteDesktopOutputSession) selectionWriteDone(serial uint32, success bool) error {
	return s.client.obj.Call(
		ClipboardSelectionWriteDoneMethod,
		0,
		s.sessionHandle,
		serial,
		success,
	).Store()
}

func (c *Client) createRemoteDesktopSession(ctx context.Context, uniqueName string) (dbus.ObjectPath, error) {
	signalCh, cleanup, err := c.watchRequestResponses(ctx, uniqueName)
	if err != nil {
		return "", err
	}
	defer cleanup()

	handleToken := makeToken("rd_create")
	sessionToken := makeToken("rd_session")
	expectedHandle := requestPath(uniqueName, handleToken)

	options := map[string]dbus.Variant{
		"handle_token":         dbus.MakeVariant(handleToken),
		"session_handle_token": dbus.MakeVariant(sessionToken),
	}

	var handle dbus.ObjectPath
	if err := c.obj.CallWithContext(ctx, RemoteDesktopCreateSessionMethod, 0, options).Store(&handle); err != nil {
		return "", err
	}
	if handle == "" {
		handle = expectedHandle
	}

	response, err := awaitRequestResponse(ctx, signalCh, handle)
	if err != nil {
		return "", err
	}

	sessionHandle, err := variantObjectPath(response.Results, "session_handle")
	if err != nil {
		return "", err
	}
	if sessionHandle == "" {
		sessionHandle = sessionPath(uniqueName, sessionToken)
	}
	return sessionHandle, nil
}

func (c *Client) selectRemoteDesktopDevices(ctx context.Context, uniqueName string, sessionHandle dbus.ObjectPath, deviceTypes uint32) error {
	signalCh, cleanup, err := c.watchRequestResponses(ctx, uniqueName)
	if err != nil {
		return err
	}
	defer cleanup()

	handleToken := makeToken("rd_select")
	var handle dbus.ObjectPath
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
		"types":        dbus.MakeVariant(deviceTypes),
	}
	if err := c.obj.CallWithContext(ctx, RemoteDesktopSelectDevicesMethod, 0, sessionHandle, options).Store(&handle); err != nil {
		return err
	}
	if handle == "" {
		handle = requestPath(uniqueName, handleToken)
	}

	_, err = awaitRequestResponse(ctx, signalCh, handle)
	return err
}

func (c *Client) requestClipboard(ctx context.Context, sessionHandle dbus.ObjectPath) error {
	return c.obj.CallWithContext(ctx, ClipboardRequestClipboardMethod, 0, sessionHandle, map[string]dbus.Variant{}).Store()
}

func (c *Client) startRemoteDesktop(ctx context.Context, uniqueName string, sessionHandle dbus.ObjectPath) (uint32, bool, error) {
	signalCh, cleanup, err := c.watchRequestResponses(ctx, uniqueName)
	if err != nil {
		return 0, false, err
	}
	defer cleanup()

	handleToken := makeToken("rd_start")
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
	}

	var handle dbus.ObjectPath
	if err := c.obj.CallWithContext(ctx, RemoteDesktopStartMethod, 0, sessionHandle, "", options).Store(&handle); err != nil {
		return 0, false, err
	}
	if handle == "" {
		handle = requestPath(uniqueName, handleToken)
	}

	response, err := awaitRequestResponse(ctx, signalCh, handle)
	if err != nil {
		return 0, false, err
	}

	devices, _ := variantUint32(response.Results, "devices")
	clipboardEnabled, _ := variantBool(response.Results, "clipboard_enabled")
	return devices, clipboardEnabled, nil
}

func (c *Client) closeSession(sessionHandle dbus.ObjectPath) error {
	if sessionHandle == "" {
		return nil
	}
	return c.conn.Object(DesktopBusName, sessionHandle).Call(SessionCloseMethod, 0).Store()
}
