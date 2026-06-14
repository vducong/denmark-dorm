package kkik

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// KKIK detail pages embed the dorm's address in a Google Maps link
// (…/maps?q=<address>); the q value is UTF-8 percent-encoded, so it decodes
// cleanly regardless of the page's Latin-1 body.
var mapsQueryRe = regexp.MustCompile(`maps\.google\.com/maps\?q=([^"&]+)`)

var addressHTTPClient = &http.Client{Timeout: 20 * time.Second}

// LookupAddress fetches a KKIK detail page and returns the dorm's street
// address. The page is public, so a plain GET (no login) suffices. The caller
// caches the result, so this runs once per dorm.
func (k *KKIK) LookupAddress(ctx context.Context, detailURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, detailURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := addressHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("kkik detail page status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return addressFromHTML(string(body))
}

// addressFromHTML extracts the street address from a KKIK detail page's Google
// Maps link.
func addressFromHTML(html string) (string, error) {
	m := mapsQueryRe.FindStringSubmatch(html)
	if m == nil {
		return "", fmt.Errorf("no address (map link) found on page")
	}
	dec, err := url.QueryUnescape(m[1])
	if err != nil {
		return "", fmt.Errorf("decode address: %w", err)
	}
	addr := normalizeAddress(dec)
	if addr == "" {
		return "", fmt.Errorf("empty address in map link")
	}
	return addr, nil
}

// normalizeAddress collapses whitespace and drops the space before a comma that
// KKIK's "Street No , City" formatting leaves behind.
func normalizeAddress(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return strings.ReplaceAll(s, " ,", ",")
}
