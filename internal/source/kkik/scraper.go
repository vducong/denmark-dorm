package kkik

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"housing-waitlist/internal/config"
)

// scraper logs into KKIK and fetches the housing requests page HTML.
type scraper struct {
	cfg config.SourceSettings
}

// fetchHTML performs login and returns the housing page HTML.
func (s *scraper) fetchHTML(ctx context.Context) (string, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", s.cfg.Headless),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
	)

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()

	runCtx, cancelRun := context.WithTimeout(browserCtx, s.cfg.Timeout())
	defer cancelRun()

	var html string
	// Login and housing are separate Run calls so the housing Navigate does not
	// abort the ASP.NET login postback (net::ERR_ABORTED).
	err := chromedp.Run(runCtx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(selLoginUser, chromedp.ByQuery),
		chromedp.SendKeys(selLoginUser, s.cfg.Login.Email, chromedp.ByQuery),
		chromedp.SendKeys(selLoginPassword, s.cfg.Login.Password, chromedp.ByQuery),
		chromedp.Click(selLoginSubmit, chromedp.ByQuery),
	)
	if err != nil {
		return s.fail(runCtx, err)
	}
	if err := chromedp.Run(runCtx, waitLoginPostback()); err != nil {
		return s.fail(runCtx, err)
	}
	err = chromedp.Run(runCtx,
		chromedp.Navigate(housingURL),
		chromedp.WaitVisible(selHousingMarker, chromedp.ByQuery),
		chromedp.OuterHTML("html", &html, chromedp.ByQuery),
	)
	if err != nil {
		return s.fail(runCtx, err)
	}

	if !strings.Contains(html, "func-housingrequests") {
		if dumpErr := s.dumpDebug(runCtx, "missing-grid"); dumpErr != nil {
			return "", fmt.Errorf("housing page missing expected content (debug dump: %v)", dumpErr)
		}
		return "", fmt.Errorf("housing page missing expected content")
	}

	return html, nil
}

func (s *scraper) fail(ctx context.Context, err error) (string, error) {
	if dumpErr := s.dumpDebug(ctx, "failure"); dumpErr != nil {
		return "", fmt.Errorf("scrape: %w (debug dump: %v)", err, dumpErr)
	}
	return "", fmt.Errorf("scrape: %w", err)
}

// waitLoginPostback gives the login form POST time to finish before another Navigate.
func waitLoginPostback() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		if err := chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx); err != nil {
			return err
		}
		time.Sleep(2 * time.Second)
		return nil
	})
}

func (s *scraper) dumpDebug(ctx context.Context, tag string) error {
	if s.cfg.DebugDir == "" {
		return nil
	}
	if err := os.MkdirAll(s.cfg.DebugDir, 0o755); err != nil {
		return err
	}

	ts := time.Now().Format("20060102-150405")
	base := filepath.Join(s.cfg.DebugDir, fmt.Sprintf("%s-%s", tag, ts))

	var html string
	if err := chromedp.Run(ctx, chromedp.OuterHTML("html", &html, chromedp.ByQuery)); err == nil {
		_ = os.WriteFile(base+".html", []byte(html), 0o644)
	}

	var png []byte
	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&png, 90)); err == nil {
		_ = os.WriteFile(base+".png", png, 0o644)
	}

	return nil
}
