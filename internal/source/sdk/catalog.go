package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"housing-waitlist/internal/model"

	"github.com/chromedp/chromedp"
)

const searchResultURL = baseURL + "/studiebolig/search-result/"

var buildingIDRe = regexp.MustCompile(`/studiebolig/building/(\d+)/`)

// catalogGroup is one entry from captureAllGroups, representing a room-type
// accordion section on a building detail page.
type catalogGroup struct {
	SizeRent   string `json:"sizeRent"`
	Address    string `json:"address"`
	SampleHref string `json:"sampleHref"`
}

// FetchCatalog logs into s.dk, collects all buildings from the search-result
// page, visits each building's detail page, and returns one WaitlistRow per
// room-type group. Rows are unranked (RankOrder=99, RankDisplay="") since the
// applicant may not have applied for them; scoring treats these as
// desirability-only entries.
func (s *SDK) FetchCatalog(ctx context.Context) (*model.Result, error) {
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

	if err := chromedp.Run(runCtx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(selLoginUser, chromedp.ByQuery),
		dismissCookieConsent(),
		chromedp.SendKeys(selLoginUser, s.cfg.Login.Email, chromedp.ByQuery),
		chromedp.SendKeys(selLoginPassword, s.cfg.Login.Password, chromedp.ByQuery),
		chromedp.Click(selLoginSubmit, chromedp.ByQuery),
		chromedp.WaitReady("body", chromedp.ByQuery),
	); err != nil {
		return nil, fmt.Errorf("catalog: login: %w", err)
	}

	var buildings []building
	if err := chromedp.Run(runCtx,
		chromedp.Sleep(2*time.Second),
		chromedp.Navigate(searchResultURL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		collectCatalogBuildings(&buildings),
	); err != nil {
		return nil, fmt.Errorf("catalog: collect buildings: %w", err)
	}
	if len(buildings) == 0 {
		return nil, fmt.Errorf("catalog: no buildings found on search-result page")
	}
	slog.Info("catalog: found buildings", "source", "sdk", "count", len(buildings))

	sections := make([][]model.WaitlistRow, len(buildings))
	sem := make(chan struct{}, buildingConcurrency)
	var wg sync.WaitGroup
	var done int64
	for i, b := range buildings {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, b building) {
			defer wg.Done()
			defer func() { <-sem }()
			rows, err := fetchCatalogBuildingRows(browserCtx, b)
			if err != nil {
				slog.Warn("catalog: building crawl failed", "source", "sdk", "name", b.Name, "err", err)
				return
			}
			sections[i] = rows
			slog.Info("catalog: crawled building", "source", "sdk",
				"done", atomic.AddInt64(&done, 1), "of", len(buildings), "name", b.Name)
		}(i, b)
	}
	wg.Wait()

	var rows []model.WaitlistRow
	for _, sec := range sections {
		rows = append(rows, sec...)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("catalog: no room groups found across %d buildings", len(buildings))
	}
	return &model.Result{Rows: rows}, nil
}

