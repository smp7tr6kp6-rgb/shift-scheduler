package main

import (
	"database/sql"
	"encoding/json"
	"time"

	_ "modernc.org/sqlite"
)

type ScheduleSubmission struct {
	EmployeeID       int
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
	employee_id INTEGER NOT NULL,
	schedule_json TEXT NOT NULL,
	status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'rejected')),
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
	employeeID int,
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
		WHERE employee_id = ?
		  AND status IN ('pending', 'rejected')
		ORDER BY id DESC
		LIMIT 1
	`,
		employeeID,
	).Scan(&id)

	if err == sql.ErrNoRows {
		result, err := db.Exec(`
			INSERT INTO schedule_submissions
				(employee_id, schedule_json, status, rejection_comment, submitted_at)
			VALUES
				(?, ?, 'pending', '', ?)
		`,
			employeeID,
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
		  AND employee_id = ?
		  AND status IN ('pending', 'rejected')
	`,
		string(encoded),
		submittedAt,
		id,
		employeeID,
	)

	if err != nil {
		return 0, err
	}

	return id, nil
}
