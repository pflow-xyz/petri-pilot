/**
 * schedule-rules.js — US labor law break rules + staff timeline builder
 *
 * Pure data module, no DOM. All times are in decimal hours (e.g. 8.5 = 8:30 AM).
 */

// ── US Labor Law Break Rules ──

/**
 * Auto-place breaks based on shift duration.
 * Lunch (30min unpaid) near midpoint; rest breaks (10min paid) at ~2hr and ~6hr marks.
 * Returns new shift object with breaks array populated.
 */
export function autoPlaceBreaks(shift) {
    const duration = shift.endHour - shift.startHour;
    const breaks = [];

    if (duration >= 5) {
        // Lunch break (30 min unpaid) near midpoint
        const mid = shift.startHour + duration / 2;
        const lunchStart = Math.round(mid * 2) / 2; // snap to 30min
        breaks.push({ type: 'lunch', startHour: lunchStart, endHour: lunchStart + 0.5 });
    }

    if (duration >= 4 && duration < 8) {
        // 1 rest break (10 min paid) ~2hr into shift
        const restStart = shift.startHour + 2;
        breaks.push({ type: 'rest', startHour: restStart, endHour: restStart + 10/60 });
    } else if (duration >= 8) {
        // 2 rest breaks at ~2hr and ~6hr marks
        const rest1 = shift.startHour + 2;
        const rest2 = shift.startHour + 6;
        breaks.push({ type: 'rest', startHour: rest1, endHour: rest1 + 10/60 });
        breaks.push({ type: 'rest', startHour: rest2, endHour: rest2 + 10/60 });
    }

    return { ...shift, breaks };
}

/**
 * Validate a single shift for labor law compliance.
 * Returns { violations: string[] }
 */
export function validateShift(shift) {
    const violations = [];
    const duration = shift.endHour - shift.startHour;

    if (duration > 12) {
        violations.push('Shift exceeds 12 hours maximum');
    }
    if (duration < 0) {
        violations.push('Shift end is before start');
    }

    // Lunch required for shifts >= 5 hours
    if (duration >= 5) {
        const hasLunch = (shift.breaks || []).some(b => b.type === 'lunch');
        if (!hasLunch) {
            violations.push('Lunch break required for shifts >= 5 hours');
        }
    }

    // Rest breaks: 1 for 4-8hr, 2 for 8+hr
    const restBreaks = (shift.breaks || []).filter(b => b.type === 'rest');
    if (duration >= 4 && duration < 8 && restBreaks.length < 1) {
        violations.push('At least 1 rest break required for shifts 4-8 hours');
    }
    if (duration >= 8 && restBreaks.length < 2) {
        violations.push('At least 2 rest breaks required for shifts 8+ hours');
    }

    return { violations };
}

/**
 * Validate weekly hours for an employee.
 * Returns { totalHours, overtime, violations: string[] }
 */
export function validateWeeklyHours(employeeId, weekShifts, employment) {
    const violations = [];
    let totalHours = 0;

    for (const shift of weekShifts) {
        if (shift.employeeId === employeeId) {
            totalHours += effectiveHours(shift);
        }
    }

    const overtime = Math.max(0, totalHours - 40);

    if (employment === 'FT' && totalHours < 32) {
        violations.push(`Full-time employee has only ${totalHours.toFixed(1)} hrs (expected >= 32)`);
    }
    if (totalHours > 40) {
        violations.push(`Overtime: ${overtime.toFixed(1)} hrs over 40-hr threshold`);
    }
    if (totalHours > 48) {
        violations.push(`Excessive hours: ${totalHours.toFixed(1)} hrs exceeds 48-hr limit`);
    }

    return { totalHours, overtime, violations };
}

/**
 * Effective paid hours (shift duration minus unpaid lunch).
 */
export function effectiveHours(shift) {
    const duration = shift.endHour - shift.startHour;
    const unpaid = (shift.breaks || [])
        .filter(b => b.type === 'lunch')
        .reduce((s, b) => s + (b.endHour - b.startHour), 0);
    return duration - unpaid;
}

/**
 * Build a staff timeline from shifts and employees.
 * Returns sorted array: [{ time, dvm_avail, rvt_avail, receptionist_avail }]
 * where time is ODE-hours (0 = 8AM, 10 = 6PM).
 *
 * @param {Array} shifts - All shifts for the selected day
 * @param {Array} employees - Employee roster with { id, role }
 * @returns {Array} Timeline of staff level changes
 */
export function buildStaffTimeline(shifts, employees) {
    const roleMap = {};
    for (const emp of employees) {
        roleMap[emp.id] = emp.role;
    }

    // Collect all boundary events
    const events = [];
    for (const shift of shifts) {
        const role = roleMap[shift.employeeId];
        if (!role) continue;

        events.push({ time: shift.startHour, role, delta: +1 });
        events.push({ time: shift.endHour, role, delta: -1 });

        // Breaks reduce availability
        for (const brk of (shift.breaks || [])) {
            events.push({ time: brk.startHour, role, delta: -1 });
            events.push({ time: brk.endHour, role, delta: +1 });
        }
    }

    // Sort by time (stable: +1 before -1 at same time for shift start before break end)
    events.sort((a, b) => a.time - b.time || b.delta - a.delta);

    // Walk events to build timeline
    const counts = { DVM: 0, RVT: 0, Receptionist: 0 };
    const timeline = [{ time: 0, dvm_avail: 0, rvt_avail: 0, receptionist_avail: 0 }];

    for (const ev of events) {
        counts[ev.role] = Math.max(0, (counts[ev.role] || 0) + ev.delta);
        const odeTime = ev.time - 8; // convert clock hour to ODE time (8AM = 0)
        if (odeTime < 0 || odeTime > 8) continue;
        timeline.push({
            time: odeTime,
            dvm_avail: counts.DVM,
            rvt_avail: counts.RVT,
            receptionist_avail: counts.Receptionist,
        });
    }

    // Deduplicate same-time entries (keep last)
    const deduped = [];
    for (let i = 0; i < timeline.length; i++) {
        if (i < timeline.length - 1 && timeline[i].time === timeline[i+1].time) continue;
        deduped.push(timeline[i]);
    }

    // Ensure we end at t=8
    const last = deduped[deduped.length - 1];
    if (last.time < 8) {
        deduped.push({ time: 8, dvm_avail: 0, rvt_avail: 0, receptionist_avail: 0 });
    }

    return deduped;
}