// collectCatalogBuildings waits for building cards to render on the
// search-result page and collects all unique building links across every
// page. The search-result page paginates results server-side via numbered
// "ub-pagination" links (?page=N), not a same-page "load more" button — each
// page navigation replaces the building list rather than appending to it, so
// links are collected once per page and merged, not accumulated by watching
// the DOM grow.
func collectCatalogBuildings(out *[]building) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		const sel = `a[href*="/studiebolig/building/"]`
		waitJS := `document.querySelectorAll('` + sel + `').length > 0`
		collectJS := `(() => {
			const seen = new Set(); const res = [];
			document.querySelectorAll('` + sel + `').forEach(a => {
				const href = a.getAttribute('href');
				if (!href || seen.has(href)) return;
				seen.add(href);
				const card = a.closest('[class*="card"], article, li, [class*="item"]') || a;
				const nameEl = card.querySelector('h3, h4, h5, .card-title, strong, p:first-child') || a;
				const name = nameEl.textContent.trim().replace(/\s+/g, ' ');
				const cityEl = card.querySelector('.text-muted');
				const city = cityEl ? cityEl.textContent.trim() : '';
				res.push({href, name: name || href, city});
			});
			return JSON.stringify(res);
		})()`
		// The Next link's <li> gains a "disabled" class on the last page.
		nextHrefJS := `(() => {
			const a = document.querySelector('a.ub-page-link[aria-label="Next"]');
			if (!a) return '';
			const li = a.closest('li');
			if (li && li.classList.contains('disabled')) return '';
			return a.getAttribute('href') || '';
		})()`

		seen := make(map[string]bool)
		for page := 0; page < 60; page++ {
			for i := 0; i < 30; i++ {
				var ready bool
				if err := chromedp.Evaluate(waitJS, &ready).Do(ctx); err != nil {
					return err
				}
				if ready {
					break
				}
				if err := chromedp.Sleep(500 * time.Millisecond).Do(ctx); err != nil {
					return err
				}
			}

			var pageJSON string
			if err := chromedp.Evaluate(collectJS, &pageJSON).Do(ctx); err != nil {
				return err
			}
			var pageBuildings []building
			if err := json.Unmarshal([]byte(pageJSON), &pageBuildings); err != nil {
				return fmt.Errorf("parse building links: %w", err)
			}
			for _, b := range pageBuildings {
				if seen[b.URL] {
					continue
				}
				seen[b.URL] = true
				*out = append(*out, b)
			}

			var nextHref string
			if err := chromedp.Evaluate(nextHrefJS, &nextHref).Do(ctx); err != nil {
				return err
			}
			if nextHref == "" {
				break
			}
			if err := chromedp.Navigate(absoluteURL(nextHref)).Do(ctx); err != nil {
				return err
			}
			if err := chromedp.WaitReady("body", chromedp.ByQuery).Do(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

// fetchCatalogBuildingRows opens one building's detail page and returns one
// WaitlistRow per room-type group. Unlike the personal-waitlist path, it does
// not require rank cells — all accordion sections are captured.
func fetchCatalogBuildingRows(browserCtx context.Context, b building) ([]model.WaitlistRow, error) {
	tabCtx, cancelTab := chromedp.NewContext(browserCtx)
	defer cancelTab()
	runCtx, cancelRun := context.WithTimeout(tabCtx, buildingTimeout)
	defer cancelRun()

	var groupsJSON string
	if err := chromedp.Run(runCtx,
		chromedp.Navigate(baseURL+b.URL),
		chromedp.WaitReady("body", chromedp.ByQuery),
		loadCatalogBuilding(),
		captureAllGroups(&groupsJSON),
	); err != nil {
		return nil, err
	}

	var groups []catalogGroup
	if err := json.Unmarshal([]byte(groupsJSON), &groups); err != nil {
		return nil, fmt.Errorf("parse catalog groups for %q: %w", b.Name, err)
	}

	m := buildingIDRe.FindStringSubmatch(b.URL)
	bID := ""
	if len(m) > 1 {
		bID = m[1]
	}

	var rows []model.WaitlistRow
	for _, g := range groups {
		rg := rentGroupRe.FindStringSubmatch(g.SizeRent)
		if rg == nil {
			continue
		}
		sMin := atoi(rg[1])
		sMax := atoiOr(rg[2], rg[1])
		rMin := atoi(rg[3])
		rMax := atoiOr(rg[4], rg[3])

		sizeStr := strconv.Itoa(sMin)
		if sMax != sMin {
			sizeStr = strconv.Itoa(sMin) + "-" + strconv.Itoa(sMax)
		}

		rows = append(rows, model.WaitlistRow{
			RequestID:   bID + "_" + sizeStr,
			Dorm:        b.Name,
			URL:         absoluteURL(b.URL),
			RoomType:    sizeStr + " m²",
			Size:        sizeStr,
			RankDisplay: "",
			RankOrder:   99,
			Address:     g.Address,
			RentMin:     rMin,
			RentMax:     rMax,
		})
	}
	return rows, nil
}

// loadCatalogBuilding waits for the building detail page to finish loading and
// expands all collapsed accordion panels. Unlike loadBuildingRankings, it does
// not wait for rank cells — catalog pages may have none when the applicant is
// not on the waitlist for that building.
func loadCatalogBuilding() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		loadedJS := `(() => {
			const ready = [...document.querySelectorAll('.spinner')].every(s => s.classList.contains('-paused'));
			const present = !!document.querySelector('` + selAccordionToggle + `') || !!document.querySelector('.card-header');
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
		return chromedp.Evaluate(`(() => {
			let n = 0;
			document.querySelectorAll('`+selAccordionToggle+`[aria-expanded="false"]').forEach(a => { a.click(); n++; });
			return n;
		})()`, new(int)).Do(ctx)
	})
}

// captureAllGroups returns a JSON array of catalogGroup objects — one per
// room-type accordion card on the building page. Each entry carries the card
// header's size+rent band string and a sample address drawn from the first
// tenancy link inside the card (falling back to a page-level address).
func captureAllGroups(out *string) chromedp.Action {
	js := `(() => {
		const buildingAddr = (() => {
			for (const sel of ['address', '.building-address', 'h1 ~ .text-muted', '.property-address']) {
				const el = document.querySelector(sel);
				if (el) return el.textContent.trim().replace(/\s+/g, ' ');
			}
			return '';
		})();
		const groups = [];
		document.querySelectorAll('.card').forEach(card => {
			const muted = card.querySelector('.card-header .text-muted');
			if (!muted) return;
			const sizeRent = muted.textContent.trim().replace(/\s+/g, ' ');
			if (!sizeRent || !/\d+.*kr/.test(sizeRent)) return;
			const tenancyA = card.querySelector('a[href*="/studiebolig/tenancy/"]');
			const address = tenancyA
				? tenancyA.textContent.trim().replace(/\s+/g, ' ')
				: buildingAddr;
			const sampleHref = tenancyA ? (tenancyA.getAttribute('href') || '') : '';
			groups.push({sizeRent, address, sampleHref});
		});
		return JSON.stringify(groups);
	})()`
	return chromedp.Evaluate(js, out)
}
