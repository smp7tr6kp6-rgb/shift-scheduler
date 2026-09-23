package main

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

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
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

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

	var id int64

	err = db.QueryRow(`
		SELECT id
		FROM schedule_submissions
		WHERE username = ?
		  AND status IN ('pending', 'rejected')
		ORDER BY id DESC
		LIMIT 1
	`, username).Scan(&id)

	if err == sql.ErrNoRows {
		result, err := db.Exec(`
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

		return result.LastInsertId()
	}

	if err != nil {
		return 0, err
	}

	_, err = db.Exec(`
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

	return id, nil
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

func getPendingSubmissions(db *sql.DB) ([]ScheduleSubmission, error) {
	rows, err := db.Query(`
		SELECT
			id,
			username,
			schedule_json,
			status,
			rejection_comment,
			submitted_at
		FROM schedule_submissions
		WHERE status = 'pending'
		ORDER BY id ASC
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
