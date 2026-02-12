/**
 * clinic-financials.js — Financial computation from ODE solution
 *
 * Derives cumulative procedure counts, supply consumption, revenue, costs,
 * and gross profit from ODE flux integration. Uses the same trapezoidal
 * integration pattern as event-feed.js.
 *
 * All arrays are aligned with solution.t[] for easy interpolation.
 */

import { SUPPLY_CATALOG, PROCEDURE_REVENUE, PROCEDURE_COGS, STAFF_RATES, getSuppliesForTransition, getSupplyCatalogMap } from './clinic-costs.js';

// Transitions we track for financial purposes
const TRACKED_TRANSITIONS = Object.keys(PROCEDURE_REVENUE);

/**
 * Precompute all financial data from an ODE solution.
 *
 * @param {Object} solution - ODE solution with .t[], .u[], .stateLabels
 * @param {Object} net - PflowEngine net (for inputArcs)
 * @param {Object} rates - Rate map used in ODE (transition id → rate)
 * @param {Object} config - Schedule config with staffing levels
 * @param {Array|null} staffTimeline - Time-varying staff [{time, dvm_avail, rvt_avail, receptionist_avail}]
 * @returns {Object} financials object with all cumulative arrays
 */
export function precomputeFinancials(solution, net, rates, config, staffTimeline) {
    const n = solution.t.length;
    const catalogMap = getSupplyCatalogMap();

    // ── 1. Integrate flux per start_* transition (trapezoidal rule) ──
    const cumProcs = {}; // transitionId → Float64Array[n]
    for (const tId of TRACKED_TRANSITIONS) {
        cumProcs[tId] = new Float64Array(n);
    }

    for (let i = 1; i < n; i++) {
        const dt = solution.t[i] - solution.t[i - 1];
        if (dt <= 0) {
            // Copy previous values
            for (const tId of TRACKED_TRANSITIONS) {
                cumProcs[tId][i] = cumProcs[tId][i - 1];
            }
            continue;
        }

        const prevState = solution.u[i - 1];
        const curState = solution.u[i];

        for (const tId of TRACKED_TRANSITIONS) {
            const rate = rates[tId] || 0;
            if (rate <= 0) {
                cumProcs[tId][i] = cumProcs[tId][i - 1];
                continue;
            }

            // Compute mass-action flux at prev and current states
            const inputArcs = net.inputArcs(tId);
            let fluxPrev = rate;
            let fluxCur = rate;

            for (const arc of inputArcs) {
                const w = arc.getWeightSum();
                const pPrev = Math.max(0, prevState[arc.source] || 0);
                const pCur = Math.max(0, curState[arc.source] || 0);
                fluxPrev *= Math.pow(pPrev, w);
                fluxCur *= Math.pow(pCur, w);
            }

            // Trapezoidal integration
            const avgFlux = (fluxPrev + fluxCur) / 2;
            cumProcs[tId][i] = cumProcs[tId][i - 1] + avgFlux * dt;
        }
    }

    // ── 2. Cumulative supply consumption ──
    const supplyCum = {}; // supplyId → Float64Array[n]
    for (const s of SUPPLY_CATALOG) {
        supplyCum[s.id] = new Float64Array(n);
    }

    for (let i = 0; i < n; i++) {
        for (const tId of TRACKED_TRANSITIONS) {
            const supplies = getSuppliesForTransition(tId);
            for (const { supplyId, qtyPerProc } of supplies) {
                supplyCum[supplyId][i] += cumProcs[tId][i] * qtyPerProc;
            }
        }
    }

    // ── 3. Cumulative revenue ──
    const revenue = new Float64Array(n);
    for (let i = 0; i < n; i++) {
        for (const tId of TRACKED_TRANSITIONS) {
            revenue[i] += cumProcs[tId][i] * PROCEDURE_REVENUE[tId];
        }
    }

    // ── 4. Cumulative supply cost (inventory consumables) ──
    const supplyCost = new Float64Array(n);
    for (let i = 0; i < n; i++) {
        for (const s of SUPPLY_CATALOG) {
            supplyCost[i] += supplyCum[s.id][i] * s.unitCost;
        }
    }

    // ── 4b. Cumulative procedure COGS (lab fees, drug wholesale, equipment) ──
    const cogsCost = new Float64Array(n);
    for (let i = 0; i < n; i++) {
        for (const tId of TRACKED_TRANSITIONS) {
            cogsCost[i] += cumProcs[tId][i] * (PROCEDURE_COGS[tId] || 0);
        }
    }

    // ── 5. Cumulative staff cost ──
    const staffCost = new Float64Array(n);
    if (staffTimeline && staffTimeline.length > 1) {
        // Piecewise staff cost from timeline
        let tlIdx = 0;
        for (let i = 1; i < n; i++) {
            const t = solution.t[i];
            const dt = solution.t[i] - solution.t[i - 1];
            // Advance timeline index
            while (tlIdx < staffTimeline.length - 1 && staffTimeline[tlIdx + 1].time <= t) {
                tlIdx++;
            }
            const entry = staffTimeline[tlIdx];
            const hourlyTotal = (entry.dvm_avail * STAFF_RATES.dvm)
                              + (entry.rvt_avail * STAFF_RATES.rvt)
                              + (entry.receptionist_avail * STAFF_RATES.receptionist);
            staffCost[i] = staffCost[i - 1] + hourlyTotal * dt;
        }
    } else {
        // Constant staffing
        const hourlyTotal = ((config.dvmCount || 2) * STAFF_RATES.dvm)
                          + ((config.rvtCount || 3) * STAFF_RATES.rvt)
                          + ((config.receptionistCount || 1) * STAFF_RATES.receptionist);
        for (let i = 1; i < n; i++) {
            const dt = solution.t[i] - solution.t[i - 1];
            staffCost[i] = staffCost[i - 1] + hourlyTotal * dt;
        }
    }

    // ── 6. Gross profit ──
    const grossProfit = new Float64Array(n);
    for (let i = 0; i < n; i++) {
        grossProfit[i] = revenue[i] - supplyCost[i] - cogsCost[i] - staffCost[i];
    }

    // ── 7. Inventory remaining ──
    const inventoryRemaining = {}; // supplyId → Float64Array[n]
    for (const s of SUPPLY_CATALOG) {
        inventoryRemaining[s.id] = new Float64Array(n);
        for (let i = 0; i < n; i++) {
            inventoryRemaining[s.id][i] = Math.max(0, s.startingInventory - supplyCum[s.id][i]);
        }
    }

    return {
        t: solution.t,
        cumProcs,
        supplyCum,
        revenue,
        supplyCost,
        cogsCost,
        staffCost,
        grossProfit,
        inventoryRemaining,
        catalogMap,
    };
}

