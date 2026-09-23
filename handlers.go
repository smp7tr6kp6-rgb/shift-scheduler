package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
	"strconv"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username string
	Role     string // "student" | "admin"
}

func mustHash(password string) []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}

var users = map[string]struct {
	PasswordHash []byte
	Role         string
}{
	"student1": {mustHash("password"), "student"},
	"admin1":   {mustHash("password"), "admin"},
}

type Session struct {
	Username  string
	ExpiresAt time.Time
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]Session
}

var sessions = sessionStore{
	sessions: make(map[string]Session),
}

var templates = template.Must(template.ParseGlob("./templates/*.html"))

func render(w http.ResponseWriter, r *http.Request, name string, data any) {
	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, name, data); err != nil {
		log.Println(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, err := w.Write(page.Bytes())
		if err != nil {
			log.Println(err)
		}
		return
	}

	pageData := struct {
		Content template.HTML
	}{
		Content: template.HTML(page.String()),
	}

	if err := templates.ExecuteTemplate(w, "base", pageData); err != nil {
		log.Println(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// whenever user goes to website, it takes them to login first
func homeHandlerRedirect(w http.ResponseWriter, r *http.Request) {
	_, ok := getLoggedInUsername(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	homeHandler(w, r)
}

func generateToken() (string, error) {
	b := make([]byte, 32)

	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, ok := users[username]
	if !ok {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)

		fmt.Fprint(w, `
			<div class="text-red-600">
				Invalid username or password.
			</div>
		`)
		return
	}

	if err := bcrypt.CompareHashAndPassword(
		user.PasswordHash,
		[]byte(password),
	); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)

		fmt.Fprint(w, `
			<div class="text-red-600">
				Invalid username or password.
			</div>
		`)
		return
	}

	//generates random token for session for cookie
	token, err := generateToken()
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Storing session in map
	sessions.mu.Lock()
	sessions.sessions[token] = Session{
		Username:  username,
		ExpiresAt: time.Now().Add(8 * time.Hour),
	}
	sessions.mu.Unlock()

	// makes cookie cookie for session
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: false, //set to true before deployment
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 3600,
	})

	redirect := "/"

	if user.Role == "admin" {
		redirect = "/admin"
	}

	// Tell HTMX to redirect to home page
	w.Header().Set("HX-Redirect", redirect)
	w.WriteHeader(http.StatusNoContent)
}

// Renders login page
func loginPageHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "login", nil)
}

func getLoggedInUsername(r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return "", false
	}

	sessions.mu.RLock()
	session, ok := sessions.sessions[cookie.Value]
	sessions.mu.RUnlock()

	if !ok {
		return "", false
	}

	if time.Now().After(session.ExpiresAt) {
		sessions.mu.Lock()
		delete(sessions.sessions, cookie.Value)
		sessions.mu.Unlock()

		return "", false
	}

	return session.Username, true
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	render(w, r, "home", nil)
}

func scheduleMasterHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeScheduleError(w, http.StatusBadRequest, "Could not read the schedule submission.")
		return
	}

	var schedule map[string][]int
	if err := json.Unmarshal([]byte(r.FormValue("schedule")), &schedule); err != nil {
		writeScheduleError(w, http.StatusBadRequest, "The submitted schedule is not valid JSON.")
		return
	}

	if err := validateSchedule(schedule); err != nil {
		writeScheduleError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	if submissionStore == nil {
		writeScheduleError(w, http.StatusInternalServerError, "Schedule storage is unavailable.")
		return
	}

	username, ok := getLoggedInUsername(r)
	if !ok {
		writeScheduleError(w, http.StatusUnauthorized, "You must be logged in.")
		return
	}

	if _, err := saveSubmission(submissionStore, username, schedule); err != nil {
		log.Println("save schedule submission:", err)
		writeScheduleError(w, http.StatusInternalServerError, "Could not save the schedule submission.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span id="id-a82dd6d7-f6de-4617-8432-85bc55ec8091-status" class="status-badge ml-2 flex-shrink-0 inline-block px-2 py-0.5 text-xs font-semibold rounded-full text-yellow-800 bg-yellow-100">Pending</span>`))
}

// limits/constraints of how short or long you can work each shift, day, week
const (
	minShiftMinutes  = 3 * 60
	maxDailyMinutes  = 9 * 60
	minWeeklyMinutes = 20 * 60
	maxWeeklyMinutes = 40 * 60
)

// the days that are able to be selected to work on
var scheduleDays = map[string]bool{
	"Mon": true,
	"Tue": true,
	"Wed": true,
	"Thu": true,
	"Fri": true,
}

// used for how many minutes are okay(can't be 04:33 or something)
var selectableMinutes = map[int]bool{
	0: true, 10: true, 20: true,
	30: true, 40: true, 50: true,
}

func validateSchedule(schedule map[string][]int) error {
	weeklyMinutes := 0
	for day, minutes := range schedule {
		if !scheduleDays[day] {
			return fmt.Errorf("%s is not a valid schedule day", day)
		}

		sortedMinutes := append([]int(nil), minutes...)
		sort.Ints(sortedMinutes)
		if len(sortedMinutes) != len(uniqueMinutes(sortedMinutes)) {
			return fmt.Errorf("%s contains duplicate time slots", day)
		}

		for _, minute := range sortedMinutes {
			if minute < 480 || minute > 1080 || !selectableMinutes[minute%60] {
				return fmt.Errorf("%s contains a time outside 7:30 AM to 6:30 PM", day)
			}
		}

		dayMinutes := 0
		for _, block := range scheduleBlocks(sortedMinutes) {
			blockMinutes := block[1] - block[0]
			if blockMinutes < minShiftMinutes {
				return fmt.Errorf("%s has a shift shorter than 3 consecutive hours", day)
			}
			dayMinutes += blockMinutes
		}
		if dayMinutes > maxDailyMinutes {
			return fmt.Errorf("%s exceeds the 9-hour daily limit", day)
		}
		weeklyMinutes += dayMinutes
	}

	if weeklyMinutes < minWeeklyMinutes || weeklyMinutes > maxWeeklyMinutes {
		return fmt.Errorf("the schedule must total between 20 and 40 hours per week")
	}
	return nil
}

func uniqueMinutes(minutes []int) []int {
	unique := make([]int, 0, len(minutes))
	for _, minute := range minutes {
		if len(unique) == 0 || unique[len(unique)-1] != minute {
			unique = append(unique, minute)
		}
	}
	return unique
}

func scheduleBlocks(minutes []int) [][2]int {
	if len(minutes) == 0 {
		return nil
	}

	blocks := [][2]int{}
	start := minutes[0]
	last := minutes[0]
	for _, minute := range minutes[1:] {
		if minute == last+slotLength(last) {
			last = minute
			continue
		}
		blocks = append(blocks, [2]int{start, last + slotLength(last)})
		start = minute
		last = minute
	}
	return append(blocks, [2]int{start, last + slotLength(last)})
}

func slotLength(minute int) int {
	selectable := []int{0, 10, 20, 30, 40, 50}
	minutePart := minute % 60
	for index, value := range selectable {
		if minutePart == value {
			if index == len(selectable)-1 {
				return 60 - value
			}
			return selectable[index+1] - value
		}
	}
	return 10
}

func writeScheduleError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<span id="id-a82dd6d7-f6de-4617-8432-85bc55ec8091-status" class="status-badge ml-2 flex-shrink-0 inline-block px-2 py-0.5 text-xs font-semibold rounded-full text-red-800 bg-red-100">%s</span>`, template.HTMLEscapeString(message))
}
func getLoggedInUser(r *http.Request) (User, bool) {
	cookie, err := r.Cookie("session")
	if err != nil {
		return User{}, false
	}

	sessions.mu.RLock()
	session, ok := sessions.sessions[cookie.Value]
	sessions.mu.RUnlock()

	if !ok {
		return User{}, false
	}

	if time.Now().After(session.ExpiresAt) {
		sessions.mu.Lock()
		delete(sessions.sessions, cookie.Value)
		sessions.mu.Unlock()

		return User{}, false
	}

	user, ok := users[session.Username]
	if !ok {
		return User{}, false
	}

	return User{
		Username: session.Username,
		Role:     user.Role,
	}, true
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		// Remove the session from the session store.
		sessions.mu.Lock()
		delete(sessions.sessions, cookie.Value)
		sessions.mu.Unlock()
	}

	// Clear the session cookie in the browser.
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: false, //set to true before deployment
		SameSite: http.SameSiteLaxMode,
	})

	// Tell HTMX to go back to the login page.
	w.Header().Set("HX-Redirect", "/login")
	w.WriteHeader(http.StatusNoContent)
}

func requireAdmin(w http.ResponseWriter, r *http.Request) (User, bool) {
	user, ok := getLoggedInUser(r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return User{}, false
	}

	if user.Role != "admin" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return User{}, false
	}

	return user, true
}

func submissionsHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	if submissionStore == nil {
		http.Error(w, "submission storage is unavailable", http.StatusInternalServerError)
		return
	}

	submissions, err := getPendingSubmissions(submissionStore)
	if err != nil {
		log.Println("get pending submissions:", err)
		http.Error(w, "Could not load submissions.", http.StatusInternalServerError)
		return
	}

	render(w, r, "submissions", submissions)
}

func approveHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	if submissionStore == nil {
		http.Error(w, "submission storage is unavailable", http.StatusInternalServerError)
		return
	}

	idString := r.PathValue("id")

	submissionID, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	if err := approveSubmission(submissionStore, submissionID); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "submission not found or is no longer pending", http.StatusNotFound)
			return
		}

		log.Println("approve submission:", err)
		http.Error(w, "Could not approve submission.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/submissions", http.StatusSeeOther)
}

func rejectHandler(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r); !ok {
		return
	}

	if submissionStore == nil {
		http.Error(w, "submission storage is unavailable", http.StatusInternalServerError)
		return
	}

	idString := r.PathValue("id")
	comment := r.FormValue("comment")

	submissionID, err := strconv.ParseInt(idString, 10, 64)
	if err != nil {
		http.Error(w, "invalid submission id", http.StatusBadRequest)
		return
	}

	if err := rejectSubmission(submissionStore, submissionID, comment); err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "submission not found or is no longer pending", http.StatusNotFound)
			return
		}

		log.Println("reject submission:", err)
		http.Error(w, "Could not reject submission.", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/submissions", http.StatusSeeOther)
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	//checks if admin is required and present
	if _, ok := requireAdmin(w, r); !ok {
		return
	}
	//if admin is clear, render admin view
	render(w, r, "admin", nil)
}
