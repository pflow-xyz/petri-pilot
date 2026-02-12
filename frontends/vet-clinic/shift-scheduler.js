/**
 * <shift-scheduler> Web Component — weekly schedule grid
 *
 * Displays employee shifts on a time grid with drag-to-create and resize.
 * Dispatches 'schedule-updated' events with full shift data.
 */

import { autoPlaceBreaks, validateShift, validateWeeklyHours, effectiveHours } from './schedule-rules.js';
import { ROLE_ORDER, ROLE_COLORS } from './employee-registry.js';

const DAYS = ['Mon','Tue','Wed','Thu','Fri','Sat'];
const GRID_START = 6;   // 6:00 AM
const GRID_END = 20;    // 8:00 PM
const SLOTS = (GRID_END - GRID_START) * 2; // 28 half-hour slots
const SLOT_WIDTH = 32;  // px per 30-min slot

function hourToSlot(hour) { return (hour - GRID_START) * 2; }
function slotToHour(slot) { return GRID_START + slot / 2; }

function formatHour(h) {
    const hr = Math.floor(h);
    const min = Math.round((h - hr) * 60);
    const ampm = hr >= 12 ? 'PM' : 'AM';
    const h12 = hr > 12 ? hr - 12 : (hr === 0 ? 12 : hr);
    return `${h12}:${min.toString().padStart(2,'0')} ${ampm}`;
}

/**
 * Generate default shifts for a given day + employee list.
 */
function generateDefaultShifts(employees, day) {
    const shifts = [];
    for (const emp of employees) {
        if (!emp.workDays.includes(day)) continue;

        let startHour, endHour;
        if (day === 'Sat') {
            startHour = 8; endHour = 14; // Saturday skeleton crew
        } else if (emp.employment === 'FT') {
            startHour = 8; endHour = 16.5; // 8:00 - 4:30 (8.5 hr with 30 min lunch)
        } else {
            // PT employees get varied start times
            if (emp.workDays.indexOf(day) % 2 === 0) {
                startHour = 8; endHour = 13; // morning
            } else {
                startHour = 10; endHour = 15; // mid-day
            }
        }

        const shift = autoPlaceBreaks({ employeeId: emp.id, day, startHour, endHour, breaks: [] });
        shifts.push(shift);
    }
    return shifts;
}

class ShiftScheduler extends HTMLElement {
    constructor() {
        super();
        this._employees = [];
        this._shifts = {};       // { day: shift[] }
        this._activeDay = 'Mon';
        this._dragState = null;
        this._initialized = false;
    }

    connectedCallback() {
        this.render();
    }

    get activeDay() { return this._activeDay; }
    set activeDay(day) {
        if (DAYS.includes(day)) {
            this._activeDay = day;
            this.render();
        }
    }

    get shifts() { return this._shifts; }
    get allShiftsFlat() {
        return Object.values(this._shifts).flat();
    }

    setEmployees(employees) {
        this._employees = employees;
        // Generate defaults for any day that doesn't have shifts yet
        for (const day of DAYS) {
            if (!this._shifts[day]) {
                this._shifts[day] = generateDefaultShifts(employees, day);
            } else {
                // Remove shifts for deleted employees
                const empIds = new Set(employees.map(e => e.id));
                this._shifts[day] = this._shifts[day].filter(s => empIds.has(s.employeeId));
            }
        }
        if (!this._initialized) {
            this._initialized = true;
        }
        this.render();
    }

