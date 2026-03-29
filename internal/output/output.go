package output

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"coe/internal/focus"
	"coe/internal/platform/portal"
)

type PortalSession interface {
	SetClipboard(context.Context, string) error
	SendPaste(context.Context, string) error
	Close() error
	RestoreToken() string
}

type PortalFactory func(context.Context, PortalRequest) (PortalSession, error)

type PortalRequest struct {
	Clipboard    bool
	Paste        bool
	Persist      bool
	RestoreToken string
}

type Coordinator struct {
	ClipboardPlan         string
	PastePlan             string
	ClipboardBinary       string
	PasteBinary           string
	EnableAutoPaste       bool
	PasteShortcut         string
	TerminalPasteShortcut string
	UsePortalClipboard    bool
	UsePortalPaste        bool
	PersistPortalAccess   bool
	FocusProvider         focus.Provider
	PortalFactory         PortalFactory
	PortalStateStore      *PortalStateStore

	portalMu sync.Mutex
	portal   PortalSession
}

type Delivery struct {
	ClipboardWritten  bool
	ClipboardMethod   string
	ClipboardWarning  string
	ClipboardDuration time.Duration
	PasteExecuted     bool
	PasteMethod       string
	PasteShortcut     string
	PasteTarget       string
	PasteWarning      string
	PasteDuration     time.Duration
}

const portalPasteDelay = 150 * time.Millisecond

const (
	portalOperationTimeout  = 5 * time.Second
	clipboardCommandTimeout = 2 * time.Second
	pasteCommandTimeout     = 2 * time.Second
)

func (c *Coordinator) Summary() string {
	if c == nil {
		return "disabled"
	}
	return fmt.Sprintf("clipboard=%s, paste=%s", c.ClipboardPlan, c.PastePlan)
}

func (c *Coordinator) Deliver(ctx context.Context, text string) (Delivery, error) {
	return c.DeliverWithTarget(ctx, text, nil)
}

