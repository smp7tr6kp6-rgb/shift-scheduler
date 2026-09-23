package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
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

	response := postSchedule(t, shortShiftScheduleJSON())
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

	token, err := generateToken()
	if err != nil {
		t.Fatal(err)
	}

	sessions.mu.Lock()
	sessions.sessions[token] = Session{
		Username:  "student1",
		ExpiresAt: time.Now().Add(8 * time.Hour),
	}
	sessions.mu.Unlock()

	t.Cleanup(func() {
		sessions.mu.Lock()
		delete(sessions.sessions, token)
		sessions.mu.Unlock()
	})

	request := httptest.NewRequest(
		http.MethodPost,
		"/schedule/master",
		strings.NewReader("schedule="+schedule),
	)

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.AddCookie(&http.Cookie{
		Name:  "session",
		Value: token,
	})

	response := httptest.NewRecorder()
	scheduleMasterHandler(response, request)

	return response
}

// dayJSON builds a contiguous run of 10 minute slots starting at startMinute.
func dayJSON(startMinute, hours int) string {
	slots := make([]string, 0, hours*6)
	for minute := startMinute; minute < startMinute+hours*60; minute += 10 {
		slots = append(slots, strconv.Itoa(minute))
	}
	return "[" + strings.Join(slots, ",") + "]"
}

// validScheduleJSON is 5 hours on each of Mon-Thu, so 20 hours for the week.
func validScheduleJSON() string {
	days := make([]string, 0, 4)
	for _, day := range []string{"Mon", "Tue", "Wed", "Thu"} {
		days = append(days, `"`+day+`":`+dayJSON(8*60, 5))
	}
	return "{" + strings.Join(days, ",") + "}"
}

// shortShiftScheduleJSON is a single 2 hour shift, below the 3 hour minimum.
func shortShiftScheduleJSON() string {
	return `{"Mon":` + dayJSON(8*60, 2) + `}`
}

func TestScheduleMasterHandlerRequiresLogin(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	request := httptest.NewRequest(
		http.MethodPost,
		"/schedule/master",
		strings.NewReader("schedule="+validScheduleJSON()),
	)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response := httptest.NewRecorder()
	scheduleMasterHandler(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", response.Code)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedule_submissions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected no submission to be saved, got %d rows", count)
	}
}

func TestScheduleMasterHandlerRejectsOutsideWindow(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	// 7:00 AM is before the 8:00 AM start of the window.
	response := postSchedule(t, `{"Mon":`+dayJSON(7*60, 4)+`}`)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "8:00 AM to 6:00 PM") {
		t.Fatalf("expected window error, got %s", response.Body.String())
	}
}

func TestScheduleMasterHandlerRejectsOverlongWeek(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	// 9 hours on each of 5 days is 45 hours, over the 40 hour weekly cap.
	days := make([]string, 0, 5)
	for _, day := range []string{"Mon", "Tue", "Wed", "Thu", "Fri"} {
		days = append(days, `"`+day+`":`+dayJSON(8*60, 9))
	}

	response := postSchedule(t, "{"+strings.Join(days, ",")+"}")
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", response.Code)
	}
	if !strings.Contains(response.Body.String(), "between 20 and 40 hours") {
		t.Fatalf("expected weekly total error, got %s", response.Body.String())
	}
}

