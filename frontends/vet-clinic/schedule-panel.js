/**
 * <schedule-panel> Web Component — staff, day, and service mix configuration
 *
 * Dispatches 'schedule-change' events with the full config object.
 */

const DAYS = ['Mon','Tue','Wed','Thu','Fri','Sat'];
const SURGERY_DAYS = { Mon: true, Wed: true }; // Default surgery schedule

// ── Day-type presets ──
// Target: each DVM generates $5-7K on a busy day
const PRESETS = {
    slow: {
        label: 'Slow',
        desc: '~$3K/DVM',
        arrivalRate: 3,
        surgeryDay: false,
        mix: { exam: 0.50, tech: 0.30, surgery: 0, dental: 0, diag: 0.20 },
    },
    normal: {
        label: 'Normal',
        desc: '~$4-5K/DVM',
        arrivalRate: 5,
        surgeryDay: false,
        mix: { exam: 0.45, tech: 0.25, surgery: 0, dental: 0, diag: 0.30 },
    },
    busy: {
        label: 'Busy',
        desc: '~$5-6K/DVM',
        arrivalRate: 7,
        surgeryDay: true,
        mix: { exam: 0.40, tech: 0.20, surgery: 0.15, dental: 0.08, diag: 0.17 },
    },
    surgery: {
        label: 'Surgery',
        desc: '~$6-7K/DVM',
        arrivalRate: 8,
        surgeryDay: true,
        mix: { exam: 0.25, tech: 0.05, surgery: 0.25, dental: 0.15, diag: 0.30 },
    },
};

const DEFAULT_CONFIG = {
    day: 'Tue',
    surgeryDay: false,
    dvmCount: 2,
    rvtCount: 3,
    receptionistCount: 1,
    arrivalRate: 5, // patients per hour
    preset: 'normal',
    mix: {
        exam: 0.45,
        tech: 0.25,
        surgery: 0,
        dental: 0,
        diag: 0.30,
    }
};

class SchedulePanel extends HTMLElement {
    constructor() {
        super();
        this._config = { ...DEFAULT_CONFIG, mix: { ...DEFAULT_CONFIG.mix } };
        this._scheduleDriven = false;
        this._scheduleStaffNote = '';
    }

    connectedCallback() {
        this.render();
        this._bindEvents();
    }

    get config() { return this._config; }

    /**
     * Set staff levels from schedule timeline. Disables manual sliders.
     * @param {{ dvm: number, rvt: number, receptionist: number }} peak - peak staff counts
     */
    setStaffFromSchedule(peak) {
        this._scheduleDriven = true;
        this._config.dvmCount = peak.dvm;
        this._config.rvtCount = peak.rvt;
        this._config.receptionistCount = peak.receptionist;
        this._scheduleStaffNote = `${peak.dvm} DVMs, ${peak.rvt} RVTs, ${peak.receptionist} Recep on shift`;
        this.render();
        this._bindEvents();
    }

    /** Re-enable manual sliders (fallback mode). */
    enableManualStaffing() {
        this._scheduleDriven = false;
        this._scheduleStaffNote = '';
        this.render();
        this._bindEvents();
    }

    render() {
        const c = this._config;
        this.innerHTML = `
        <div class="schedule-panel card">
            <h3>Schedule</h3>

            <div class="section-label">Day Type</div>
            <div class="preset-picker">
                ${Object.entries(PRESETS).map(([k, p]) =>
                    `<button class="preset-btn${c.preset === k ? ' active' : ''}" data-preset="${k}" title="${p.desc}">
                        <span class="preset-label">${p.label}</span>
                        <span class="preset-desc">${p.desc}</span>
                    </button>`
                ).join('')}
            </div>

            <div class="section-label">Day</div>
            <div class="day-picker">
                ${DAYS.map(d => `<button class="day-btn${d === c.day ? ' active' : ''}${SURGERY_DAYS[d] ? ' surgery' : ''}" data-day="${d}">${d}</button>`).join('')}
            </div>

            <div class="toggle-row">
                <span>Surgery Day</span>
                <label class="toggle-switch">
                    <input type="checkbox" id="sp-surgery" ${c.surgeryDay ? 'checked' : ''}>
                    <span class="toggle-slider"></span>
                </label>
            </div>

            <div class="section-label">Staff</div>
            ${this._scheduleDriven ? `<div class="staff-schedule-note">${this._scheduleStaffNote}</div>` : ''}
            <label>
                <span>DVMs</span>
                <span id="sp-dvm-val">${c.dvmCount}</span>
            </label>
            <input type="range" id="sp-dvm" min="1" max="3" step="1" value="${c.dvmCount}" ${this._scheduleDriven ? 'disabled' : ''}>

            <label>
                <span>RVTs</span>
                <span id="sp-rvt-val">${c.rvtCount}</span>
            </label>
            <input type="range" id="sp-rvt" min="1" max="5" step="1" value="${c.rvtCount}" ${this._scheduleDriven ? 'disabled' : ''}>

            <label>
                <span>Receptionists</span>
                <span id="sp-recep-val">${c.receptionistCount}</span>
            </label>
            <input type="range" id="sp-recep" min="1" max="2" step="1" value="${c.receptionistCount}" ${this._scheduleDriven ? 'disabled' : ''}>

            <div class="section-label">Patient Arrival</div>
            <label>
                <span>Arrivals/hr</span>
                <span id="sp-arr-val">${c.arrivalRate}</span>
            </label>
            <input type="range" id="sp-arrival" min="1" max="15" step="0.5" value="${c.arrivalRate}">

            <div class="section-label">Service Mix</div>
            <div class="mix-weights">
                <label>Exam <span class="mix-value" id="sp-mix-exam-val">${(c.mix.exam*100).toFixed(0)}%</span>
                    <input type="range" id="sp-mix-exam" min="0" max="100" step="5" value="${c.mix.exam*100}">
                </label>
                <label>Tech <span class="mix-value" id="sp-mix-tech-val">${(c.mix.tech*100).toFixed(0)}%</span>
                    <input type="range" id="sp-mix-tech" min="0" max="100" step="5" value="${c.mix.tech*100}">
                </label>
                <label>Surgery <span class="mix-value" id="sp-mix-surgery-val">${(c.mix.surgery*100).toFixed(0)}%</span>
                    <input type="range" id="sp-mix-surgery" min="0" max="100" step="5" value="${c.mix.surgery*100}">
                </label>
                <label>Dental <span class="mix-value" id="sp-mix-dental-val">${(c.mix.dental*100).toFixed(0)}%</span>
                    <input type="range" id="sp-mix-dental" min="0" max="100" step="5" value="${c.mix.dental*100}">
                </label>
                <label>Diag <span class="mix-value" id="sp-mix-diag-val">${(c.mix.diag*100).toFixed(0)}%</span>
                    <input type="range" id="sp-mix-diag" min="0" max="100" step="5" value="${c.mix.diag*100}">
                </label>
            </div>
        </div>`;
    }

