/**
 * <financial-dashboard> Web Component — P&L, inventory, and cost tracking
 *
 * Displays revenue, supply costs, staff costs, gross profit with margin,
 * inventory depletion bars with color-coded alerts, and low stock warnings.
 * Updated each frame from precomputed financial data.
 */

import { SUPPLY_CATALOG } from './clinic-costs.js';

// Supplies to show in the inventory section (skip mundane items like gloves)
const INVENTORY_DISPLAY = [
    'nexgard', 'heartgard', 'vaccines', 'anesthesia', 'sutures',
    'dental_paste', 'xray_film', 'blood_tubes', 'iv_fluids',
    'antibiotics', 'pain_meds', 'bandage', 'syringes',
];

class FinancialDashboard extends HTMLElement {
    constructor() {
        super();
        this._financials = null;
        this._snapshot = null;
    }

    connectedCallback() {
        this.render();
    }

    /** Store precomputed financial arrays */
    setFinancials(data) {
        this._financials = data;
    }

    /** Update display at current scrub time */
    update(snapshot) {
        if (!snapshot) return;
        this._snapshot = snapshot;
        this._updatePL();
        this._updateAlerts();
        this._updateInventory();
    }

    render() {
        const supplyRows = INVENTORY_DISPLAY.map(id => {
            const s = SUPPLY_CATALOG.find(c => c.id === id);
            if (!s) return '';
            return `<div class="fin-inv-row" id="fin-inv-${id}">
                <span class="fin-inv-label">${s.label}</span>
                <div class="fin-bar-track">
                    <div class="fin-bar-fill" id="fin-bar-${id}"></div>
                </div>
                <span class="fin-inv-nums" id="fin-nums-${id}">—</span>
            </div>`;
        }).join('');

        this.innerHTML = `
        <div class="financial-dashboard card">
            <h3>Financials</h3>

            <div class="fin-pl">
                <div class="fin-row">
                    <span class="fin-label">Revenue</span>
                    <span class="fin-value" id="fin-revenue">$0</span>
                    <div class="fin-bar-sm"><div class="fin-bar-sm-fill revenue-bar" id="fin-rev-bar"></div></div>
                </div>
                <div class="fin-row">
                    <span class="fin-label">Supplies</span>
                    <span class="fin-value" id="fin-supply-cost">$0</span>
                    <div class="fin-bar-sm"><div class="fin-bar-sm-fill supply-bar" id="fin-sup-bar"></div></div>
                </div>
                <div class="fin-row">
                    <span class="fin-label">Labs & Meds</span>
                    <span class="fin-value" id="fin-cogs-cost">$0</span>
                    <div class="fin-bar-sm"><div class="fin-bar-sm-fill cogs-bar" id="fin-cogs-bar"></div></div>
                </div>
                <div class="fin-row">
                    <span class="fin-label">Staff Cost</span>
                    <span class="fin-value" id="fin-staff-cost">$0</span>
                    <div class="fin-bar-sm"><div class="fin-bar-sm-fill staff-bar" id="fin-staff-bar"></div></div>
                </div>
                <div class="fin-divider"></div>
                <div class="fin-row fin-profit-row">
                    <span class="fin-label"><strong>Gross Profit</strong></span>
                    <span class="fin-value fin-profit" id="fin-profit">$0</span>
                    <span class="fin-margin" id="fin-margin">0%</span>
                </div>
            </div>

            <div class="fin-alerts" id="fin-alerts" style="display:none;"></div>

            <div class="fin-inventory-section">
                <div class="fin-inv-header">
                    <span>Inventory</span>
                    <span class="fin-inv-legend">Stock / Used</span>
                </div>
                ${supplyRows}
            </div>
        </div>`;
    }

    _updatePL() {
        const s = this._snapshot;
        const setVal = (id, val) => {
            const el = this.querySelector(`#${id}`);
            if (el) el.textContent = val;
        };

        setVal('fin-revenue', '$' + Math.round(s.revenue).toLocaleString());
        setVal('fin-supply-cost', '$' + Math.round(s.supplyCost).toLocaleString());
        setVal('fin-cogs-cost', '$' + Math.round(s.cogsCost).toLocaleString());
        setVal('fin-staff-cost', '$' + Math.round(s.staffCost).toLocaleString());
        setVal('fin-profit', '$' + Math.round(s.grossProfit).toLocaleString());

        // Color profit green/red
        const profitEl = this.querySelector('#fin-profit');
        if (profitEl) {
            profitEl.style.color = s.grossProfit >= 0 ? 'var(--accent-green)' : 'var(--accent-red)';
        }

        // Margin badge
        const marginEl = this.querySelector('#fin-margin');
        if (marginEl) {
            marginEl.textContent = Math.round(s.margin) + '%';
            marginEl.className = 'fin-margin' + (s.margin >= 40 ? ' good' : s.margin >= 20 ? ' ok' : ' low');
        }

        // P&L bars (proportional to revenue as 100%)
        const maxVal = Math.max(s.revenue, 1);
        this._setBarWidth('fin-rev-bar', 100);
        this._setBarWidth('fin-sup-bar', (s.supplyCost / maxVal) * 100);
        this._setBarWidth('fin-cogs-bar', (s.cogsCost / maxVal) * 100);
        this._setBarWidth('fin-staff-bar', (s.staffCost / maxVal) * 100);
    }

    _updateAlerts() {
        const alertsEl = this.querySelector('#fin-alerts');
        if (!alertsEl) return;

        const alerts = this._snapshot.alerts;
        if (alerts.length === 0) {
            alertsEl.style.display = 'none';
            return;
        }

        alertsEl.style.display = 'block';
        alertsEl.innerHTML = '<div class="fin-alert-title">Low Stock</div>' +
            alerts.map(a => {
                const icon = a.critical ? '\u26a0' : '\u25b3';
                const cls = a.critical ? 'critical' : 'warning';
                return `<div class="fin-alert-item ${cls}">${icon} ${a.label}: ${a.remaining} remaining</div>`;
            }).join('');
    }

    _updateInventory() {
        const s = this._snapshot;
        for (const id of INVENTORY_DISPLAY) {
            const supply = SUPPLY_CATALOG.find(c => c.id === id);
            if (!supply) continue;

            const remaining = s.inventory[id] || 0;
            const used = s.supplyUsed[id] || 0;
            const startInv = supply.startingInventory;
            const pct = startInv > 0 ? (remaining / startInv) * 100 : 0;

            const bar = this.querySelector(`#fin-bar-${id}`);
            if (bar) {
                bar.style.width = Math.max(0, Math.min(100, pct)) + '%';
                bar.className = 'fin-bar-fill' + (pct > 50 ? ' green' : pct > 25 ? ' yellow' : ' red');
            }

            const nums = this.querySelector(`#fin-nums-${id}`);
            if (nums) {
                nums.textContent = `${Math.round(remaining)} / ${Math.round(used)}`;
            }
        }
    }

    _setBarWidth(id, pct) {
        const el = this.querySelector(`#${id}`);
        if (el) el.style.width = Math.max(0, Math.min(100, pct)) + '%';
    }
}

customElements.define('financial-dashboard', FinancialDashboard);
export { FinancialDashboard };
