package sdk

import (
	"context"
	"fmt"
	"html"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"housing-waitlist/internal/config"

	"github.com/chromedp/chromedp"
)

const (
	// buildingConcurrency caps how many building pages
	// are crawled in parallel tabs of the shared browser.
	buildingConcurrency = 6
	// buildingTimeout bounds a single building-page crawl
	// so one stuck page cannot hold a worker indefinitely.
	buildingTimeout = 90 * time.Second
)

// scraper logs into s.dk and crawls the studiebolig waitlist.
type scraper struct {
	cfg config.SourceSettings
}

// building is one property the applicant is signed up for, taken from the home list.
// Its detail page holds the per-tenancy waiting-list rankings.
type building struct {
	URL  string `json:"href"`
	Name string `json:"name"`
	City string `json:"city"`
}

// fetchHTML logs in, lists every building the applicant is queued for, then
// visits each building's detail page and collects its ranking tables.
//
// s.dk shows no rank on the home list; the per-tenancy "Ranking on the waiting list" letter
// lives only on each building page. So the crawl fans out one detail-page visit per building
// and stitches the ranking tables into a single document that the parser walks section by section.
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

	err := chromedp.Run(runCtx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(selLoginUser, chromedp.ByQuery),
		dismissCookieConsent(),
		chromedp.SendKeys(selLoginUser, s.cfg.Login.Email, chromedp.ByQuery),
		chromedp.SendKeys(selLoginPassword, s.cfg.Login.Password, chromedp.ByQuery),
		chromedp.Click(selLoginSubmit, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
	)
	if err != nil {
		return s.fail(runCtx, err)
	}

	// List every building the applicant is signed up for.
	var buildings []building
	err = chromedp.Run(runCtx,
		chromedp.Sleep(2*time.Second),
		chromedp.Navigate(listURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		chromedp.WaitVisible(selAccordionToggle, chromedp.ByQuery),
		expandAllProperties(),
		collectBuildings(&buildings),
	)
	if err != nil {
		return s.fail(runCtx, err)
	}
	if len(buildings) == 0 {
		return s.fail(runCtx, fmt.Errorf("no buildings found on list page"))
	}

	// Crawl every building page concurrently — each in its own tab sharing the
	// logged-in session — then stitch the ranking tables into one document in
	// building order.
	sections := make([]string, len(buildings))
	sem := make(chan struct{}, buildingConcurrency)
	var wg sync.WaitGroup
	var done int64
	for i, b := range buildings {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, b building) {
			defer wg.Done()
			defer func() { <-sem }()

			tables, err := fetchBuildingTables(browserCtx, b)
			if err != nil {
				slog.Warn("building crawl failed", "source", "sdk", "name", b.Name, "err", err)
			}
			sections[i] = fmt.Sprintf(
				`<section class="sdk-building" data-name="%s" data-city="%s" data-url="%s">%s</section>`,
				html.EscapeString(b.Name), html.EscapeString(b.City), html.EscapeString(b.URL), tables,
			)
			slog.Info("crawled building", "source", "sdk", "done", atomic.AddInt64(&done, 1), "of", len(buildings), "name", b.Name)
		}(i, b)
	}
	wg.Wait()

	var sb strings.Builder
	sb.WriteString("<!doctype html><html><body>")
	for _, sec := range sections {
		sb.WriteString(sec)
	}
	sb.WriteString("</body></html>")

	return sb.String(), nil
}

// fetchBuildingTables opens one building page in its own tab and returns the
// HTML of its ranking tables. Some buildings render their ranking labels on a
// delayed pass, so it re-captures a few times while they are still empty.
func fetchBuildingTables(browserCtx context.Context, b building) (string, error) {
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	defer cancelTab()
	runCtx, cancelRun := context.WithTimeout(tabCtx, buildingTimeout)
	defer cancelRun()

	var tables, rentGroups string
	if err := chromedp.Run(runCtx,
		chromedp.Navigate(baseURL+b.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		loadBuildingRankings(),
		captureRankingTables(&tables),
		captureRentGroups(&rentGroups),
	); err != nil {
		return "", err
	}
	return tables + rentGroups, nil
}

// dismissCookieConsent clicks the cookie-accept banner if present so it cannot
// intercept clicks on the login form. A missing banner is not an error.
func dismissCookieConsent() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var clicked bool
		js := `(() => { const b = document.querySelector('` + selCookieAccept + `'); if (b) { b.click(); return true; } return false; })()`
		return chromedp.Evaluate(js, &clicked).Do(ctx)
	})
}

