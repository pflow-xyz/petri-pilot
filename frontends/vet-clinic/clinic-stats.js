/**
 * <clinic-stats> Web Component — utilization display
 *
 * Shows room utilization bars, staff utilization, throughput, bottleneck indicator.
 * Updated each frame from ODE state.
 */

const ROOM_RESOURCES = [
    { id: 'exam_room_free', label: 'Exam Rooms', max: 4, color: '#9b59b6' },
    { id: 'surgery_free', label: 'Surgery', max: 1, color: '#e74c3c' },
    { id: 'dental_free', label: 'Dental', max: 1, color: '#e74c3c' },
    { id: 'radiology_free', label: 'Radiology', max: 1, color: '#2ea44f' },
    { id: 'treatment_free', label: 'Treatment', max: 3, color: '#f39c12' },
    { id: 'recovery_free', label: 'Recovery', max: 6, color: '#1abc9c' },
    { id: 'lab_free', label: 'Lab', max: 1, color: '#2ea44f' },
];

const STAFF_RESOURCES = [
    { id: 'dvm_avail', label: 'DVMs', max: 2, color: '#e74c3c' },
    { id: 'rvt_avail', label: 'RVTs', max: 3, color: '#3498db' },
    { id: 'receptionist_avail', label: 'Reception', max: 1, color: '#f39c12' },
];

const QUEUE_PLACES = ['wait_exam','wait_tech','wait_surgery','wait_dental','wait_diag'];

class ClinicStats extends HTMLElement {
    constructor() {
        super();
        this._staffMax = { dvm_avail: 2, rvt_avail: 3, receptionist_avail: 1 };
        this._roomMax = {};
        this._staffTimeline = null;
        ROOM_RESOURCES.forEach(r => this._roomMax[r.id] = r.max);
    }

    connectedCallback() {
        this.render();
    }

    /** Update max values when schedule changes */
    setStaffLevels(dvmCount, rvtCount, receptionistCount) {
        this._staffMax.dvm_avail = dvmCount;
        this._staffMax.rvt_avail = rvtCount;
        this._staffMax.receptionist_avail = receptionistCount;
    }

    /** Set staff timeline for time-varying utilization */
    setStaffTimeline(timeline) {
        this._staffTimeline = timeline;
    }

    /** Look up staff levels at a given ODE time from the timeline */
    _getStaffAtTime(time) {
        if (!this._staffTimeline || this._staffTimeline.length === 0) return null;
        let entry = this._staffTimeline[0];
        for (const e of this._staffTimeline) {
            if (e.time <= time) entry = e;
            else break;
        }
        return entry;
    }

    render() {
        const allResources = [
            ...STAFF_RESOURCES.map(r => ({ ...r, type: 'staff' })),
            ...ROOM_RESOURCES.map(r => ({ ...r, type: 'room' })),
        ];

        this.innerHTML = `
        <div class="clinic-stats card">
            <h3>Utilization</h3>
            <div class="staff-on-duty" id="cs-staff-on-duty"></div>
            ${allResources.map(r => `
                <div class="util-row">
                    <div class="util-label">
                        <span>${r.label}</span>
                        <span id="cs-val-${r.id}">0%</span>
                    </div>
                    <div class="util-bar-bg">
                        <div class="util-bar" id="cs-bar-${r.id}" style="width:0%;background:${r.color};"></div>
                    </div>
                </div>
            `).join('')}

            <div class="throughput">
                <div>
                    <div class="throughput-value" id="cs-throughput">0.0</div>
                    <div class="throughput-label">patients discharged</div>
                </div>
                <div>
                    <div class="throughput-value" id="cs-queued" style="color:var(--accent-orange)">0.0</div>
                    <div class="throughput-label">total queued</div>
                </div>
            </div>

            <div class="bottleneck ok" id="cs-bottleneck">
                <strong>No bottleneck</strong> — clinic is flowing smoothly
            </div>
        </div>`;
    }

    update(state, time) {
        if (!state) return;

        // Update staff-on-duty indicator if timeline exists
        const timelineEntry = this._getStaffAtTime(time);
        const dutyEl = this.querySelector('#cs-staff-on-duty');
        if (dutyEl && timelineEntry) {
            dutyEl.innerHTML = `
                <div class="staff-on-duty-item"><span class="staff-duty-dot" style="background:#e74c3c"></span> <span class="staff-duty-count">${timelineEntry.dvm_avail}</span> DVM</div>
                <div class="staff-on-duty-item"><span class="staff-duty-dot" style="background:#3498db"></span> <span class="staff-duty-count">${timelineEntry.rvt_avail}</span> RVT</div>
                <div class="staff-on-duty-item"><span class="staff-duty-dot" style="background:#f39c12"></span> <span class="staff-duty-count">${timelineEntry.receptionist_avail}</span> Recep</div>`;
        }

        // Use time-varying max if timeline available
        if (timelineEntry) {
            this._staffMax.dvm_avail = timelineEntry.dvm_avail;
            this._staffMax.rvt_avail = timelineEntry.rvt_avail;
            this._staffMax.receptionist_avail = timelineEntry.receptionist_avail;
        }

        // Update utilization bars
        const all = [...STAFF_RESOURCES, ...ROOM_RESOURCES];
        for (const r of all) {
            const max = (r.type === 'staff' ? this._staffMax[r.id] : null) || r.max;
            const avail = Math.max(0, state[r.id] || 0);
            const util = max > 0 ? Math.max(0, 1 - avail / max) : 0;
            const pct = Math.min(100, util * 100);

            const bar = this.querySelector(`#cs-bar-${r.id}`);
            const val = this.querySelector(`#cs-val-${r.id}`);
            if (bar) bar.style.width = `${pct.toFixed(1)}%`;
            if (val) val.textContent = `${pct.toFixed(0)}%`;
        }

        // Throughput
        const discharged = Math.max(0, state.discharged || 0);
        const throughputEl = this.querySelector('#cs-throughput');
        if (throughputEl) throughputEl.textContent = discharged.toFixed(1);

        // Total queued
        const totalQueued = QUEUE_PLACES.reduce((s, p) => s + Math.max(0, state[p] || 0), 0);
        const queuedEl = this.querySelector('#cs-queued');
        if (queuedEl) queuedEl.textContent = totalQueued.toFixed(1);

        // Bottleneck detection
        const bnEl = this.querySelector('#cs-bottleneck');
        if (bnEl) {
            // Find the queue that's growing fastest (highest value)
            let worstQueue = null;
            let worstVal = 2; // threshold for "bottleneck"
            for (const qp of QUEUE_PLACES) {
                const v = state[qp] || 0;
                if (v > worstVal) {
                    worstVal = v;
                    worstQueue = qp;
                }
            }

            if (worstQueue) {
                const name = worstQueue.replace('wait_', '');
                bnEl.className = 'bottleneck';
                bnEl.innerHTML = `<strong>Bottleneck: ${name}</strong> — ${worstVal.toFixed(1)} patients queued`;
            } else {
                bnEl.className = 'bottleneck ok';
                bnEl.innerHTML = `<strong>No bottleneck</strong> — clinic is flowing smoothly`;
            }
        }
    }
}

customElements.define('clinic-stats', ClinicStats);
export { ClinicStats };
