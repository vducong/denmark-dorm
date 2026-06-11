package sheets

import (
	"context"
	"fmt"
	"strings"
	"time"

	"housing-waitlist/internal/config"
	"housing-waitlist/internal/export"
	"housing-waitlist/internal/model"

	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
)

const headerRowIndex = 1 // 0-based; row 0 is "Last updated at" metadata

// Update merges scrape data into the target tab using time-series ddmmyy
// columns. csvDir is the source's data dir scanned for historical snapshots;
// rankOrder projects display ranks for the latest_diff column.
func Update(
	ctx context.Context,
	google config.Google,
	target config.SheetTarget,
	rows []model.WaitlistRow,
	csvDir string,
	rankOrder func(string) (int, bool),
) (string, error) {
	httpClient, err := client(ctx, google)
	if err != nil {
		return "", err
	}

	svc, err := sheets.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return "", fmt.Errorf("create sheets client: %w", err)
	}

	snapshots, err := export.LoadDailySnapshots(csvDir)
	if err != nil {
		return "", err
	}

	readRange := sheetRange(target.SheetName, "A:ZZ")
	resp, err := svc.Spreadsheets.Values.Get(target.SpreadsheetID, readRange).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("read sheet: %w", err)
	}

	now := time.Now()
	values, err := BuildMatrix(rows, snapshots, resp.Values, now, rankOrder)
	if err != nil {
		return "", err
	}

	sheetID, err := sheetIDByName(svc, target.SpreadsheetID, target.SheetName)
	if err != nil {
		return "", err
	}

	colCount := matrixWidth(values)
	lastCol := columnLetter(colCount)
	updateRange := sheetRange(target.SheetName, fmt.Sprintf("A1:%s%d", lastCol, len(values)))
	_, err = svc.Spreadsheets.Values.Update(
		target.SpreadsheetID,
		updateRange,
		&sheets.ValueRange{
			MajorDimension: "ROWS",
			Values:         values,
		},
	).ValueInputOption("RAW").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("update sheet: %w", err)
	}

	if err := formatSheet(ctx, svc, target.SpreadsheetID, sheetID, colCount); err != nil {
		return "", err
	}

	return config.SheetURL(target.SpreadsheetID), nil
}

func formatSheet(ctx context.Context, svc *sheets.Service, spreadsheetID string, sheetID int64, columnCount int) error {
	_, err := svc.Spreadsheets.BatchUpdate(spreadsheetID, &sheets.BatchUpdateSpreadsheetRequest{
		Requests: []*sheets.Request{
			{
				RepeatCell: &sheets.RepeatCellRequest{
					Range: &sheets.GridRange{
						SheetId:          sheetID,
						StartRowIndex:    headerRowIndex,
						EndRowIndex:      headerRowIndex + 1,
						StartColumnIndex: 0,
						EndColumnIndex:   int64(columnCount),
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
						EndIndex:   int64(columnCount),
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

func matrixWidth(values [][]any) int {
	max := 0
	for _, row := range values {
		if len(row) > max {
			max = len(row)
		}
	}
	return max
}

func columnLetter(n int) string {
	if n <= 0 {
		return "A"
	}
	var b strings.Builder
	for n > 0 {
		n--
		b.WriteByte(byte('A' + n%26))
		n /= 26
	}
	runes := []rune(b.String())
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func sheetRange(sheetName, cells string) string {
	if strings.ContainsAny(sheetName, " '!") {
		escaped := strings.ReplaceAll(sheetName, "'", "''")
		return fmt.Sprintf("'%s'!%s", escaped, cells)
	}
	return sheetName + "!" + cells
}
