package kkik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"housing-waitlist/internal/model"

	"github.com/PuerkitoBio/goquery"
)

// kollegiumListURL is KKIK's PUBLIC catalog of every dorm and room type, listing
// each room type's monthly rent and size. It needs no login, so a plain GET (as
// in address.go) suffices, and one fetch covers every dorm.
const kollegiumListURL = "https://www.kollegierneskontor.dk/default.aspx?func=kkikportal.kollegiumlist&mid=40&topmenuid=34&lang=GB"

var aridRe = regexp.MustCompile(`arid=(\d+)`)

// EnrichRent fills RentMin/RentMax on each row from the public catalog, joined
// by the room type's arid (carried in the row's detail-page URL). KKIK shows a
// single rent per room type, so RentMin and RentMax are set equal.
func (k *KKIK) EnrichRent(ctx context.Context, rows []model.WaitlistRow) error {
	body, err := fetchKollegiumList(ctx)
	if err != nil {
		return err
	}
	rents := parseRentTable(body)
	for i := range rows {
		if arid := aridFromURL(rows[i].URL); arid != "" {
			if rent, ok := rents[arid]; ok {
				rows[i].RentMin, rows[i].RentMax = rent, rent
			}
		}
	}
	return nil
}

func fetchKollegiumList(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, kollegiumListURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := addressHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kkik kollegium list status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// parseRentTable maps each room type's arid to its monthly rent (whole DKK). A
// room row is one div.row holding a roomtypedetails link and a price cell
// ("kr. N"); header rows and links without a price cell are skipped. Only ASCII
// (arid digits, "kr.", rent digits) is read, so the page's Latin-1 body needs no
// transcoding.
func parseRentTable(htmlStr string) map[string]int {
	out := map[string]int{}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlStr))
	if err != nil {
		return out
	}
	doc.Find("a[href*='roomtypedetails']").Each(func(_ int, link *goquery.Selection) {
		arid := aridFromURL(link.AttrOr("href", ""))
		if arid == "" {
			return
		}
		if _, seen := out[arid]; seen {
			return
		}
		// The price sits in a sibling col-md-2 cell of the same room row.
		link.Closest("div.row").ChildrenFiltered("div.col-md-2").EachWithBreak(func(_ int, cell *goquery.Selection) bool {
			t := cleanText(cell)
			if !strings.Contains(t, "kr") {
				return true
			}
			if rent, ok := parseKr(t); ok {
				out[arid] = rent
				return false
			}
			return true
		})
	})
	return out
}

func aridFromURL(u string) string {
	m := aridRe.FindStringSubmatch(u)
	if m == nil {
		return ""
	}
	return m[1]
}

// parseKr parses a KKIK price cell ("kr. 3,701.00", English thousands/decimal)
// into whole DKK.
func parseKr(s string) (int, bool) {
	s = strings.ReplaceAll(s, "kr.", "")
	s = strings.ReplaceAll(s, "kr", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(f), true
}
