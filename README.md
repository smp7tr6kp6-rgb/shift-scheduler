# Shift-Scheduler

A simple web app for scheduling weekly work shifts. Students pick their availability on a calendar-style grid, and an admin approves or rejects each request. 

## What it does

- Students can:
- Drag across a Monday-Friday, 8am-6pm grid to select hours they want to work
- See their daily and weekly totals update live as they select time.
- Submit their schedule for approval
- See whether their schedule is Pending, Approved, or Rejected.
- If rejected, see the admin's comment, make changes, and resubmit.

  Admins can:
  - See a list of everyone's submitted schedules
  - Approve a schedule, or reject it with an optional comment explaining why.
  - Reset an approved schedule if it needs to change later.
 
  Log in as 'student1' to submit a schedule, then log in as 'admin1' to approve or reject it.

  ## Running it

  '''bash
  go run .
  '''

  Then open 'http://localhost:4000' in a browser. Make sure to clear cache after use to reset. Possible issues may occur otherwise.
