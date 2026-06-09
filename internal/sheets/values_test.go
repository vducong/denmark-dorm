package sheets

import (
	"testing"
	"time"

	"denmark-housing-waitlist/internal/parser"
)

func TestBuildMatrix_metadataRow(t *testing.T) {
	today := time.Date(2026, 5, 26, 15, 4, 5, 0, time.UTC)
	rows := []parser.WaitlistRow{
		{RequestID: "1", Dorm: "D", YourRank: 3},
	}

	matrix, err := BuildMatrix(rows, nil, nil, today)
	if err != nil {
		t.Fatalf("BuildMatrix() err = %v", err)
	}
	if matrix[0][0] != metaLabel || matrix[0][1] != "2026-05-26T15:04:05Z" {
		t.Errorf("meta row = %v", matrix[0])
	}
	if matrix[1][0] != "request_id" {
		t.Errorf("header row = %v", matrix[1])
	}
}
