# Maintenance Scheduling Context

This context defines the language used to validate same-day maintenance plans
before publication.

## Language

**Maintenance Window**:
A same-day half-open interval with an inclusive start and exclusive end.
_Avoid_: Slot, booking, appointment

**Boundary Touch**:
Two windows where one ends exactly when the other starts; this is allowed and
is not an overlap.
_Avoid_: Collision, conflict

**Overlap**:
Two windows sharing a positive duration; an overlap makes the plan invalid.
_Avoid_: Boundary touch, adjacent window

**Maintenance Plan**:
An ordered set of windows submitted for validation.
_Avoid_: Schedule database, calendar
