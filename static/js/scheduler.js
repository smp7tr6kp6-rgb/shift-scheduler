const days = ["Mon", "Tue", "Wed", "Thu", "Fri"];

const START_MINUTE = 8 * 60;
const END_MINUTE = 18 * 60;
const SLOT_LENGTH = 10;

const MIN_SHIFT_MINUTES = 3 * 60;
const MAX_DAILY_MINUTES = 9 * 60;
const MIN_WEEKLY_MINUTES = 20 * 60;
const MAX_WEEKLY_MINUTES = 40 * 60;

let schedule = {
    Mon: [],
    Tue: [],
    Wed: [],
    Thu: [],
    Fri: []
};

let readOnly = false;

let dragging = false;
let dragDay = null;
let dragStart = null;
let dragEnd = null;
let dragMode = "select";


function initializeScheduler() {

    const grid = document.getElementById("schedule-grid");

    if (!grid) {
        return;
    }


    if (grid.children.length > 0) {
        return;
    }

    readOnly = grid.dataset.readonly === "true";

    schedule = loadSavedSchedule();

    createGrid(grid);
    updateDisplay();
}


function loadSavedSchedule() {

    const empty = {
        Mon: [],
        Tue: [],
        Wed: [],
        Thu: [],
        Fri: []
    };

    const saved = window.SAVED_SCHEDULE;

    if (!saved || typeof saved !== "object") {
        return empty;
    }

    for (const day of days) {

        if (!Array.isArray(saved[day])) {
            continue;
        }

        empty[day] = saved[day]
            .filter(minute =>
                Number.isInteger(minute) &&
                minute >= START_MINUTE &&
                minute < END_MINUTE &&
                minute % SLOT_LENGTH === 0
            )
            .sort((a, b) => a - b);
    }

    return empty;
}


function createGrid(grid) {

    for (
        let minute = START_MINUTE;
        minute < END_MINUTE;
        minute += SLOT_LENGTH
    ) {


        const timeCell = document.createElement("div");

        timeCell.className = "time-cell";

        if (minute % 60 === 0) {
            timeCell.textContent = formatTime(minute);
        }

        grid.appendChild(timeCell);


       
        for (const day of days) {

            const slot = document.createElement("div");

            slot.className = "schedule-slot";

            slot.dataset.day = day;
            slot.dataset.minute = minute;

            slot.addEventListener("mousedown", function(event) {

                event.preventDefault();

                startDrag(day, minute);

            });

            slot.addEventListener("mouseenter", function() {

                if (!dragging) {
                    return;
                }

                if (day !== dragDay) {
                    return;
                }

                dragEnd = minute;

                updateDragPreview();

            });

            grid.appendChild(slot);
        }
    }
}


function startDrag(day, minute) {

    if (readOnly) {
        return;
    }

    dragging = true;

    dragDay = day;

    dragStart = minute;

    dragEnd = minute;

    dragMode =
        schedule[day].includes(minute)
            ? "remove"
            : "select";

    updateDragPreview();
}


document.addEventListener("mouseup", function() {

    if (!dragging) {
        return;
    }

    finishDrag();

    dragging = false;

    dragDay = null;
    dragStart = null;
    dragEnd = null;

});


function finishDrag() {

    const start = Math.min(dragStart, dragEnd);
    const end = Math.max(dragStart, dragEnd);

    for (
        let minute = start;
        minute <= end;
        minute += SLOT_LENGTH
    ) {

        const selected =
            schedule[dragDay].includes(minute);

        if (dragMode === "select") {

            if (!selected) {
                schedule[dragDay].push(minute);
            }

        } else {

            if (selected) {

                schedule[dragDay] =
                    schedule[dragDay].filter(
                        value => value !== minute
                    );

            }
        }
    }

    schedule[dragDay].sort(
        (a, b) => a - b
    );

    updateDisplay();
}


