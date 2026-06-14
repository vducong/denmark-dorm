package kkik

import "testing"

func TestAddressFromHTML(t *testing.T) {
	// Mirrors a real KKIK detail page: the address is a UTF-8 percent-encoded
	// Google Maps q param, with a stray space before the comma.
	html := `<div class="maplink"><h3><a href="http://maps.google.com/maps?q=Strandlodsvej%2013K%20,%20K%c3%b8benhavn%20S">Show map</a></h3></div>`
	got, err := addressFromHTML(html)
	if err != nil {
		t.Fatalf("addressFromHTML err = %v", err)
	}
	if got != "Strandlodsvej 13K, København S" {
		t.Errorf("address = %q, want %q", got, "Strandlodsvej 13K, København S")
	}
}

func TestAddressFromHTML_noLink(t *testing.T) {
	if _, err := addressFromHTML(`<html><body>no map here</body></html>`); err == nil {
		t.Fatal("expected error when no map link is present")
	}
}
