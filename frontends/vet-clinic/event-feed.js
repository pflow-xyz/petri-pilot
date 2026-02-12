/**
 * <event-feed> Web Component — live event log from ODE simulation
 *
 * Watches state changes between time steps and generates human-readable
 * event entries when transition flux is significant. Shows a scrolling
 * feed of clinic activity.
 */

const TRANSITION_LABELS = {
    patient_arrives: ['Patient arrived', 'reception'],
    triage_to_exam: ['Triaged to exam', 'reception'],
    triage_to_tech: ['Triaged to tech', 'reception'],
    triage_to_surgery: ['Triaged to surgery', 'reception'],
    triage_to_dental: ['Triaged to dental', 'reception'],
    triage_to_diag: ['Triaged to diagnostics', 'reception'],
    start_wellness: ['Wellness exam started', 'exam'],
    finish_wellness: ['Wellness exam done', 'exam'],
    start_sick: ['Sick visit started', 'exam'],
    finish_sick: ['Sick visit done', 'exam'],
    start_vaccine: ['Vaccination started', 'exam'],
    finish_vaccine: ['Vaccination done', 'exam'],
    start_nail_trim: ['Nail trim started', 'tech'],
    finish_nail_trim: ['Nail trim done', 'tech'],
    start_weight: ['Weight check started', 'tech'],
    finish_weight: ['Weight check done', 'tech'],
    start_suture: ['Suture removal started', 'tech'],
    finish_suture: ['Suture removal done', 'tech'],
    start_spay: ['Spay surgery started', 'surgery'],
    finish_spay: ['Spay surgery done', 'surgery'],
    start_neuter: ['Neuter surgery started', 'surgery'],
    finish_neuter: ['Neuter surgery done', 'surgery'],
    start_dental: ['Dental cleaning started', 'dental'],
    finish_dental: ['Dental cleaning done', 'dental'],
    start_xray: ['X-ray started', 'diag'],
    finish_xray: ['X-ray done', 'diag'],
    start_bloodwork: ['Blood draw started', 'diag'],
    finish_bloodwork: ['Blood draw done', 'diag'],
    start_lab: ['Lab processing started', 'diag'],
    finish_lab: ['Lab results ready', 'diag'],
    process_checkout: ['Patient checked out', 'reception'],
    finish_recovery: ['Recovery complete', 'recovery'],
};

const CATEGORY_COLORS = {
    reception: '#3498db',
    exam: '#9b59b6',
    tech: '#f39c12',
    surgery: '#e74c3c',
    dental: '#e74c3c',
    diag: '#2ea44f',
    recovery: '#1abc9c',
};

class EventFeed extends HTMLElement {
    constructor() {
        super();
        this._events = [];
        this._prevState = null;
        this._prevTime = 0;
        this._accumulators = {}; // track fractional events per transition
        this._maxEvents = 80;
    }

    connectedCallback() {
        this.render();
    }

    render() {
        this.innerHTML = `
        <div class="event-feed card">
            <h3>Activity Feed</h3>
            <div class="event-feed-list" id="ef-list"></div>
        </div>`;
    }

    /** Reset feed (called on time reset) */
    reset() {
        this._events = [];
        this._prevState = null;
        this._prevTime = 0;
        this._accumulators = {};
        const list = this.querySelector('#ef-list');
        if (list) list.innerHTML = '<div style="text-align:center;color:var(--text-muted);padding:20px;font-size:0.8rem;">Press play to see activity...</div>';
    }

    /**
     * Update with new ODE state. Uses cumulative throughput tracking:
     * we estimate total flow through each transition by integrating the
     * ODE flux (rate * product of input concentrations) over time.
     * When cumulative throughput crosses an integer, we emit an event.
     */
    update(state, time, net) {
        if (!state || !net || time <= this._prevTime) {
            if (time < this._prevTime) this.reset();
            return;
        }

        const dt = time - this._prevTime;
        if (dt <= 0 || !this._prevState) {
            this._prevState = { ...state };
            this._prevTime = time;
            return;
        }

        let anyEvent = false;

        for (const [tId] of net.transitions) {
            const info = TRANSITION_LABELS[tId];
            if (!info) continue;

            // Compute instantaneous flux = rate * product(input_place_values)
            // We approximate using the average of prev and current state (trapezoidal)
            const inputArcs = net.inputArcs(tId);
            let fluxPrev = 1.0;
            let fluxCur = 1.0;

            for (const arc of inputArcs) {
                const w = arc.getWeightSum();
                const pPrev = Math.max(0, this._prevState[arc.source] || 0);
                const pCur = Math.max(0, state[arc.source] || 0);
                // Mass-action: flux *= concentration^weight
                fluxPrev *= Math.pow(pPrev, w);
                fluxCur *= Math.pow(pCur, w);
            }

            // Source transitions (no inputs) have constant flux = rate
            // We don't know the actual rate here, but flux=1 per unit time is a
            // decent proxy. The absolute count won't be perfect, but the event
            // timing/ordering will be right.
            const avgFlux = (fluxPrev + fluxCur) / 2;
            if (avgFlux <= 0.001) continue;

            // Integrate: cumulative throughput += avgFlux * dt
            if (!this._accumulators[tId]) this._accumulators[tId] = 0;
            this._accumulators[tId] += avgFlux * dt;

            while (this._accumulators[tId] >= 1.0) {
                this._accumulators[tId] -= 1.0;
                this._addEvent(time, tId, info[0], info[1]);
                anyEvent = true;
            }
        }

        this._prevState = { ...state };
        this._prevTime = time;
        if (anyEvent) this._renderList();
    }

    _addEvent(time, transId, label, category) {
        const totalMinutes = Math.floor(time * 60);
        const h = 8 + Math.floor(totalMinutes / 60);
        const m = totalMinutes % 60;
        const ampm = h >= 12 ? 'PM' : 'AM';
        const h12 = h > 12 ? h - 12 : h;
        const clock = `${h12}:${m.toString().padStart(2, '0')} ${ampm}`;

        this._events.push({ clock, label, category, time });

        if (this._events.length > this._maxEvents) {
            this._events = this._events.slice(-this._maxEvents);
        }
    }

    _renderList() {
        const list = this.querySelector('#ef-list');
        if (!list) return;

        // Only render the last 30 visible
        const visible = this._events.slice(-30);
        list.innerHTML = visible.map(ev => {
            const color = CATEGORY_COLORS[ev.category] || '#999';
            return `<div class="event-feed-item">
                <span class="ef-time">${ev.clock}</span>
                <span class="ef-dot" style="background:${color}"></span>
                <span class="ef-label">${ev.label}</span>
            </div>`;
        }).join('');

        list.scrollTop = list.scrollHeight;
    }
}

customElements.define('event-feed', EventFeed);
export { EventFeed };
