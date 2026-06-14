package mailer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/model"
)

func sampleSections(t *testing.T) []Section {
	t.Helper()
	dir := t.TempDir()
	kkikCSV := filepath.Join(dir, "202606130012_waitlist.csv")
	sdkCSV := filepath.Join(dir, "202606130012_waitlist.csv2") // distinct path, same basename intent
	if err := os.WriteFile(kkikCSV, []byte("request_id,dorm\n1,Husum\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sdkCSV, []byte("request_id,dorm\n9,Tietgen\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return []Section{
		{
			Name:      "kkik",
			Title:     "KKIK",
			PortalURL: "https://kkik.example/portal",
			SheetURL:  "https://sheet.example/kkik",
			CSVPath:   kkikCSV,
			Result: &model.Result{
				Rows: []model.WaitlistRow{
					{RequestID: "1", Dorm: "Husumvej", RoomType: "Room", RankDisplay: "3", RankOrder: 3},
				},
				Meta: model.Meta{ApplicantName: "Ada"},
			},
		},
		{
			Name:      "sdk",
			Title:     "s.dk",
			PortalURL: "https://sdk.example/portal",
			SheetURL:  "https://sheet.example/sdk",
			CSVPath:   sdkCSV,
			Result: &model.Result{
				Rows: []model.WaitlistRow{
					{RequestID: "9", Dorm: "Tietgen", RoomType: "Studio", RankDisplay: "B", RankOrder: 2},
				},
				Meta: model.Meta{ApplicantName: "Ada"},
			},
		},
	}
}

func TestBuildDigestBody_multipleSections(t *testing.T) {
	body := buildDigestBody(sampleSections(t))

	for _, want := range []string{
		"KKIK", "s.dk", // both source titles
		"Husumvej", "Tietgen", // both top leaders
		"https://sheet.example/kkik", "https://sheet.example/sdk", // both sheet links
		"https://kkik.example/portal", "https://sdk.example/portal", // both portal links
	} {
		if !strings.Contains(body, want) {
			t.Errorf("digest body missing %q", want)
		}
	}
}

func TestWriteSection_commuteSummary(t *testing.T) {
	dir := t.TempDir()
	csv := filepath.Join(dir, "202606130012_waitlist.csv")
	if err := os.WriteFile(csv, []byte("request_id,dorm\n1,X\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sections := []Section{{
		Name:    "sdk",
		Title:   "s.dk",
		Dests:   []string{"cbs", "rda"},
		CSVPath: csv,
		Result: &model.Result{Rows: []model.WaitlistRow{
			{RequestID: "1", Dorm: "Nørrebro", RoomType: "Room", RankDisplay: "B", RankOrder: 2,
				Commute: map[string]string{
					"cbs_transit_morning_min": "28", "cbs_walk_min": "44",
					"rda_transit_morning_min": "", "rda_walk_min": "70",
				}},
		}},
	}}
	body := buildDigestBody(sections)
	for _, want := range []string{"cbs transit 28 / walk 44 min", "rda transit – / walk 70 min"} {
		if !strings.Contains(body, want) {
			t.Errorf("digest body missing %q\n%s", want, body)
		}
	}
}

func TestBuildMessage_multipleAttachments(t *testing.T) {
	sections := sampleSections(t)
	msg, err := buildMessage(config.SMTP{From: "from@example.com"}, "to@example.com", "subject", sections)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}
	s := string(msg)

	if got := strings.Count(s, "Content-Disposition: attachment"); got != 2 {
		t.Errorf("attachment parts = %d, want 2", got)
	}
	// Attachment filenames are prefixed with the source name so two sources that
	// scrape in the same minute don't collide on the timestamped basename.
	if !strings.Contains(s, `filename="kkik_`) || !strings.Contains(s, `filename="sdk_`) {
		t.Errorf("attachments not source-prefixed:\n%s", s)
	}
	if !strings.Contains(s, "To: to@example.com") {
		t.Error("missing To header")
	}
}