func (c *Coordinator) DeliverWithTarget(ctx context.Context, text string, target *focus.Target) (Delivery, error) {
	result := Delivery{}
	if c == nil || text == "" {
		return result, nil
	}

	if err := c.writeClipboard(ctx, text, &result); err != nil {
		return result, err
	}

	if err := c.autoPaste(ctx, &result, target); err != nil {
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
	startedAt := time.Now()
	defer func() {
		result.ClipboardDuration = time.Since(startedAt)
	}()

	var portalErr error
	if c.UsePortalClipboard {
		session, err := c.ensurePortal(ctx)
		if err != nil {
			portalErr = fmt.Errorf("portal clipboard session failed: %w", err)
		} else if err := c.setPortalClipboard(ctx, session, text); err != nil {
			portalErr = fmt.Errorf("portal clipboard write failed: %w", err)
		} else {
			result.ClipboardWritten = true
			result.ClipboardMethod = "portal"
			return nil
		}
	}

	if c.ClipboardBinary != "" {
		commandCtx, cancel := context.WithTimeout(ctx, clipboardCommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(commandCtx, c.ClipboardBinary)
		cmd.Stdin = strings.NewReader(text)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("clipboard command failed: %w (%s)", err, strings.TrimSpace(string(output)))
		}

		result.ClipboardWritten = true
		result.ClipboardMethod = filepath.Base(c.ClipboardBinary)
		if portalErr != nil {
			result.ClipboardWarning = portalErr.Error()
		}
		return nil
	}

	if portalErr != nil {
		result.ClipboardWarning = portalErr.Error()
		return portalErr
	}
	return fmt.Errorf("clipboard output is not configured")
}

func (c *Coordinator) autoPaste(ctx context.Context, result *Delivery, target *focus.Target) error {
	startedAt := time.Now()
	defer func() {
		result.PasteDuration = time.Since(startedAt)
	}()

	if !c.EnableAutoPaste {
		return nil
	}

	shortcut, targetSummary := c.resolvePasteShortcut(ctx, target)
	result.PasteShortcut = shortcut
	result.PasteTarget = targetSummary

	var portalErr error
	if c.UsePortalPaste {
		session, err := c.ensurePortal(ctx)
		if err != nil {
			portalErr = fmt.Errorf("portal paste session failed: %w", err)
		} else if result.ClipboardMethod == "portal" {
			if err := sleepContext(ctx, portalPasteDelay); err != nil {
				portalErr = fmt.Errorf("portal paste delayed by context cancellation: %w", err)
			}
		}

		if portalErr == nil && session != nil {
			if err := c.sendPortalPaste(ctx, session, shortcut); err != nil {
				portalErr = fmt.Errorf("portal paste failed: %w", err)
			} else {
				result.PasteExecuted = true
				result.PasteMethod = "portal"
				return nil
			}
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
		args, err := ydotoolPasteArgs(shortcut)
		if err != nil {
			result.PasteWarning = err.Error()
			return nil
		}

		commandCtx, cancel := context.WithTimeout(ctx, pasteCommandTimeout)
		defer cancel()

		cmd := exec.CommandContext(commandCtx, c.PasteBinary, args...)
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

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Coordinator) FocusedTarget(ctx context.Context) (focus.Target, error) {
	if c == nil || c.FocusProvider == nil {
		return focus.Target{}, fmt.Errorf("focus provider is unavailable")
	}
	return c.FocusProvider.Focused(ctx)
}

func (c *Coordinator) resolvePasteShortcut(ctx context.Context, target *focus.Target) (string, string) {
	baseShortcut := NormalizePasteShortcut(c.PasteShortcut)
	terminalShortcut := NormalizePasteShortcut(c.TerminalPasteShortcut)
	if terminalShortcut == "" {
		terminalShortcut = "ctrl+shift+v"
	}

	if target == nil {
		focused, err := c.FocusedTarget(ctx)
		if err != nil {
			return baseShortcut, ""
		}
		target = &focused
	}

	if focus.LooksLikeTerminal(*target) {
		return terminalShortcut, target.Summary()
	}
	return baseShortcut, target.Summary()
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
		Clipboard:    c.UsePortalClipboard,
		Paste:        c.EnableAutoPaste && c.UsePortalPaste,
		Persist:      c.PersistPortalAccess,
		RestoreToken: c.loadRestoreToken(),
	})
	if err != nil {
		return nil, err
	}

	c.saveRestoreToken(session.RestoreToken())
	c.portal = session
	return c.portal, nil
}

func (c *Coordinator) setPortalClipboard(ctx context.Context, session PortalSession, text string) error {
	callCtx, cancel := context.WithTimeout(ctx, portalOperationTimeout)
	defer cancel()

	err := session.SetClipboard(callCtx, text)
	if err == nil || !isInvalidPortalSessionError(err) {
		return err
	}

	retried, retryErr := c.resetPortalAndRetry(ctx)
	if retryErr != nil {
		return errors.Join(err, retryErr)
	}

	callCtx, cancel = context.WithTimeout(ctx, portalOperationTimeout)
	defer cancel()
	if retryErr = retried.SetClipboard(callCtx, text); retryErr != nil {
		return errors.Join(err, retryErr)
	}
	return nil
}

func (c *Coordinator) sendPortalPaste(ctx context.Context, session PortalSession, shortcut string) error {
	callCtx, cancel := context.WithTimeout(ctx, portalOperationTimeout)
	defer cancel()

	err := session.SendPaste(callCtx, shortcut)
	if err == nil || !isInvalidPortalSessionError(err) {
		return err
	}

	retried, retryErr := c.resetPortalAndRetry(ctx)
	if retryErr != nil {
		return errors.Join(err, retryErr)
	}

	callCtx, cancel = context.WithTimeout(ctx, portalOperationTimeout)
	defer cancel()
	if retryErr = retried.SendPaste(callCtx, shortcut); retryErr != nil {
		return errors.Join(err, retryErr)
	}
	return nil
}

func (c *Coordinator) resetPortalAndRetry(ctx context.Context) (PortalSession, error) {
	c.portalMu.Lock()
	if c.portal != nil {
		_ = c.portal.Close()
		c.portal = nil
	}
	c.portalMu.Unlock()

	return c.ensurePortal(ctx)
}

func isInvalidPortalSessionError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid session")
}

func defaultPortalFactory(ctx context.Context, request PortalRequest) (PortalSession, error) {
	client, err := portal.ConnectSession()
	if err != nil {
		return nil, err
	}

	session, err := client.CreateRemoteDesktopOutputSession(ctx, portal.OutputSessionOptions{
		WantClipboard: request.Clipboard,
		WantKeyboard:  request.Paste,
		PersistAccess: request.Persist,
		RestoreToken:  request.RestoreToken,
	})
	if err != nil {
		_ = client.Close()
		return nil, err
	}

	return session, nil
}

func (c *Coordinator) loadRestoreToken() string {
	if !c.PersistPortalAccess || c.PortalStateStore == nil {
		return ""
	}

	current, err := c.PortalStateStore.Load()
	if err != nil {
		return ""
	}
	return current.RemoteDesktopRestoreToken
}

func (c *Coordinator) saveRestoreToken(token string) {
	if !c.PersistPortalAccess || c.PortalStateStore == nil || strings.TrimSpace(token) == "" {
		return
	}

	_ = c.PortalStateStore.Save(PortalAccess{
		RemoteDesktopRestoreToken: token,
	})
}