function updateDragPreview() {

    document
        .querySelectorAll(".drag-preview")
        .forEach(slot => {
            slot.classList.remove("drag-preview");
        });

    const start =
        Math.min(dragStart, dragEnd);

    const end =
        Math.max(dragStart, dragEnd);

    for (
        let minute = start;
        minute <= end;
        minute += SLOT_LENGTH
    ) {

        const slot =
            getSlot(dragDay, minute);

        if (slot) {
            slot.classList.add("drag-preview");
        }
    }
}


function updateDisplay() {

    updateSelectedSlots();

    updateDailyTotals();

    updateWeeklyTotal();

    updateForm();

    validateClientSide();
}


function updateSelectedSlots() {

    document
        .querySelectorAll(".schedule-slot")
        .forEach(slot => {

            const day = slot.dataset.day;

            const minute =
                Number(slot.dataset.minute);

            slot.classList.toggle(
                "selected",
                schedule[day].includes(minute)
            );

        });
}


function updateDailyTotals() {

    for (const day of days) {

        const minutes =
            schedule[day].length * SLOT_LENGTH;

        const hours =
            minutes / 60;

        const element =
            document.getElementById(
                `total-${day}`
            );

        if (element) {

            element.textContent =
                `${hours.toFixed(1)}h`;

        }
    }
}


function updateWeeklyTotal() {

    let totalMinutes = 0;

    for (const day of days) {

        totalMinutes +=
            schedule[day].length * SLOT_LENGTH;

    }

    const hours =
        totalMinutes / 60;

    const element =
        document.getElementById("weekly-total");

    if (element) {

        element.textContent =
            `${hours.toFixed(1)} hours`;

    }
}


function updateForm() {

    const input =
        document.getElementById("schedule-json");

    if (input) {

        input.value =
            JSON.stringify(schedule);

    }
}


function shiftBlocks(minutes) {

    if (minutes.length === 0) {
        return [];
    }

    const sorted = [...minutes].sort((a, b) => a - b);

    const blocks = [];

    let start = sorted[0];
    let last = sorted[0];

    for (const minute of sorted.slice(1)) {

        if (minute === last + SLOT_LENGTH) {
            last = minute;
            continue;
        }

        blocks.push({ start: start, end: last + SLOT_LENGTH });

        start = minute;
        last = minute;
    }

    blocks.push({ start: start, end: last + SLOT_LENGTH });

    return blocks;
}


function validateClientSide() {

    const validation =
        document.getElementById(
            "schedule-validation"
        );

    const submitButton =
        document.getElementById(
            "submit-schedule"
        );

    if (!validation || !submitButton) {
        return;
    }

    const problem = findScheduleProblem();

    validation.textContent = problem === null ? "" : problem;

    submitButton.disabled = problem !== null;
}


function findScheduleProblem() {

    let weeklyMinutes = 0;

    for (const day of days) {

        const minutes =
            schedule[day].length *
            SLOT_LENGTH;

        weeklyMinutes += minutes;

        if (minutes > MAX_DAILY_MINUTES) {
            return `${day} exceeds the 9 hour daily limit.`;
        }

        for (const block of shiftBlocks(schedule[day])) {

            if (block.end - block.start < MIN_SHIFT_MINUTES) {
                return `${day} has a shift shorter than 3 consecutive hours.`;
            }
        }
    }

    if (weeklyMinutes < MIN_WEEKLY_MINUTES) {
        return "You need at least 20 hours per week.";
    }

    if (weeklyMinutes > MAX_WEEKLY_MINUTES) {
        return "You cannot exceed 40 hours per week.";
    }

    return null;
}


function getSlot(day, minute) {

    return document.querySelector(
        `.schedule-slot[data-day="${day}"][data-minute="${minute}"]`
    );
}


function formatTime(minutes) {

    let hours =
        Math.floor(minutes / 60);

    const mins =
        minutes % 60;

    const suffix =
        hours >= 12 ? "PM" : "AM";

    if (hours > 12) {
        hours -= 12;
    }

    if (hours === 0) {
        hours = 12;
    }

    return `${hours}:${String(mins).padStart(2, "0")} ${suffix}`;
}




document.addEventListener(
    "DOMContentLoaded",
    initializeScheduler
);



document.body.addEventListener(
    "htmx:afterSwap",
    initializeScheduler
);
