package mailer

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html"
	"mime"
	"mime/multipart"
	"net"
	"net/smtp"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"housing-waitlist/internal/commute"
	"housing-waitlist/internal/config"
	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"
)

// Section is one source's contribution to the combined report: its labels,
// parsed rows, CSV attachment, and (optional) live sheet link.
type Section struct {
	Name      string   // source token, e.g. "kkik"; prefixes the CSV attachment filename
	Title     string   // source title, e.g. "KKIK"
	PortalURL string   // source portal link
	Dests     []string // commute destination names, in order (empty when commute off)
	Result    *model.Result
	CSVPath   string
	SheetURL  string
}

// SendDigest emails one combined report covering every section, to a single recipient.
func SendDigest(cfg config.SMTP, to string, sections []Section) error {
	if len(sections) == 0 {
		return nil
	}
	subject := digestSubject(sections, time.Now())

	msg, err := buildMessage(cfg, to, subject, sections)
	if err != nil {
		return err
	}

	if err := sendSMTP(cfg, to, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// digestSubject names every source so the one email is recognizable at a glance.
func digestSubject(sections []Section, now time.Time) string {
	titles := make([]string, 0, len(sections))
	for _, s := range sections {
		titles = append(titles, s.Title)
	}
	return fmt.Sprintf("Denmark Housing Waitlist Report (%s) — %s", strings.Join(titles, ", "), now.Format("2006-01-02"))
}

func sendSMTP(cfg config.SMTP, to string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return err
		}
	}

	auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(cfg.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

// buildDigestBody renders one combined email with a section per source.
func buildDigestBody(sections []Section) string {
	var b strings.Builder
	fmt.Fprint(&b, `<!DOCTYPE html><html><body>`)
	fmt.Fprint(&b, `<p>Kính mợ,</p>`)
	for _, s := range sections {
		writeSection(&b, s)
	}
	fmt.Fprintf(&b, `<p>Generated: %s</p>`, time.Now().Format(time.RFC3339))
	fmt.Fprint(&b, `<p>Anh Zou trân trọng chơm một cái &lt;3</p></body></html>`)
	return b.String()
}

// writeSection renders one source's block: heading, meta, top-5, and links.
func writeSection(b *strings.Builder, s Section) {
	fmt.Fprintf(b, `<h2>%s waitlist:</h2>`, html.EscapeString(s.Title))
	fmt.Fprintf(b, `<p>Rows: %d<br>`, len(s.Result.Rows))
	if s.Result.Meta.ApplicantName != "" {
		fmt.Fprintf(b, `Applicant: %s (aka em pé cụa anh)<br>`, html.EscapeString(s.Result.Meta.ApplicantName))
	}
	if s.Result.Meta.RenewalDeadline != "" {
		fmt.Fprintf(b, `Renew before: %s<br>`, html.EscapeString(s.Result.Meta.RenewalDeadline))
	}
	fmt.Fprint(b, `</p>`)

	sorted := export.SortRows(s.Result.Rows)
	fmt.Fprint(b, `<p><strong>Top 5 best positions:</strong><br>`)
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		row := sorted[i]
		line := fmt.Sprintf("%d. #%s — %s — %s", i+1, row.RankDisplay, row.Dorm, truncate(row.RoomType, 60))
		if c := commuteSummary(row, s.Dests); c != "" {
			line += "  " + c
		}
		fmt.Fprintf(b, `%s<br>`, html.EscapeString(line))
	}
	fmt.Fprint(b, `</p>`)

	fmt.Fprintf(b, `<p>For more details, see <a href="%s">the %s housing portal</a>`,
		html.EscapeString(s.PortalURL), html.EscapeString(s.Title))
	if s.SheetURL != "" {
		fmt.Fprintf(b, `, <a href="%s">the live Google Sheet</a>`, html.EscapeString(s.SheetURL))
	}
	fmt.Fprint(b, `, or the attached CSV file.</p>`)
}

func buildMessage(cfg config.SMTP, to, subject string, sections []Section) ([]byte, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	h := make(map[string][]string)
	h["From"] = []string{cfg.From}
	h["To"] = []string{to}
	h["Subject"] = []string{subject}
	h["MIME-Version"] = []string{"1.0"}
	h["Content-Type"] = []string{fmt.Sprintf("multipart/mixed; boundary=%s", w.Boundary())}
	if err := writeHeaders(&buf, h); err != nil {
		return nil, err
	}

	htmlPart, err := w.CreatePart(map[string][]string{
		"Content-Type": {"text/html; charset=UTF-8"},
	})
	if err != nil {
		return nil, err
	}
	if _, err := htmlPart.Write([]byte(buildDigestBody(sections))); err != nil {
		return nil, err
	}

	for _, s := range sections {
		if err := attachCSV(w, s); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// attachCSV adds one source's CSV as an attachment, prefixing the filename with
// the source name so same-minute timestamped basenames don't collide.
func attachCSV(w *multipart.Writer, s Section) error {
	csvData, err := os.ReadFile(s.CSVPath)
	if err != nil {
		return fmt.Errorf("read csv attachment for %s: %w", s.Name, err)
	}
	fileName := fmt.Sprintf("%s_%s", s.Name, filepath.Base(s.CSVPath))
	attachPart, err := w.CreatePart(map[string][]string{
		"Content-Type":              {"text/csv; charset=UTF-8"},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, mime.QEncoding.Encode("utf-8", fileName))},
	})
	if err != nil {
		return err
	}
	enc := base64.NewEncoder(base64.StdEncoding, attachPart)
	if _, err := enc.Write(csvData); err != nil {
		return err
	}
	return enc.Close()
}

func writeHeaders(buf *bytes.Buffer, h map[string][]string) error {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h[k] {
			if _, err := fmt.Fprintf(buf, "%s: %s\r\n", k, v); err != nil {
				return err
			}
		}
	}
	_, err := buf.WriteString("\r\n")
	return err
}

// commuteSummary renders a row's per-campus headline for the email: morning
// transit and walk minutes for each destination. The full set (including the
// evening leg) lives in the CSV and Sheet; the email stays terse. Empty when the
// row has no commute data.
func commuteSummary(row model.WaitlistRow, dests []string) string {
	if len(row.Commute) == 0 || len(dests) == 0 {
		return ""
	}
	parts := make([]string, 0, len(dests))
	for _, d := range dests {
		t := orDash(row.Commute[commute.TransitMorningCol(d)])
		w := orDash(row.Commute[commute.WalkCol(d)])
		parts = append(parts, fmt.Sprintf("%s transit %s / walk %s min", d, t, w))
	}
	return "[" + strings.Join(parts, " · ") + "]"
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// DialSMTP can be used in tests to verify connectivity.
func DialSMTP(cfg config.SMTP) (net.Conn, error) {
	return net.Dial("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
}
