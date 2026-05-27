package sheets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"denmark-housing-waitlist/internal/config"
	"denmark-housing-waitlist/internal/export"
	"denmark-housing-waitlist/internal/parser"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const (
	sheetColumnCount = 5
	headerRowIndex   = 1 // 0-based; row 0 is "Last updated at" metadata
)

// Update replaces header + data rows on the configured tab.
func Update(ctx context.Context, cfg *config.Config, rows []parser.WaitlistRow) (string, error) {
	httpClient, err := client(ctx, cfg)
	if err != nil {
		return "", err
	}

	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return "", fmt.Errorf("create sheets client: %w", err)
	}

	records := export.Records(rows)
	if len(records) == 0 {
		return "", fmt.Errorf("no rows to write")
	}
	values := sheetValues(records, time.Now())

	sheetID, err := sheetIDByName(svc, cfg.Sheets.SpreadsheetID, cfg.Sheets.SheetName)
	if err != nil {
		return "", err
	}

	clearRange := sheetRange(cfg.Sheets.SheetName, "A:Z")
	if _, err := svc.Spreadsheets.Values.Clear(cfg.Sheets.SpreadsheetID, clearRange, &sheets.ClearValuesRequest{}).Context(ctx).Do(); err != nil {
		return "", fmt.Errorf("clear sheet: %w", err)
	}

	updateRange := sheetRange(cfg.Sheets.SheetName, fmt.Sprintf("A1:E%d", len(values)))
	_, err = svc.Spreadsheets.Values.Update(
		cfg.Sheets.SpreadsheetID,
		updateRange,
		&sheets.ValueRange{
			MajorDimension: "ROWS",
			Values:         values,
		},
	).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("update sheet: %w", err)
	}

	if err := formatSheet(ctx, svc, cfg.Sheets.SpreadsheetID, sheetID); err != nil {
		return "", err
	}

	return cfg.SheetURL(), nil
}

func formatSheet(ctx context.Context, svc *sheets.Service, spreadsheetID string, sheetID int64) error {
	_, err := svc.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						SheetId:          sheetID,
						StartRowIndex:    headerRowIndex,
						EndRowIndex:      headerRowIndex + 1,
						StartColumnIndex: 0,
						EndColumnIndex:   sheetColumnCount,
					},
					Cell: &sheets.CellData{
						UserEnteredFormat: &sheets.CellFormat{
							TextFormat: &sheets.TextFormat{Bold: true},
						},
					},
					Fields: "userEnteredFormat.textFormat.bold",
				},
			},
			{
				UpdateSheetProperties: &sheets.UpdateSheetPropertiesRequest{
					Properties: &sheets.SheetProperties{
						SheetId: sheetID,
						GridProperties: &sheets.GridProperties{
							FrozenRowCount: headerRowIndex + 1,
						},
					},
					Fields: "gridProperties.frozenRowCount",
				},
			},
			{
				AutoResizeDimensions: &sheets.AutoResizeDimensionsRequest{
					Dimensions: &sheets.DimensionRange{
						SheetId:    sheetID,
						Dimension:  "COLUMNS",
						StartIndex: 0,
						EndIndex:   sheetColumnCount,
					},
				},
			},
		},
	}).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("format sheet: %w", err)
	}
	return nil
}

func sheetIDByName(svc *sheets.Service, spreadsheetID, sheetName string) (int64, error) {
	meta, err := svc.Spreadsheets.Get(spreadsheetID).Fields("sheets.properties").Do()
	if err != nil {
		return 0, fmt.Errorf("get spreadsheet: %w", err)
	}
	for _, sh := range meta.Sheets {
		if sh.Properties != nil && sh.Properties.Title == sheetName {
			return sh.Properties.SheetId, nil
		}
	}
	var names []string
	for _, sh := range meta.Sheets {
		if sh.Properties != nil {
			names = append(names, sh.Properties.Title)
		}
	}
	return 0, fmt.Errorf("sheet tab %q not found (available: %s)", sheetName, strings.Join(names, ", "))
}

// sheetValues prepends a metadata row: A1="Last updated at", B1=<timestamp>.
func sheetValues(records [][]string, updatedAt time.Time) [][]any {
	data := recordsToValues(records)
	out := make([][]any, 0, len(data)+1)
	out = append(out, []any{"Last updated at", updatedAt.Format(time.RFC3339)})
	out = append(out, data...)
	return out
}

func recordsToValues(records [][]string) [][]any {
	out := make([][]any, len(records))
	for i, rec := range records {
		row := make([]any, len(rec))
		for j, v := range rec {
			row[j] = v
		}
		out[i] = row
	}
	return out
}

func sheetRange(sheetName, cells string) string {
	if strings.ContainsAny(sheetName, " '!") {
		escaped := strings.ReplaceAll(sheetName, "'", "''")
		return fmt.Sprintf("'%s'!%s", escaped, cells)
	}
	return sheetName + "!" + cells
}