// expandAllProperties opens every collapsed org accordion so its property
// list-group lazy-loads, then clicks each "Vis flere ejendomme" button until the
// full list is loaded.
//
// The list is a Vue SPA: items are fetched only after the accordion expands, and
// only once the page's initial load has finished wiring its handlers. So it first
// waits for the loading spinner to pause, expands, then polls until the unique
// building count stops growing and no load-more buttons remain.
func expandAllProperties() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		// Wait until the applicationlist's initial load finishes (spinner paused),
		// so Vue's expand handler is wired before we click.
		loadedJS := `(() => {
			const s = document.querySelector('article.applicationlist .spinner');
			return !s || s.classList.contains('-paused');
		})()`
		for i := 0; i < 20; i++ {
			var loaded bool
			if err := chromedp.Evaluate(loadedJS, &loaded).Do(ctx); err != nil {
				return err
			}
			if loaded {
				break
			}
			if err := chromedp.Sleep(500 * time.Millisecond).Do(ctx); err != nil {
				return err
			}
		}

		// Expand any still-collapsed org accordion so its list-group renders.
		var expanded int
		if err := chromedp.Evaluate(`(() => {
			let n = 0;
			document.querySelectorAll('`+selAccordionToggle+`').forEach(a => {
				if (a.getAttribute('aria-expanded') !== 'true') { a.click(); n++; }
			});
			return n;
		})()`, &expanded).Do(ctx); err != nil {
			return err
		}

		// Page through the list: the list-group shows ~10 at a time behind a "Vis
		// flere ejendomme" button. Each click loads a batch (an "Opdaterer …"
		// spinner shows), so click once, wait for the distinct count to grow, and
		// repeat until the button is gone.
		countJS := `new Set([...document.querySelectorAll('` + selBuildingLink + `')].map(a => a.getAttribute('href'))).size`

		// Wait for the first batch to render after expanding the accordion.
		for i := 0; i < 30; i++ {
			var n int
			if err := chromedp.Evaluate(countJS, &n).Do(ctx); err != nil {
				return err
			}
			if n > 0 {
				break
			}
			if err := chromedp.Sleep(400 * time.Millisecond).Do(ctx); err != nil {
				return err
			}
		}

		clickMoreJS := `(() => {
			const b = [...document.querySelectorAll('button, a')].find(
				el => (el.textContent || '').trim().toLowerCase() === '` + txtShowMore + `');
			if (!b) return false;
			b.click();
			return true;
		})()`
		for page := 0; page < 20; page++ {
			var prev int
			if err := chromedp.Evaluate(countJS, &prev).Do(ctx); err != nil {
				return err
			}
			var clicked bool
			if err := chromedp.Evaluate(clickMoreJS, &clicked).Do(ctx); err != nil {
				return err
			}
			if !clicked {
				break // no more "Vis flere ejendomme" button — list fully loaded
			}
			// Wait for this batch to finish loading (the distinct count grows).
			for w := 0; w < 20; w++ {
				if err := chromedp.Sleep(300 * time.Millisecond).Do(ctx); err != nil {
					return err
				}
				var cur int
				if err := chromedp.Evaluate(countJS, &cur).Do(ctx); err != nil {
					return err
				}
				if cur > prev {
					break
				}
			}
		}
		return nil
	})
}

