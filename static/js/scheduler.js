const days = ["Mon", "Tue", "Wed", "Thu", "Fri"];

const START_MINUTE = 8 * 60;
const END_MINUTE = 18 * 60;
const SLOT_LENGTH = 10;

const schedule = {
    Mon: [],
    Tue: [],
    Wed: [],
    Thu: [],
    Fri: []
};

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

    /*
     * Don't create the grid twice.
     */
    if (grid.children.length > 0) {
        return;
    }

    createGrid(grid);
    updateDisplay();
}


function createGrid(grid) {

    for (
        let minute = START_MINUTE;
        minute < END_MINUTE;
        minute += SLOT_LENGTH
    ) {

        /*
         * Time column
         */

        const timeCell = document.createElement("div");

        timeCell.className = "time-cell";

        if (minute % 60 === 0) {
            timeCell.textContent = formatTime(minute);
        }

        grid.appendChild(timeCell);


        /*
         * Five days
         */

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

    let weeklyMinutes = 0;

    for (const day of days) {

        const minutes =
            schedule[day].length *
            SLOT_LENGTH;

        weeklyMinutes += minutes;

        if (minutes > 9 * 60) {

            validation.textContent =
                `${day} exceeds the 9 hour daily limit.`;

            submitButton.disabled = true;

            return;
        }
    }

    if (weeklyMinutes < 20 * 60) {

        validation.textContent =
            "You need at least 20 hours per week.";

        submitButton.disabled = true;

        return;
    }

    if (weeklyMinutes > 40 * 60) {

        validation.textContent =
            "You cannot exceed 40 hours per week.";

        submitButton.disabled = true;

        return;
    }

    validation.textContent = "";

    submitButton.disabled = false;
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


/*
 * Initial page load.
 */

document.addEventListener(
    "DOMContentLoaded",
    initializeScheduler
);


/*
 * HTMX page replacement.
 */

document.body.addEventListener(
    "htmx:afterSwap",
    initializeScheduler
);
