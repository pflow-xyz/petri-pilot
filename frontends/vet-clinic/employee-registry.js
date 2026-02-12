/**
 * <employee-registry> Web Component — named staff roster
 *
 * Manages employee list grouped by role. Dispatches 'employees-change' events.
 */

const DEFAULT_EMPLOYEES = [
    { id: 'emp-1', name: 'Dr. Sarah Chen',  role: 'DVM',          employment: 'FT', color: '#e74c3c', workDays: ['Mon','Tue','Wed','Thu','Fri'] },
    { id: 'emp-2', name: 'Dr. James Park',  role: 'DVM',          employment: 'FT', color: '#c0392b', workDays: ['Mon','Tue','Wed','Thu','Fri'] },
    { id: 'emp-3', name: 'Dr. Lisa Wong',   role: 'DVM',          employment: 'PT', color: '#e67e22', workDays: ['Thu','Fri'] },
    { id: 'emp-4', name: 'Maria Garcia',    role: 'RVT',          employment: 'FT', color: '#3498db', workDays: ['Mon','Tue','Wed','Thu','Fri'] },
    { id: 'emp-5', name: 'Tyler Brooks',    role: 'RVT',          employment: 'FT', color: '#2980b9', workDays: ['Mon','Tue','Wed','Thu','Fri'] },
    { id: 'emp-6', name: 'Aisha Johnson',   role: 'RVT',          employment: 'PT', color: '#1abc9c', workDays: ['Mon','Wed','Fri'] },
    { id: 'emp-7', name: 'Kim Patel',       role: 'RVT',          employment: 'PT', color: '#16a085', workDays: ['Tue','Thu','Sat'] },
    { id: 'emp-8', name: 'Sam Rivera',      role: 'Receptionist', employment: 'FT', color: '#f39c12', workDays: ['Mon','Tue','Wed','Thu','Fri','Sat'] },
];

const ROLE_ORDER = ['DVM', 'RVT', 'Receptionist'];
const ROLE_COLORS = { DVM: '#e74c3c', RVT: '#3498db', Receptionist: '#f39c12' };

let nextId = 9;

class EmployeeRegistry extends HTMLElement {
    constructor() {
        super();
        this._employees = DEFAULT_EMPLOYEES.map(e => ({ ...e, workDays: [...e.workDays] }));
        this._showForm = false;
    }

    connectedCallback() {
        this.render();
    }

    get employees() { return this._employees; }

    set employees(list) {
        this._employees = list;
        this.render();
    }

    render() {
        const grouped = {};
        for (const role of ROLE_ORDER) grouped[role] = [];
        for (const emp of this._employees) {
            if (grouped[emp.role]) grouped[emp.role].push(emp);
        }

        let html = `<div class="employee-registry card">
            <h3>Staff</h3>`;

        for (const role of ROLE_ORDER) {
            const emps = grouped[role];
            html += `<div class="er-role-group">
                <div class="er-role-header">
                    <span class="er-role-badge" style="background:${ROLE_COLORS[role]}">${role}</span>
                    <span class="er-role-count">${emps.length}</span>
                </div>`;

            for (const emp of emps) {
                html += `<div class="er-employee-row" data-id="${emp.id}">
                    <span class="er-color-swatch" style="background:${emp.color}"></span>
                    <span class="er-name">${emp.name}</span>
                    <span class="er-employment-badge ${emp.employment.toLowerCase()}">${emp.employment}</span>
                    <button class="er-remove-btn" data-id="${emp.id}" title="Remove">&times;</button>
                </div>`;
            }
            html += `</div>`;
        }

        if (this._showForm) {
            html += `<div class="er-add-form">
                <input type="text" id="er-new-name" placeholder="Name" class="er-input">
                <select id="er-new-role" class="er-select">
                    <option value="DVM">DVM</option>
                    <option value="RVT">RVT</option>
                    <option value="Receptionist">Receptionist</option>
                </select>
                <div class="er-form-row">
                    <label class="er-radio-label">
                        <input type="radio" name="er-employment" value="FT" checked> FT
                    </label>
                    <label class="er-radio-label">
                        <input type="radio" name="er-employment" value="PT"> PT
                    </label>
                </div>
                <div class="er-form-actions">
                    <button id="er-confirm-add" class="er-btn er-btn-confirm">Add</button>
                    <button id="er-cancel-add" class="er-btn er-btn-cancel">Cancel</button>
                </div>
            </div>`;
        } else {
            html += `<button class="er-add-btn" id="er-show-add">+ Add Employee</button>`;
        }

        html += `</div>`;
        this.innerHTML = html;
        this._bindEvents();
    }

    _bindEvents() {
        // Remove buttons
        this.querySelectorAll('.er-remove-btn').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                this._employees = this._employees.filter(emp => emp.id !== btn.dataset.id);
                this.render();
                this._emit();
            });
        });

        // Show add form
        const showBtn = this.querySelector('#er-show-add');
        if (showBtn) {
            showBtn.addEventListener('click', () => {
                this._showForm = true;
                this.render();
                this.querySelector('#er-new-name')?.focus();
            });
        }

        // Add form actions
        const confirmBtn = this.querySelector('#er-confirm-add');
        if (confirmBtn) {
            confirmBtn.addEventListener('click', () => this._addEmployee());
        }
        const cancelBtn = this.querySelector('#er-cancel-add');
        if (cancelBtn) {
            cancelBtn.addEventListener('click', () => {
                this._showForm = false;
                this.render();
            });
        }

        // Enter to submit
        const nameInput = this.querySelector('#er-new-name');
        if (nameInput) {
            nameInput.addEventListener('keydown', (e) => {
                if (e.key === 'Enter') this._addEmployee();
                if (e.key === 'Escape') { this._showForm = false; this.render(); }
            });
        }
    }

    _addEmployee() {
        const name = this.querySelector('#er-new-name')?.value?.trim();
        if (!name) return;
        const role = this.querySelector('#er-new-role')?.value || 'RVT';
        const employment = this.querySelector('input[name="er-employment"]:checked')?.value || 'FT';

        const palette = { DVM: '#e74c3c', RVT: '#3498db', Receptionist: '#f39c12' };
        const baseColor = palette[role];
        // Slight variation per employee
        const variation = (nextId * 37) % 60 - 30;
        const color = baseColor; // keep simple for now

        const workDays = employment === 'FT'
            ? ['Mon','Tue','Wed','Thu','Fri']
            : ['Mon','Wed','Fri'];

        this._employees.push({
            id: `emp-${nextId++}`,
            name,
            role,
            employment,
            color,
            workDays,
        });

        this._showForm = false;
        this.render();
        this._emit();
    }

    _emit() {
        this.dispatchEvent(new CustomEvent('employees-change', {
            detail: { employees: this._employees.map(e => ({ ...e })) },
            bubbles: true,
        }));
    }
}

customElements.define('employee-registry', EmployeeRegistry);
export { EmployeeRegistry, DEFAULT_EMPLOYEES, ROLE_ORDER, ROLE_COLORS };