// collectBuildings reads the distinct buildings from the expanded list-group.
func collectBuildings(out *[]building) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		js := `(() => {
			const seen = new Set(); const res = [];
			document.querySelectorAll('` + selBuildingLink + `').forEach(a => {
				const href = a.getAttribute('href');
				if (!href || seen.has(href)) return;
				seen.add(href);
				const cityEl = a.querySelector('.text-muted');
				const city = cityEl ? cityEl.textContent.trim() : '';
				let name = (a.querySelector('p') || a).textContent;
				if (city) name = name.replace(city, '');
				res.push({href: href, name: name.trim(), city: city});
			});
			return res;
		})()`
		return chromedp.Evaluate(js, out).Do(ctx)
	})
}

// loadBuildingRankings waits for the building page to finish loading, expands
// any collapsed residence-group accordion, then waits for the ranking labels.
//
// A building page lists its residence groups as accordions; some render
// collapsed, with the tenancy table rendered lazily on expand. The expand
// handler is only wired once the SPA's initial load finishes, so it first waits
// for the loading spinners to pause (and the groups to exist) before clicking,
// then waits for the signed-up tenancy rows to render.
func loadBuildingRankings() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		loadedJS := `(() => {
			const ready = [...document.querySelectorAll('.spinner')].every(s => s.classList.contains('-paused'));
			const present = !!document.querySelector('` + selAccordionToggle + `') || !!document.querySelector('` + selRankCell + `');
			return ready && present;
		})()`
		for i := 0; i < 30; i++ {
			var loaded bool
			if err := chromedp.Evaluate(loadedJS, &loaded).Do(ctx); err != nil {
				return err
			}
			if loaded {
				break
			}
			if err := chromedp.Sleep(400 * time.Millisecond).Do(ctx); err != nil {
				return err
			}
		}

		// Expand any collapsed group so its tenancy table renders.
		if err := chromedp.Evaluate(`(() => {
			let n = 0;
			document.querySelectorAll('`+selAccordionToggle+`[aria-expanded="false"]').forEach(a => { a.click(); n++; });
			return n;
		})()`, new(int)).Do(ctx); err != nil {
			return err
		}

		// Wait for the signed-up tenancy rows to render (ranked or "Not set").
		rowsJS := `document.querySelectorAll('` + selRankCell + `').length`
		for i := 0; i < 16; i++ {
			var n int
			if err := chromedp.Evaluate(rowsJS, &n).Do(ctx); err != nil {
				return err
			}
			if n > 0 {
				return chromedp.Sleep(400 * time.Millisecond).Do(ctx)
			}
			if err := chromedp.Sleep(500 * time.Millisecond).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// captureRankingTables returns the outer HTML of every table on the building
// page that carries the applicant's signed-up tenancies (ranked or "Not set").
func captureRankingTables(out *string) chromedp.Action {
	js := `[...document.querySelectorAll('table')]
		.filter(t => t.querySelector('` + selRankCell + `'))
		.map(t => t.outerHTML).join('')`
	return chromedp.Evaluate(js, out)
}

// captureRentGroups returns a <div class="sdk-rent-groups"> holding one span per
// room-type group on the building page, each carrying that group's size and
// monthly-rent band (e.g. "37-37 m2, 2588-2875 kr.") read from its accordion
// header. s.dk shows rent only per group, so the parser matches a tenancy to its
// group by size. Best-effort: a building with no rent headers yields an empty
// div and the rows simply keep unknown rent.
func captureRentGroups(out *string) chromedp.Action {
	js := `'<div class="sdk-rent-groups">' + [...document.querySelectorAll('.card-header .text-muted')]
		.map(e => (e.textContent || '').trim())
		.filter(t => /kr\.?/.test(t))
		.map(t => '<span>' + t.replace(/[<>]/g, '') + '</span>')
		.join('') + '</div>'`
	return chromedp.Evaluate(js, out)
}

func (s *scraper) fail(ctx context.Context, err error) (string, error) {
	if dumpErr := s.dumpDebug(ctx, "failure"); dumpErr != nil {
		return "", fmt.Errorf("scrape: %w (debug dump: %v)", err, dumpErr)
	}
	return "", fmt.Errorf("scrape: %w", err)
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
