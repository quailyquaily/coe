package output

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"coe/internal/platform/portal"
)

type PortalSession interface {
	SetClipboard(context.Context, string) error
	SendPaste(context.Context) error
	Close() error
}

type PortalFactory func(context.Context, PortalRequest) (PortalSession, error)

type PortalRequest struct {
	Clipboard bool
	Paste     bool
}

type Coordinator struct {
	ClipboardPlan      string
	PastePlan          string
	ClipboardBinary    string
	PasteBinary        string
	EnableAutoPaste    bool
	UsePortalClipboard bool
	UsePortalPaste     bool
	PortalFactory      PortalFactory

	portalMu sync.Mutex
	portal   PortalSession
}

type Delivery struct {
	ClipboardWritten bool
	ClipboardMethod  string
	PasteExecuted    bool
	PasteMethod      string
	PasteWarning     string
}

func (c *Coordinator) Summary() string {
	if c == nil {
		return "disabled"
	}
	return fmt.Sprintf("clipboard=%s, paste=%s", c.ClipboardPlan, c.PastePlan)
}

func (c *Coordinator) Deliver(ctx context.Context, text string) (Delivery, error) {
	result := Delivery{}
	if c == nil || text == "" {
		return result, nil
	}

	if err := c.writeClipboard(ctx, text, &result); err != nil {
		return result, err
	}

	if err := c.autoPaste(ctx, &result); err != nil {
		return result, err
	}

	return result, nil
}

func (c *Coordinator) Close() error {
	if c == nil {
		return nil
	}

	c.portalMu.Lock()
	defer c.portalMu.Unlock()

	if c.portal == nil {
		return nil
	}

	err := c.portal.Close()
	c.portal = nil
	return err
}

func (c *Coordinator) writeClipboard(ctx context.Context, text string, result *Delivery) error {
	var portalErr error
	if c.UsePortalClipboard {
		session, err := c.ensurePortal(ctx)
		if err != nil {
			portalErr = fmt.Errorf("portal clipboard session failed: %w", err)
		} else if err := session.SetClipboard(ctx, text); err != nil {
			portalErr = fmt.Errorf("portal clipboard write failed: %w", err)
		} else {
			result.ClipboardWritten = true
			result.ClipboardMethod = "portal"
			return nil
		}
	}

	if c.ClipboardBinary != "" {
		cmd := exec.CommandContext(ctx, c.ClipboardBinary)
		cmd.Stdin = strings.NewReader(text)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("clipboard command failed: %w (%s)", err, strings.TrimSpace(string(output)))
		}

		result.ClipboardWritten = true
		result.ClipboardMethod = filepath.Base(c.ClipboardBinary)
		return nil
	}

	if portalErr != nil {
		return portalErr
	}
	return fmt.Errorf("clipboard output is not configured")
}

func (c *Coordinator) autoPaste(ctx context.Context, result *Delivery) error {
	if !c.EnableAutoPaste {
		return nil
	}

	var portalErr error
	if c.UsePortalPaste {
		session, err := c.ensurePortal(ctx)
		if err != nil {
			portalErr = fmt.Errorf("portal paste session failed: %w", err)
		} else if err := session.SendPaste(ctx); err != nil {
			portalErr = fmt.Errorf("portal paste failed: %w", err)
		} else {
			result.PasteExecuted = true
			result.PasteMethod = "portal"
			return nil
		}
	}

	if c.PasteBinary == "" {
		if portalErr != nil {
			result.PasteWarning = portalErr.Error()
		}
		return nil
	}

	switch filepath.Base(c.PasteBinary) {
	case "ydotool":
		cmd := exec.CommandContext(ctx, c.PasteBinary, "key", "29:1", "47:1", "47:0", "29:0")
		if output, err := cmd.CombinedOutput(); err != nil {
			result.PasteWarning = fmt.Sprintf("ydotool paste failed: %v (%s)", err, strings.TrimSpace(string(output)))
			return nil
		}
		result.PasteExecuted = true
		result.PasteMethod = "ydotool"
		return nil
	default:
		if portalErr != nil {
			result.PasteWarning = portalErr.Error()
		}
		return nil
	}
}

func (c *Coordinator) ensurePortal(ctx context.Context) (PortalSession, error) {
	c.portalMu.Lock()
	defer c.portalMu.Unlock()

	if c.portal != nil {
		return c.portal, nil
	}

	factory := c.PortalFactory
	if factory == nil {
		factory = defaultPortalFactory
	}

	session, err := factory(ctx, PortalRequest{
		Clipboard: c.UsePortalClipboard,
		Paste:     c.EnableAutoPaste && c.UsePortalPaste,
	})
	if err != nil {
		return nil, err
	}

	c.portal = session
	return c.portal, nil
}

func defaultPortalFactory(ctx context.Context, request PortalRequest) (PortalSession, error) {
	client, err := portal.ConnectSession()
	if err != nil {
		return nil, err
	}

	session, err := client.CreateRemoteDesktopOutputSession(ctx, request.Clipboard, request.Paste)
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return session, nil
}
