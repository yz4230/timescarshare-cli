// Package session manages the Times Car Share login session: a browser-driven
// login that captures the site cookies, and the on-disk store that later
// commands reuse to authenticate plain HTTP API calls.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// loginURL is the member page; when not logged in the site redirects it to the
// tpLogin.jsp form.
const loginURL = "https://share.timescar.jp/view/member/mypage.jsp"

// successMarker appears in the URL once the login redirect completes.
const successMarker = "/view/member/mypage.jsp"

// cookieURLs are the origins whose cookies make up the session.
var cookieURLs = []string{"https://share.timescar.jp/", "https://api.timesclub.jp/"}

// Cookie is a serializable subset of an HTTP cookie.
type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HTTPOnly bool   `json:"http_only"`
}

// Session is the persisted login state.
type Session struct {
	SavedAt time.Time `json:"saved_at"`
	Cookies []Cookie  `json:"cookies"`
}

// ConfigDir returns the per-user config directory, creating it if needed.
func ConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "timescarshare")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// Path returns the session file path.
func Path() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "session.json"), nil
}

// Load reads the persisted session. It returns fs.ErrNotExist wrapped when no
// session has been saved yet.
func Load() (*Session, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(s.Cookies) == 0 {
		return nil, fmt.Errorf("session %s has no cookies; run `login` again", path)
	}
	return &s, nil
}

// Save writes the session to disk with 0600 permissions.
func (s *Session) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o600)
}

// CookieHeader renders the session cookies as an HTTP Cookie header value.
func (s *Session) CookieHeader() string {
	parts := make([]string, 0, len(s.Cookies))
	for _, c := range s.Cookies {
		parts = append(parts, c.Name+"="+c.Value)
	}
	return strings.Join(parts, "; ")
}

// ErrTimeout is returned when the user does not finish logging in in time.
var ErrTimeout = errors.New("timed out waiting for login")

// Login opens a headed Chrome window at the login page and waits until the user
// has signed in, then saves the resulting cookies.
func Login(ctx context.Context) error {
	profile, err := ConfigDir()
	if err != nil {
		return err
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserDataDir(filepath.Join(profile, "chrome-profile")),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	if err := chromedp.Do(browserCtx, chromedp.Navigate(loginURL)); err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	prefill(browserCtx)

	deadline := time.Now().Add(10 * time.Minute)
	for {
		loc, err := chromedp.Run(browserCtx, chromedp.Location())
		if err != nil {
			return fmt.Errorf("browser closed before login completed: %w", err)
		}
		if strings.Contains(loc, successMarker) {
			break
		}
		if time.Now().After(deadline) {
			return ErrTimeout
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}

	res, err := chromedp.Run(browserCtx, func(ctx context.Context, t *chromedp.Target) (network.GetCookiesResult, error) {
		return cdp.Call(ctx, t, network.GetCookies, network.GetCookiesParams{URLs: cookieURLs})
	})
	if err != nil {
		return fmt.Errorf("read cookies: %w", err)
	}
	s := &Session{SavedAt: time.Now()}
	for _, c := range res.Cookies {
		s.Cookies = append(s.Cookies, Cookie{
			Name: c.Name, Value: c.Value, Domain: c.Domain, Path: c.Path,
			Secure: c.Secure, HTTPOnly: c.HTTPOnly,
		})
	}
	if len(s.Cookies) == 0 {
		return errors.New("login succeeded but no cookies were captured")
	}
	if err := s.Save(); err != nil {
		return err
	}
	path, _ := Path()
	fmt.Fprintf(os.Stderr, "login saved to %s (%d cookies)\n", path, len(s.Cookies))
	return nil
}

// prefill types the member number and password from the environment into the
// login form, so the user only has to solve the reCAPTCHA and click log in.
// It is best-effort: if the form is absent (already logged in) or the
// variables are unset, it does nothing.
func prefill(ctx context.Context) {
	member := digitsOnly(os.Getenv("TIMESCAR_MEMBER_NO"))
	pass := os.Getenv("TIMESCAR_PASSWORD")
	if len(member) < 10 || pass == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err := chromedp.Do(ctx,
		chromedp.WaitReady(chromedp.CSS("#tpPassword")),
		chromedp.SetValue(chromedp.CSS("#cardNo1"), member[:4]),
		chromedp.SetValue(chromedp.CSS("#cardNo2"), member[4:10]),
		chromedp.SetValue(chromedp.CSS("#tpPassword"), pass),
	)
	if err != nil {
		slog.Debug("prefill login form skipped", "error", err)
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