/**
 * Interpolate financial data at an arbitrary time.
 * Uses binary search + linear interpolation.
 *
 * @param {Object} financials - Result from precomputeFinancials()
 * @param {number} time - Time to interpolate at
 * @returns {Object} Snapshot of all financial values at time
 */
export function interpolateAt(financials, time) {
    const t = financials.t;
    const n = t.length;
    if (n === 0) return null;

    // Binary search
    let lo = 0, hi = n - 1;
    while (lo < hi - 1) {
        const mid = (lo + hi) >> 1;
        if (t[mid] <= time) lo = mid; else hi = mid;
    }

    // Interpolation helper
    function lerp(arr) {
        if (time <= t[lo]) return arr[lo];
        if (time >= t[hi]) return arr[hi];
        const frac = (time - t[lo]) / (t[hi] - t[lo]);
        return arr[lo] + frac * (arr[hi] - arr[lo]);
    }

    // Interpolate scalar arrays
    const result = {
        revenue: lerp(financials.revenue),
        supplyCost: lerp(financials.supplyCost),
        cogsCost: lerp(financials.cogsCost),
        staffCost: lerp(financials.staffCost),
        grossProfit: lerp(financials.grossProfit),
        inventory: {},
        supplyUsed: {},
        procedures: {},
    };

    // Margin
    result.margin = result.revenue > 0 ? (result.grossProfit / result.revenue) * 100 : 0;

    // Inventory remaining + used
    for (const s of SUPPLY_CATALOG) {
        result.inventory[s.id] = lerp(financials.inventoryRemaining[s.id]);
        result.supplyUsed[s.id] = lerp(financials.supplyCum[s.id]);
    }

    // Per-procedure counts
    for (const tId of TRACKED_TRANSITIONS) {
        result.procedures[tId] = lerp(financials.cumProcs[tId]);
    }

    // Low stock alerts
    result.alerts = [];
    for (const s of SUPPLY_CATALOG) {
        if (result.inventory[s.id] <= s.restockAt && result.supplyUsed[s.id] > 0) {
            result.alerts.push({
                supplyId: s.id,
                label: s.label,
                remaining: Math.round(result.inventory[s.id]),
                restockAt: s.restockAt,
                critical: result.inventory[s.id] <= s.restockAt * 0.5,
            });
        }
    }

    return result;
}