func TestResubmitReplacesRejectedSubmission(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	if response := postSchedule(t, validScheduleJSON()); response.Code != http.StatusOK {
		t.Fatalf("first submit: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if err := rejectSubmission(db, 1, "not enough Friday cover"); err != nil {
		t.Fatal(err)
	}

	if response := postSchedule(t, validScheduleJSON()); response.Code != http.StatusOK {
		t.Fatalf("resubmit: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	// The rejected row should be reused and cleared, not duplicated.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedule_submissions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row after resubmit, got %d", count)
	}

	submission, err := getLatestSubmission(db, "student1")
	if err != nil {
		t.Fatal(err)
	}
	if submission.Status != "pending" {
		t.Fatalf("expected status pending, got %q", submission.Status)
	}
	if submission.RejectionComment != "" {
		t.Fatalf("expected rejection comment to be cleared, got %q", submission.RejectionComment)
	}
}

func TestApprovedScheduleIsViewOnly(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	if response := postSchedule(t, validScheduleJSON()); response.Code != http.StatusOK {
		t.Fatalf("first submit: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	if err := approveSubmission(db, 1); err != nil {
		t.Fatal(err)
	}

	response := postSchedule(t, validScheduleJSON())
	if response.Code != http.StatusConflict {
		t.Fatalf("expected status 409 once approved, got %d: %s", response.Code, response.Body.String())
	}

	var status string
	if err := db.QueryRow("SELECT status FROM schedule_submissions WHERE id = 1").Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "approved" {
		t.Fatalf("expected the approved schedule to be untouched, got %q", status)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM schedule_submissions").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected no extra pending row, got %d rows", count)
	}
}

func TestResetReopensApprovedSchedule(t *testing.T) {
	db := testSubmissionStore(t)
	oldStore := submissionStore
	submissionStore = db
	t.Cleanup(func() { submissionStore = oldStore })

	if response := postSchedule(t, validScheduleJSON()); response.Code != http.StatusOK {
		t.Fatalf("first submit: expected 200, got %d: %s", response.Code, response.Body.String())
	}
	if err := approveSubmission(db, 1); err != nil {
		t.Fatal(err)
	}
	if err := resetSubmission(db, 1, "roster changed"); err != nil {
		t.Fatal(err)
	}

	// After a reset the user can edit and resubmit again.
	if response := postSchedule(t, validScheduleJSON()); response.Code != http.StatusOK {
		t.Fatalf("resubmit after reset: expected 200, got %d: %s", response.Code, response.Body.String())
	}

	submission, err := getLatestSubmission(db, "student1")
	if err != nil {
		t.Fatal(err)
	}
	if submission.Status != "pending" {
		t.Fatalf("expected status pending after resubmit, got %q", submission.Status)
	}

	// Resetting something that is not approved is a no-op.
	if err := resetSubmission(db, 1, "again"); err != sql.ErrNoRows {
		t.Fatalf("expected ErrNoRows resetting a pending row, got %v", err)
	}
}

func TestGetLatestSubmissionWithNoRows(t *testing.T) {
	db := testSubmissionStore(t)

	submission, err := getLatestSubmission(db, "student1")
	if err != nil {
		t.Fatal(err)
	}
	if submission != nil {
		t.Fatalf("expected nil submission, got %+v", submission)
	}
}

func TestRootPathRejectsUnknownRoutes(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/not-a-real-page", nil)
	response := httptest.NewRecorder()

	homeHandlerRedirect(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", response.Code)
	}
}

func TestLogoutRedirectsForPlainRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/logout", nil)
	response := httptest.NewRecorder()

	logoutHandler(response, request)

	if response.Code != http.StatusSeeOther {
		t.Fatalf("expected status 303, got %d", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/login" {
		t.Fatalf("expected redirect to /login, got %q", location)
	}
}

func TestAdminRoutesRejectStudents(t *testing.T) {
	token, err := generateToken()
	if err != nil {
		t.Fatal(err)
	}

	sessions.mu.Lock()
	sessions.sessions[token] = Session{
		Username:  "student1",
		ExpiresAt: time.Now().Add(8 * time.Hour),
	}
	sessions.mu.Unlock()

	t.Cleanup(func() {
		sessions.mu.Lock()
		delete(sessions.sessions, token)
		sessions.mu.Unlock()
	})

	request := httptest.NewRequest(http.MethodGet, "/admin", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: token})

	response := httptest.NewRecorder()
	adminHandler(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 for a student, got %d", response.Code)
	}
}
