package sheets

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"denmark-housing-waitlist/internal/config"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sheets/v4"
)

// Loopback redirect for Desktop OAuth clients (no redirect URIs in Cloud Console).
const oauthRedirectURL = "http://127.0.0.1:8080/"

func printOAuthSetupChecklist() {
	fmt.Println(`Before authorizing, confirm in Google Cloud Console (same project as client_secret.json).

The old "OAuth consent screen" menu often redirects to Overview. Use:
  ☰ menu → Google Auth platform → Audience / Data access
  Or: https://console.cloud.google.com/auth/audience?project=YOUR_PROJECT_ID

  1. Google Auth platform → Overview: click Get started if shown; pick External user type
  2. Audience: Publishing status = Testing; Test users = your Gmail
  3. Data access: add scope Google Sheets API → .../auth/spreadsheets
  4. Enabled APIs: Google Sheets API ON

On "Google hasn't verified this app": Advanced → Go to <app name> (unsafe)`)
}

// Authenticate runs the one-time OAuth browser flow and writes the token file.
func Authenticate(ctx context.Context, cfg *config.Config) error {
	oauthCfg, err := oauthConfig(cfg)
	if err != nil {
		return err
	}

	printOAuthSetupChecklist()

	state := "kkik-waitlist"
	authURL := oauthCfg.AuthCodeURL(state, oauth2.AccessTypeOffline)

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	srv := &http.Server{Addr: "127.0.0.1:8080"}
	srv.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			http.Error(w, "invalid state", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth state mismatch")
			return
		}
		if oauthErr := r.URL.Query().Get("error"); oauthErr != "" {
			desc := r.URL.Query().Get("error_description")
			http.Error(w, "Authorization failed. See the terminal.", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth error %s: %s", oauthErr, desc)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			errCh <- fmt.Errorf("oauth code missing")
			return
		}
		fmt.Fprint(w, "Authorization complete. You can close this tab and return to the terminal.")
		codeCh <- code
	})

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer srv.Shutdown(ctx)

	fmt.Println("Open this URL in your browser to authorize Google Sheets access:")
	fmt.Println(authURL)

	select {
	case err := <-errCh:
		return fmt.Errorf("oauth callback server: %w", err)
	case code := <-codeCh:
		tok, err := oauthCfg.Exchange(ctx, code)
		if err != nil {
			return fmt.Errorf("oauth exchange: %w", err)
		}
		if err := saveToken(cfg.Sheets.OAuthTokenFile, tok); err != nil {
			return err
		}
		fmt.Println("Saved token to", cfg.Sheets.OAuthTokenFile)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func oauthConfig(cfg *config.Config) (*oauth2.Config, error) {
	b, err := os.ReadFile(cfg.Sheets.OAuthClientFile)
	if err != nil {
		return nil, fmt.Errorf("read oauth client file: %w", err)
	}
	oauthCfg, err := google.ConfigFromJSON(b, sheets.SpreadsheetsScope)
	if err != nil {
		return nil, fmt.Errorf("parse oauth client file: %w", err)
	}
	oauthCfg.RedirectURL = oauthRedirectURL
	return oauthCfg, nil
}

func client(ctx context.Context, cfg *config.Config) (*http.Client, error) {
	oauthCfg, err := oauthConfig(cfg)
	if err != nil {
		return nil, err
	}
	tok, err := loadToken(cfg.Sheets.OAuthTokenFile)
	if err != nil {
		return nil, err
	}
	return oauthCfg.Client(ctx, tok), nil
}

func loadToken(path string) (*oauth2.Token, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read oauth token file: %w", err)
	}
	var tok oauth2.Token
	if err := json.Unmarshal(b, &tok); err != nil {
		return nil, fmt.Errorf("parse oauth token file: %w", err)
	}
	return &tok, nil
}

func saveToken(path string, tok *oauth2.Token) error {
	b, err := json.Marshal(tok)
	if err != nil {
		return fmt.Errorf("encode token: %w", err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write token file: %w", err)
	}
	return nil
}
