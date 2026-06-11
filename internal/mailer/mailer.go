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

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"
)

// Report holds the per-source labels used in the email subject and body.
type Report struct {
	Title     string // source title, e.g. "KKIK"
	PortalURL string // source portal link
	To        string // recipient
}

// SendReport emails the CSV report with a short summary.
func SendReport(cfg config.SMTP, report Report, result *model.Result, csvPath, sheetURL string) error {
	body := buildBody(report, result, sheetURL)
	subject := fmt.Sprintf("%s waitlist report — %s", report.Title, time.Now().Format("2006-01-02"))

	msg, err := buildMessage(cfg, report.To, subject, body, csvPath)
	if err != nil {
		return err
	}

	if err := sendSMTP(cfg, report.To, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
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

func buildBody(report Report, result *model.Result, sheetURL string) string {
	var b strings.Builder
	fmt.Fprint(&b, `<!DOCTYPE html><html><body>`)
	fmt.Fprint(&b, `<p>Kính mợ,</p>`)
	fmt.Fprintf(&b, `<p><strong>%s waitlist report</strong><br>`, html.EscapeString(report.Title))
	fmt.Fprintf(&b, `Generated: %s<br>`, time.Now().Format(time.RFC3339))
	fmt.Fprintf(&b, `Rows: %d<br>`, len(result.Rows))
	if result.Meta.ApplicantName != "" {
		fmt.Fprintf(&b, `Applicant: %s (aka em pé cụa anh)<br>`, html.EscapeString(result.Meta.ApplicantName))
	}
	if result.Meta.RenewalDeadline != "" {
		fmt.Fprintf(&b, `Renew before: %s<br>`, html.EscapeString(result.Meta.RenewalDeadline))
	}

	sorted := export.SortRows(result.Rows)
	fmt.Fprint(&b, `<p><strong>Top 5 best positions:</strong><br>`)
	limit := 5
	if len(sorted) < limit {
		limit = len(sorted)
	}
	for i := 0; i < limit; i++ {
		row := sorted[i]
		line := fmt.Sprintf("%d. #%s — %s — %s", i+1, row.RankDisplay, row.Dorm, truncate(row.RoomType, 60))
		fmt.Fprintf(&b, `%s<br>`, html.EscapeString(line))
	}
	fmt.Fprint(&b, `</p>`)

	fmt.Fprintf(&b, `<p>For more details, see <a href="%s">the %s housing portal</a>`,
		html.EscapeString(report.PortalURL), html.EscapeString(report.Title))
	if sheetURL != "" {
		fmt.Fprintf(&b, `, <a href="%s">the live Google Sheet</a>`, html.EscapeString(sheetURL))
	}
	fmt.Fprint(&b, `, or the attached CSV file.</p>`)
	fmt.Fprint(&b, `<p>Anh Zou trân trọng chơm một cái &lt;3</p></body></html>`)
	return b.String()
}

func buildMessage(cfg config.SMTP, to, subject, body, csvPath string) ([]byte, error) {
	csvData, err := os.ReadFile(csvPath)
	if err != nil {
		return nil, fmt.Errorf("read csv attachment: %w", err)
	}

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
	if _, err := htmlPart.Write([]byte(body)); err != nil {
		return nil, err
	}

	fileName := filepath.Base(csvPath)
	attachPart, err := w.CreatePart(map[string][]string{
		"Content-Type":              {"text/csv; charset=UTF-8"},
		"Content-Transfer-Encoding": {"base64"},
		"Content-Disposition":       {fmt.Sprintf(`attachment; filename="%s"`, mime.QEncoding.Encode("utf-8", fileName))},
	})
	if err != nil {
		return nil, err
	}
	enc := base64.NewEncoder(base64.StdEncoding, attachPart)
	if _, err := enc.Write(csvData); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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
