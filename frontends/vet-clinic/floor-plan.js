/**
 * <floor-plan> Web Component — SVG hospital blueprint with live token counts
 *
 * Rooms are colored rectangles. Token badges show current ODE state.
 * Staff positions shown as small circles (filled = busy, hollow = available).
 */

const ROOMS = [
    // Reception row
    { id: 'reception', x: 10, y: 10, w: 300, h: 70, color: '#3498db', label: 'Reception / Waiting',
      places: ['arrival'], queues: ['wait_exam','wait_tech','wait_surgery','wait_dental','wait_diag'] },
    { id: 'checkout_area', x: 320, y: 10, w: 160, h: 70, color: '#3498db', label: 'Checkout',
      places: ['checkout','discharged'] },

    // Exam rooms
    { id: 'exam1', x: 10, y: 100, w: 110, h: 90, color: '#9b59b6', label: 'Exam 1',
      places: ['in_wellness'] },
    { id: 'exam2', x: 130, y: 100, w: 110, h: 90, color: '#9b59b6', label: 'Exam 2',
      places: ['in_sick'] },
    { id: 'exam3', x: 250, y: 100, w: 110, h: 90, color: '#9b59b6', label: 'Exam 3',
      places: ['in_vaccine'] },
    { id: 'exam4', x: 370, y: 100, w: 110, h: 90, color: '#9b59b6', label: 'Exam 4',
      places: ['in_suture'] },

    // Diagnostics
    { id: 'radiology', x: 490, y: 100, w: 120, h: 90, color: '#2ea44f', label: 'Radiology',
      places: ['in_xray'] },

    // Treatment + Lab
    { id: 'treatment', x: 10, y: 210, w: 350, h: 80, color: '#f39c12', label: 'Treatment Area',
      places: ['in_nail_trim','in_weight','in_bloodwork'] },
    { id: 'lab', x: 490, y: 210, w: 120, h: 80, color: '#2ea44f', label: 'Lab',
      places: ['in_lab'] },

    // Surgery row
    { id: 'surgery', x: 10, y: 310, w: 180, h: 90, color: '#e74c3c', label: 'Surgery Suite',
      places: ['in_spay','in_neuter'] },
    { id: 'dental', x: 200, y: 310, w: 160, h: 90, color: '#e74c3c', label: 'Dental Suite',
      places: ['in_dental'] },
    { id: 'recovery', x: 370, y: 310, w: 240, h: 90, color: '#1abc9c', label: 'Recovery Area',
      places: ['in_recovery'] },
];

const STAFF_GROUPS = [
    { id: 'dvm', place: 'dvm_avail', color: '#e74c3c', label: 'DVM', max: 3, x: 380, y: 200 },
    { id: 'rvt', place: 'rvt_avail', color: '#3498db', label: 'RVT', max: 5, x: 420, y: 200 },
    { id: 'recep', place: 'receptionist_avail', color: '#f39c12', label: 'Rec', max: 2, x: 460, y: 200 },
];

const SVG_W = 620;
const SVG_H = 410;

class FloorPlan extends HTMLElement {
    constructor() {
        super();
        this._state = null;
        this._maxTokens = {}; // track max for queue bar scaling
    }

    connectedCallback() {
        this.render();
    }

