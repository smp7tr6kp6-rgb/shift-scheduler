package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var errScheduleLocked = errors.New("schedule is approved and locked")

type ScheduleSubmission struct {
	Username         string
	ID               int64
	Schedule         map[string][]int
	Status           string
	RejectionComment string
	SubmittedAt      time.Time
}

var submissionStore *sql.DB

func openSubmissionStore(path string) (*sql.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(1)

	const schema = `
CREATE TABLE IF NOT EXISTS schedule_submissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL,
    schedule_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK (
        status IN ('pending', 'approved', 'rejected')
    ),
    rejection_comment TEXT NOT NULL DEFAULT '',
    submitted_at TEXT NOT NULL
);`

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}

func saveSubmission(
	db *sql.DB,
	username string,
	schedule map[string][]int,
) (int64, error) {
	encoded, err := json.Marshal(schedule)
	if err != nil {
		return 0, err
	}

	submittedAt := time.Now().UTC().Format(time.RFC3339Nano)

	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var id int64
	var status string

	err = tx.QueryRow(`
		SELECT id, status
		FROM schedule_submissions
		WHERE username = ?
		ORDER BY id DESC
		LIMIT 1
	`, username).Scan(&id, &status)

	switch {
	case err == sql.ErrNoRows:
		// No submission yet, so this is the user's first one.
		result, err := tx.Exec(`
			INSERT INTO schedule_submissions
				(username, schedule_json, status, rejection_comment, submitted_at)
			VALUES
				(?, ?, 'pending', '', ?)
		`,
			username,
			string(encoded),
			submittedAt,
		)
		if err != nil {
			return 0, err
		}

		id, err = result.LastInsertId()
		if err != nil {
			return 0, err
		}

	case err != nil:
		return 0, err

	case status == "approved":
		// Requirement: once approved the schedule is view-only until reset.
		return 0, errScheduleLocked

	default:
		// Pending or rejected, so the user may edit and resubmit in place.
		_, err = tx.Exec(`
			UPDATE schedule_submissions
			SET
				schedule_json = ?,
				status = 'pending',
				rejection_comment = '',
				submitted_at = ?
			WHERE id = ?
			  AND username = ?
			  AND status IN ('pending', 'rejected')
		`,
			string(encoded),
			submittedAt,
			id,
			username,
		)
		if err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return id, nil
}

// getLatestSubmission returns the user's most recent submission, whatever its
// status. It returns nil (and no error) when the user has never submitted.
func getLatestSubmission(
	db *sql.DB,
	username string,
) (*ScheduleSubmission, error) {
	var submission ScheduleSubmission
	var scheduleJSON string
	var submittedAt string

	err := db.QueryRow(`
		SELECT
			id,
			username,
			schedule_json,
			status,
			rejection_comment,
			submitted_at
		FROM schedule_submissions
		WHERE username = ?
		ORDER BY id DESC
		LIMIT 1
	`, username).Scan(
		&submission.ID,
		&submission.Username,
		&scheduleJSON,
		&submission.Status,
		&submission.RejectionComment,
		&submittedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(scheduleJSON), &submission.Schedule); err != nil {
		return nil, err
	}

	submission.SubmittedAt, err = time.Parse(time.RFC3339Nano, submittedAt)
	if err != nil {
		return nil, err
	}

	return &submission, nil
}

func approveSubmission(db *sql.DB, id int64) error {
	result, err := db.Exec(`
		UPDATE schedule_submissions
		SET
			status = 'approved',
			rejection_comment = ''
		WHERE id = ?
		  AND status = 'pending'
	`, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func rejectSubmission(db *sql.DB, id int64, comment string) error {
	result, err := db.Exec(`
		UPDATE schedule_submissions
		SET
			status = 'rejected',
			rejection_comment = ?
		WHERE id = ?
		  AND status = 'pending'
	`, comment, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// resetSubmission reopens an approved schedule so the user can edit it again.
// It lands back in 'rejected', which is the editable-and-resubmittable state.
func resetSubmission(db *sql.DB, id int64, comment string) error {
	result, err := db.Exec(`
		UPDATE schedule_submissions
		SET
			status = 'rejected',
			rejection_comment = ?
		WHERE id = ?
		  AND status = 'approved'
	`, comment, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// getAllSubmissions lists every submission for the admin queue, with the ones
// still needing a decision first.
func getAllSubmissions(db *sql.DB) ([]ScheduleSubmission, error) {
	rows, err := db.Query(`
		SELECT
			id,
			username,
			schedule_json,
			status,
			rejection_comment,
			submitted_at
		FROM schedule_submissions
		ORDER BY
			CASE status
				WHEN 'pending' THEN 0
				WHEN 'rejected' THEN 1
				ELSE 2
			END,
			id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var submissions []ScheduleSubmission

	for rows.Next() {
		var submission ScheduleSubmission
		var scheduleJSON string
		var submittedAt string

		if err := rows.Scan(
			&submission.ID,
			&submission.Username,
			&scheduleJSON,
			&submission.Status,
			&submission.RejectionComment,
			&submittedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(
			[]byte(scheduleJSON),
			&submission.Schedule,
		); err != nil {
			return nil, err
		}

		submission.SubmittedAt, err = time.Parse(time.RFC3339Nano, submittedAt)
		if err != nil {
			return nil, err
		}

		submissions = append(submissions, submission)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return submissions, nil
}