    render() {
        const dayShifts = this._shifts[this._activeDay] || [];
        const empMap = {};
        for (const emp of this._employees) empMap[emp.id] = emp;

        // Group employees by role
        const grouped = {};
        for (const role of ROLE_ORDER) grouped[role] = [];
        for (const emp of this._employees) {
            if (grouped[emp.role]) grouped[emp.role].push(emp);
        }

        // Time header labels
        const timeHeaders = [];
        for (let h = GRID_START; h < GRID_END; h++) {
            timeHeaders.push(formatHour(h));
        }

        // Weekly hours summary
        const weeklyHours = {};
        for (const emp of this._employees) {
            let total = 0;
            for (const day of DAYS) {
                for (const s of (this._shifts[day] || [])) {
                    if (s.employeeId === emp.id) total += effectiveHours(s);
                }
            }
            weeklyHours[emp.id] = total;
        }

        let html = `<div class="shift-scheduler card">
            <h3>Schedule</h3>
            <div class="ss-day-tabs">
                ${DAYS.map(d => `<button class="ss-day-tab${d === this._activeDay ? ' active' : ''}" data-day="${d}">${d}</button>`).join('')}
            </div>
            <div class="ss-grid-wrapper">
                <div class="ss-grid" style="--slot-count:${SLOTS};--slot-width:${SLOT_WIDTH}px;">
                    <!-- Time header row -->
                    <div class="ss-row ss-header-row">
                        <div class="ss-name-cell ss-corner">Employee</div>
                        <div class="ss-time-cells">
                            ${timeHeaders.map((label, i) => `<div class="ss-time-header" style="left:${i * 2 * SLOT_WIDTH}px;width:${SLOT_WIDTH * 2}px">${label}</div>`).join('')}
                        </div>
                    </div>`;

        for (const role of ROLE_ORDER) {
            const emps = grouped[role];
            if (emps.length === 0) continue;

            html += `<div class="ss-role-divider">
                <span class="ss-role-label" style="color:${ROLE_COLORS[role]}">${role}s</span>
            </div>`;

            for (const emp of emps) {
                const empShifts = dayShifts.filter(s => s.employeeId === emp.id);
                const worksToday = emp.workDays.includes(this._activeDay);
                const wh = weeklyHours[emp.id] || 0;
                const overtime = wh > 40;
                const excessive = wh > 48;

                html += `<div class="ss-row ${worksToday ? '' : 'ss-off-day'}" data-emp-id="${emp.id}">
                    <div class="ss-name-cell">
                        <span class="ss-emp-color" style="background:${emp.color}"></span>
                        <span class="ss-emp-name">${emp.name}</span>
                        <span class="ss-weekly-hrs ${excessive ? 'excessive' : overtime ? 'overtime' : ''}">${wh.toFixed(1)}h</span>
                    </div>
                    <div class="ss-time-cells" data-emp-id="${emp.id}">`;

                // Grid lines (every 30 min)
                for (let s = 0; s < SLOTS; s++) {
                    const hour = slotToHour(s);
                    const isHour = hour === Math.floor(hour);
                    html += `<div class="ss-cell${isHour ? ' ss-hour-line' : ''}" data-slot="${s}" style="left:${s * SLOT_WIDTH}px;width:${SLOT_WIDTH}px"></div>`;
                }

                // Shift blocks
                for (const shift of empShifts) {
                    const left = hourToSlot(shift.startHour) * SLOT_WIDTH;
                    const width = (shift.endHour - shift.startHour) * 2 * SLOT_WIDTH;
                    const validation = validateShift(shift);
                    const hasViolation = validation.violations.length > 0;

                    html += `<div class="ss-shift-block${hasViolation ? ' ss-violation' : ''}"
                                  data-emp-id="${emp.id}"
                                  style="left:${left}px;width:${width}px;background:${emp.color}"
                                  title="${formatHour(shift.startHour)} - ${formatHour(shift.endHour)}${hasViolation ? '\n' + validation.violations.join('\n') : ''}">
                        <span class="ss-shift-time">${formatHour(shift.startHour)} - ${formatHour(shift.endHour)}</span>
                        <div class="ss-drag-handle ss-drag-left" data-emp-id="${emp.id}" data-edge="start"></div>
                        <div class="ss-drag-handle ss-drag-right" data-emp-id="${emp.id}" data-edge="end"></div>`;

                    // Break overlays
                    for (const brk of (shift.breaks || [])) {
                        const brkLeft = (brk.startHour - shift.startHour) * 2 * SLOT_WIDTH;
                        const brkWidth = (brk.endHour - brk.startHour) * 2 * SLOT_WIDTH;
                        html += `<div class="ss-break-overlay ss-break-${brk.type}" style="left:${brkLeft}px;width:${Math.max(4, brkWidth)}px" title="${brk.type === 'lunch' ? 'Lunch' : 'Rest'} ${formatHour(brk.startHour)}-${formatHour(brk.endHour)}"></div>`;
                    }

                    html += `</div>`;
                }

                html += `</div></div>`;
            }
        }

        html += `</div></div></div>`;
        this.innerHTML = html;
        this._bindEvents();
    }

