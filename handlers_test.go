package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestScheduleMasterHandlerSavesValidSubmission(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	response := postSchedule(t, validScheduleJSON())
	if response.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), "Pending") {
		t.Fatalf("expected pending status fragment, got %s", response.Body.String())
	}

	var status string
	if err := db.QueryRow("SELECT status FROM schedule_submissions WHERE id = 1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "pending" {
		t.Fatalf("expected stored status pending, got %q", status)
	}
}

func TestScheduleMasterHandlerRejectsInvalidSubmission(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	response := postSchedule(t, `{"Mon":[450,460,465,470,480,490,495,500,510,520,525,530,540,550,555,560]}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "shorter than 3 consecutive hours") {
		t.Fatalf("expected short-shift error, got %s", response.Body.String())
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedule_submissions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected invalid submission not to be saved, got %d rows", count)
	}
}

func TestScheduleMasterHandlerRejectsMalformedSubmission(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	response := postSchedule(t, "not-json")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "not valid JSON") {
		t.Fatalf("expected malformed JSON error, got %s", response.Body.String())
	}
}

func testSubmissionStore(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openSubmissionStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func postSchedule(t *testing.T, schedule string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/schedule/master", strings.NewReader("schedule="+schedule))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	scheduleMasterHandler(response, request)
	return response
}

func validScheduleJSON() string {
	return `{"Mon":[450,460,465,470,480,490,495,500,510,520,525,530,540,550,555,560,570,580,585,590,600,610,615,620,630,640,645,650,660,670,675,680,690,700,705,710,720,730,735,740],"Tue":[450,460,465,470,480,490,495,500,510,520,525,530,540,550,555,560,570,580,585,590,600,610,615,620,630,640,645,650,660,670,675,680,690,700,705,710,720,730,735,740],"Wed":[450,460,465,470,480,490,495,500,510,520,525,530,540,550,555,560,570,580,585,590,600,610,615,620,630,640,645,650,660,670,675,680,690,700,705,710,720,730,735,740],"Thu":[450,460,465,470,480,490,495,500,510,520,525,530,540,550,555,560,570,580,585,590,600,610,615,620,630,640,645,650,660,670,675,680,690,700,705,710,720,730,735,740]}`
}