    _bindEvents() {
        // Preset picker
        this.querySelectorAll('.preset-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                const key = btn.dataset.preset;
                const preset = PRESETS[key];
                if (!preset) return;
                this._config.preset = key;
                this._config.arrivalRate = preset.arrivalRate;
                this._config.surgeryDay = preset.surgeryDay;
                this._config.mix = { ...preset.mix };
                // Re-render to update all controls
                this.render();
                this._bindEvents();
                this._emit();
            });
        });

        // Day picker
        this.querySelectorAll('.day-btn').forEach(btn => {
            btn.addEventListener('click', () => {
                this._config.day = btn.dataset.day;
                this._config.surgeryDay = !!SURGERY_DAYS[btn.dataset.day];
                this.querySelector('#sp-surgery').checked = this._config.surgeryDay;
                this.querySelectorAll('.day-btn').forEach(b => b.classList.toggle('active', b.dataset.day === this._config.day));
                this._emit();
            });
        });

        // Surgery toggle
        this.querySelector('#sp-surgery').addEventListener('change', (e) => {
            this._config.surgeryDay = e.target.checked;
            this._emit();
        });

        // Staff sliders
        this._bindSlider('sp-dvm', 'dvmCount', 'sp-dvm-val');
        this._bindSlider('sp-rvt', 'rvtCount', 'sp-rvt-val');
        this._bindSlider('sp-recep', 'receptionistCount', 'sp-recep-val');
        this._bindSlider('sp-arrival', 'arrivalRate', 'sp-arr-val');

        // Mix sliders — normalize on change
        const mixKeys = ['exam','tech','surgery','dental','diag'];
        for (const key of mixKeys) {
            const slider = this.querySelector(`#sp-mix-${key}`);
            slider.addEventListener('input', () => {
                this._config.mix[key] = parseInt(slider.value) / 100;
                this._normalizeMix(key);
                this._updateMixDisplay();
                this._clearPreset();
                this._emit();
            });
        }
    }

    _bindSlider(sliderId, configKey, valueId) {
        const slider = this.querySelector(`#${sliderId}`);
        slider.addEventListener('input', () => {
            this._config[configKey] = parseFloat(slider.value);
            const valEl = this.querySelector(`#${valueId}`);
            if (valEl) valEl.textContent = this._config[configKey];
            this._clearPreset();
            this._emit();
        });
    }

    _clearPreset() {
        this._config.preset = null;
        this.querySelectorAll('.preset-btn').forEach(b => b.classList.remove('active'));
    }

    _normalizeMix(changedKey) {
        const keys = ['exam','tech','surgery','dental','diag'];
        const total = keys.reduce((s, k) => s + this._config.mix[k], 0);
        if (total <= 0) {
            // Reset to equal
            keys.forEach(k => this._config.mix[k] = 0.2);
            return;
        }
        // Normalize
        keys.forEach(k => this._config.mix[k] = this._config.mix[k] / total);
    }

    _updateMixDisplay() {
        const keys = ['exam','tech','surgery','dental','diag'];
        for (const key of keys) {
            const valEl = this.querySelector(`#sp-mix-${key}-val`);
            const slider = this.querySelector(`#sp-mix-${key}`);
            if (valEl) valEl.textContent = `${(this._config.mix[key]*100).toFixed(0)}%`;
            if (slider) slider.value = (this._config.mix[key]*100).toFixed(0);
        }
    }

    _emit() {
        this.dispatchEvent(new CustomEvent('schedule-change', { detail: { ...this._config, mix: { ...this._config.mix } }, bubbles: true }));
    }
}

customElements.define('schedule-panel', SchedulePanel);
export { SchedulePanel, DEFAULT_CONFIG };