    _bindEvents() {
        // Day tabs
        this.querySelectorAll('.ss-day-tab').forEach(btn => {
            btn.addEventListener('click', () => {
                this._activeDay = btn.dataset.day;
                this.render();
                this._emit();
            });
        });

        // Click on empty time cells to create a shift
        this.querySelectorAll('.ss-time-cells').forEach(cellContainer => {
            const empId = cellContainer.dataset.empId;
            if (!empId) return;

            cellContainer.addEventListener('mousedown', (e) => {
                // Don't start drag if clicking on an existing shift
                if (e.target.closest('.ss-shift-block')) return;

                const cell = e.target.closest('.ss-cell');
                if (!cell) return;

                const slot = parseInt(cell.dataset.slot);
                this._dragState = {
                    type: 'create',
                    empId,
                    startSlot: slot,
                    currentSlot: slot,
                    rect: cellContainer.getBoundingClientRect(),
                };
                e.preventDefault();
            });
        });

        // Drag handles on existing shifts
        this.querySelectorAll('.ss-drag-handle').forEach(handle => {
            handle.addEventListener('mousedown', (e) => {
                e.stopPropagation();
                e.preventDefault();
                const empId = handle.dataset.empId;
                const edge = handle.dataset.edge;
                const shiftBlock = handle.closest('.ss-shift-block');
                const cellContainer = shiftBlock.closest('.ss-time-cells');

                // Find the shift
                const dayShifts = this._shifts[this._activeDay] || [];
                const shift = dayShifts.find(s => s.employeeId === empId);
                if (!shift) return;

                this._dragState = {
                    type: 'resize',
                    empId,
                    edge,
                    originalStart: shift.startHour,
                    originalEnd: shift.endHour,
                    rect: cellContainer.getBoundingClientRect(),
                };
            });
        });

        // Global mouse handlers for drag
        this._onMouseMove = (e) => {
            if (!this._dragState) return;
            const ds = this._dragState;
            const x = e.clientX - ds.rect.left;
            const slot = Math.max(0, Math.min(SLOTS - 1, Math.round(x / SLOT_WIDTH)));
            const hour = slotToHour(slot);

            if (ds.type === 'create') {
                ds.currentSlot = slot;
            } else if (ds.type === 'resize') {
                if (ds.edge === 'start') {
                    ds.newStart = Math.min(hour, ds.originalEnd - 0.5);
                } else {
                    ds.newEnd = Math.max(hour, ds.originalStart + 0.5);
                }
            }
        };

        this._onMouseUp = (e) => {
            if (!this._dragState) return;
            const ds = this._dragState;

            if (ds.type === 'create') {
                const startSlot = Math.min(ds.startSlot, ds.currentSlot);
                const endSlot = Math.max(ds.startSlot, ds.currentSlot) + 1;
                const startHour = slotToHour(startSlot);
                const endHour = slotToHour(endSlot);

                if (endHour - startHour >= 0.5) {
                    const shift = autoPlaceBreaks({
                        employeeId: ds.empId,
                        day: this._activeDay,
                        startHour,
                        endHour,
                        breaks: [],
                    });

                    if (!this._shifts[this._activeDay]) this._shifts[this._activeDay] = [];
                    // Remove existing shift for this employee on this day
                    this._shifts[this._activeDay] = this._shifts[this._activeDay].filter(s => s.employeeId !== ds.empId);
                    this._shifts[this._activeDay].push(shift);
                    this.render();
                    this._emit();
                }
            } else if (ds.type === 'resize') {
                const dayShifts = this._shifts[this._activeDay] || [];
                const shift = dayShifts.find(s => s.employeeId === ds.empId);
                if (shift) {
                    if (ds.edge === 'start' && ds.newStart !== undefined) {
                        shift.startHour = ds.newStart;
                    } else if (ds.edge === 'end' && ds.newEnd !== undefined) {
                        shift.endHour = ds.newEnd;
                    }
                    // Re-place breaks
                    const updated = autoPlaceBreaks({ ...shift, breaks: [] });
                    Object.assign(shift, updated);
                    this.render();
                    this._emit();
                }
            }

            this._dragState = null;
        };

        document.addEventListener('mousemove', this._onMouseMove);
        document.addEventListener('mouseup', this._onMouseUp);
    }

    disconnectedCallback() {
        if (this._onMouseMove) document.removeEventListener('mousemove', this._onMouseMove);
        if (this._onMouseUp) document.removeEventListener('mouseup', this._onMouseUp);
    }

    _emit() {
        this.dispatchEvent(new CustomEvent('schedule-updated', {
            detail: {
                day: this._activeDay,
                shifts: this._shifts,
                dayShifts: this._shifts[this._activeDay] || [],
            },
            bubbles: true,
        }));
    }
}

customElements.define('shift-scheduler', ShiftScheduler);
export { ShiftScheduler, generateDefaultShifts, DAYS };