    render() {
        let svg = `<svg viewBox="0 0 ${SVG_W} ${SVG_H}" xmlns="http://www.w3.org/2000/svg" style="background:rgba(0,0,0,0.3);border-radius:12px;">`;

        // Draw rooms
        for (const room of ROOMS) {
            svg += `<rect class="room" x="${room.x}" y="${room.y}" width="${room.w}" height="${room.h}" fill="${room.color}" fill-opacity="0.2" rx="6" ry="6"/>`;
            svg += `<text class="room-label" x="${room.x + room.w/2}" y="${room.y + 16}" fill="white" font-size="11" font-weight="600" text-anchor="middle">${room.label}</text>`;

            // Place badges — positioned within each room
            const places = room.places || [];
            const pCount = places.length;
            for (let i = 0; i < pCount; i++) {
                const px = room.x + (room.w / (pCount + 1)) * (i + 1);
                const py = room.y + room.h / 2 + 8;

                // Badge circle background
                svg += `<circle id="badge_bg_${places[i]}" cx="${px}" cy="${py}" r="14" fill="rgba(0,0,0,0.4)" stroke="${room.color}" stroke-width="1.5"/>`;
                // Token count text
                svg += `<text id="badge_${places[i]}" class="token-badge" x="${px}" y="${py}" fill="white" font-size="13" font-weight="700" text-anchor="middle" dominant-baseline="central">0</text>`;
                // Place label underneath
                const shortLabel = places[i].replace(/^in_/, '').replace(/_/g, ' ');
                svg += `<text x="${px}" y="${py + 22}" fill="rgba(255,255,255,0.5)" font-size="8" text-anchor="middle">${shortLabel}</text>`;
            }

            // Queue bars for reception area
            if (room.queues) {
                for (let i = 0; i < room.queues.length; i++) {
                    const qy = room.y + 32 + i * 8;
                    svg += `<rect x="${room.x + 8}" y="${qy}" width="0" height="5" rx="2" fill="${room.color}" fill-opacity="0.6" id="qbar_${room.queues[i]}"/>`;
                    const shortQ = room.queues[i].replace(/^wait_/, '');
                    svg += `<text x="${room.x + 8}" y="${qy - 1}" fill="rgba(255,255,255,0.4)" font-size="7">${shortQ}</text>`;
                }
            }
        }

        // Staff indicators — legend + dots
        for (const sg of STAFF_GROUPS) {
            svg += `<text x="${sg.x}" y="${sg.y - 4}" fill="${sg.color}" font-size="9" font-weight="600" text-anchor="middle">${sg.label}</text>`;
            for (let i = 0; i < sg.max; i++) {
                svg += `<circle id="staff_${sg.id}_${i}" cx="${sg.x - (sg.max-1)*6 + i*12}" cy="${sg.y + 8}" r="5" fill="transparent" stroke="${sg.color}" stroke-width="1.5" class="staff-dot"/>`;
            }
        }

        // Resource room indicators (small badges at bottom-right of relevant rooms)
        const resourceBadges = [
            { place: 'exam_room_free', x: 475, y: 185, color: '#9b59b6', label: 'Exam' },
            { place: 'surgery_free', x: 185, y: 395, color: '#e74c3c', label: 'Surg' },
            { place: 'dental_free', x: 355, y: 395, color: '#e74c3c', label: 'Dent' },
            { place: 'radiology_free', x: 605, y: 185, color: '#2ea44f', label: 'Rad' },
            { place: 'treatment_free', x: 355, y: 285, color: '#f39c12', label: 'Trt' },
            { place: 'recovery_free', x: 605, y: 395, color: '#1abc9c', label: 'Rec' },
            { place: 'lab_free', x: 605, y: 285, color: '#2ea44f', label: 'Lab' },
            { place: 'surgery_day', x: 95, y: 395, color: '#e74c3c', label: 'S-Day' },
        ];

        for (const rb of resourceBadges) {
            svg += `<rect x="${rb.x - 22}" y="${rb.y - 9}" width="44" height="18" rx="4" fill="rgba(0,0,0,0.5)" stroke="${rb.color}" stroke-width="1"/>`;
            svg += `<text id="res_${rb.place}" x="${rb.x}" y="${rb.y + 3}" fill="white" font-size="9" font-weight="600" text-anchor="middle">${rb.label}: 0</text>`;
        }

        svg += `</svg>`;
        this.innerHTML = `<div class="floor-plan-container">${svg}</div>`;
    }

    update(state) {
        if (!state) return;
        this._state = state;

        // Update place badges
        for (const room of ROOMS) {
            for (const p of (room.places || [])) {
                const val = Math.max(0, state[p] || 0);
                const badge = this.querySelector(`#badge_${p}`);
                if (badge) badge.textContent = val.toFixed(1);

                const bg = this.querySelector(`#badge_bg_${p}`);
                if (bg) {
                    bg.setAttribute('fill-opacity', val > 0.1 ? '0.7' : '0.4');
                }
            }

            // Queue bars
            for (const q of (room.queues || [])) {
                const val = Math.max(0, state[q] || 0);
                this._maxTokens[q] = Math.max(this._maxTokens[q] || 1, val, 1);
                const bar = this.querySelector(`#qbar_${q}`);
                if (bar) {
                    const maxW = room.w - 60;
                    const w = (val / this._maxTokens[q]) * maxW;
                    bar.setAttribute('width', Math.max(0, w).toFixed(1));
                }
            }
        }

        // Staff dots
        for (const sg of STAFF_GROUPS) {
            const avail = Math.max(0, state[sg.place] || 0);
            const total = sg.max;
            for (let i = 0; i < total; i++) {
                const dot = this.querySelector(`#staff_${sg.id}_${i}`);
                if (dot) {
                    if (i < Math.round(avail)) {
                        dot.setAttribute('fill', sg.color);
                        dot.setAttribute('fill-opacity', '0.9');
                    } else if (i < total) {
                        dot.setAttribute('fill', sg.color);
                        dot.setAttribute('fill-opacity', '0.15');
                    }
                }
            }
        }

        // Resource badges
        const resourcePlaces = [
            'exam_room_free', 'surgery_free', 'dental_free', 'radiology_free',
            'treatment_free', 'recovery_free', 'lab_free', 'surgery_day'
        ];
        const labels = {
            exam_room_free: 'Exam', surgery_free: 'Surg', dental_free: 'Dent',
            radiology_free: 'Rad', treatment_free: 'Trt', recovery_free: 'Rec',
            lab_free: 'Lab', surgery_day: 'S-Day'
        };
        for (const rp of resourcePlaces) {
            const el = this.querySelector(`#res_${rp}`);
            if (el) {
                const val = Math.max(0, state[rp] || 0);
                el.textContent = `${labels[rp]}: ${val.toFixed(1)}`;
            }
        }
    }
}

customElements.define('floor-plan', FloorPlan);
export { FloorPlan, ROOMS };
