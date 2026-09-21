package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sort"
)

var templates = template.Must(template.ParseGlob("./templates/*.html"))

func render(w http.ResponseWriter, name string, data any) {
	var page bytes.Buffer
	if err := templates.ExecuteTemplate(&page, name, data); err != nil {
		log.Println(err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	if r, ok := data.(*http.Request); ok && r.Header.Get("HX-Request") == "true" {
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

func homeHandler(w http.ResponseWriter, r *http.Request) {
	render(w, "home", r)
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
	if _, err := saveSubmission(submissionStore, schedule); err != nil {
		log.Println("save schedule submission:", err)
		writeScheduleError(w, http.StatusInternalServerError, "Could not save the schedule submission.")
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<span id="id-a82dd6d7-f6de-4617-8432-85bc55ec8091-status" class="status-badge ml-2 flex-shrink-0 inline-block px-2 py-0.5 text-xs font-semibold rounded-full text-yellow-800 bg-yellow-100">Pending</span>`))
}

const (
	minShiftMinutes  = 3 * 60
	maxDailyMinutes  = 9 * 60
	minWeeklyMinutes = 20 * 60
	maxWeeklyMinutes = 40 * 60
)

var scheduleDays = map[string]bool{
	"Mon": true,
	"Tue": true,
	"Wed": true,
	"Thu": true,
	"Fri": true,
}

var selectableMinutes = map[int]bool{
	0: true, 10: true, 15: true, 20: true,
	30: true, 40: true, 45: true, 50: true,
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
			if minute < 450 || minute > 1110 || !selectableMinutes[minute%60] {
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
	selectable := []int{0, 10, 15, 20, 30, 40, 45, 50}
	minutePart := minute % 60
	for index, value := range selectable {
		if minutePart == value {
			if index == len(selectable)-1 {
				return 60 - value
			}
			return selectable[index+1] - value
		}
	}
	return 0
}

func writeScheduleError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	fmt.Fprintf(w, `<span id="id-a82dd6d7-f6de-4617-8432-85bc55ec8091-status" class="status-badge ml-2 flex-shrink-0 inline-block px-2 py-0.5 text-xs font-semibold rounded-full text-red-800 bg-red-100">%s</span>`, template.HTMLEscapeString(message))
}
