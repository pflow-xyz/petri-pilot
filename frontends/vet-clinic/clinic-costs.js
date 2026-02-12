/**
 * clinic-costs.js — Supply catalog, procedure revenue, and staff cost data
 *
 * Pure data module: no DOM, no side-effects. Consumed by clinic-financials.js
 * to derive inventory depletion and P&L from ODE flux integration.
 */

// ── Supply Catalog ──
// Each entry: id, label, startingInventory, unitCost, restockThreshold, unit
export const SUPPLY_CATALOG = [
    { id: 'gloves',       label: 'Exam gloves (box)',  startingInventory: 10,  unitCost: 8.50,  restockAt: 3,  unit: 'box' },
    { id: 'syringes',     label: 'Syringes',           startingInventory: 200, unitCost: 0.45,  restockAt: 50, unit: 'ea' },
    { id: 'vaccines',     label: 'Vaccine doses',      startingInventory: 40,  unitCost: 12.00, restockAt: 10, unit: 'dose' },
    { id: 'nexgard',      label: 'NexGard (flea/tick)',startingInventory: 30,  unitCost: 18.50, restockAt: 8,  unit: 'dose' },
    { id: 'heartgard',    label: 'Heartgard (heartworm)',startingInventory: 30, unitCost: 14.00, restockAt: 8,  unit: 'dose' },
    { id: 'anesthesia',   label: 'Anesthesia kit',     startingInventory: 8,   unitCost: 45.00, restockAt: 3,  unit: 'kit' },
    { id: 'sutures',      label: 'Suture pack',        startingInventory: 15,  unitCost: 18.00, restockAt: 5,  unit: 'pack' },
    { id: 'dental_paste', label: 'Dental prophy paste',startingInventory: 12,  unitCost: 8.00,  restockAt: 4,  unit: 'tube' },
    { id: 'xray_film',    label: 'X-ray film',         startingInventory: 60,  unitCost: 3.50,  restockAt: 15, unit: 'sheet' },
    { id: 'blood_tubes',  label: 'Blood collection tubes',startingInventory: 120, unitCost: 1.20, restockAt: 30, unit: 'tube' },
    { id: 'iv_fluids',    label: 'IV fluids (bag)',    startingInventory: 25,  unitCost: 6.50,  restockAt: 8,  unit: 'bag' },
    { id: 'bandage',      label: 'Bandage/wrap',       startingInventory: 40,  unitCost: 2.80,  restockAt: 10, unit: 'roll' },
    { id: 'antibiotics',  label: 'Antibiotics (dose)', startingInventory: 50,  unitCost: 4.50,  restockAt: 15, unit: 'dose' },
    { id: 'pain_meds',    label: 'Pain meds (dose)',   startingInventory: 60,  unitCost: 3.20,  restockAt: 15, unit: 'dose' },
];

// ── Per-transition supply consumption ──
// Maps start_* transition → [{ supplyId, qtyPerProc }]
// Fractional quantities model probability (e.g. 0.4 = 40% of visits include this)
const TRANSITION_SUPPLIES = {
    start_wellness:  [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'syringes', qtyPerProc: 1 }, { supplyId: 'nexgard', qtyPerProc: 0.4 }, { supplyId: 'heartgard', qtyPerProc: 0.3 }],
    start_sick:      [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'syringes', qtyPerProc: 1 }, { supplyId: 'antibiotics', qtyPerProc: 0.7 }],
    start_vaccine:   [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'syringes', qtyPerProc: 1 }, { supplyId: 'vaccines', qtyPerProc: 1 }],
    start_nail_trim: [{ supplyId: 'gloves', qtyPerProc: 0.04 }],
    start_weight:    [],
    start_suture:    [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'bandage', qtyPerProc: 1 }, { supplyId: 'syringes', qtyPerProc: 0.5 }],
    start_spay:      [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'anesthesia', qtyPerProc: 1 }, { supplyId: 'sutures', qtyPerProc: 1 }, { supplyId: 'iv_fluids', qtyPerProc: 0.8 }, { supplyId: 'pain_meds', qtyPerProc: 1 }, { supplyId: 'antibiotics', qtyPerProc: 0.5 }, { supplyId: 'bandage', qtyPerProc: 1 }],
    start_neuter:    [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'anesthesia', qtyPerProc: 1 }, { supplyId: 'sutures', qtyPerProc: 1 }, { supplyId: 'iv_fluids', qtyPerProc: 0.8 }, { supplyId: 'pain_meds', qtyPerProc: 1 }, { supplyId: 'antibiotics', qtyPerProc: 0.5 }, { supplyId: 'bandage', qtyPerProc: 1 }],
    start_dental:    [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'anesthesia', qtyPerProc: 1 }, { supplyId: 'dental_paste', qtyPerProc: 1 }, { supplyId: 'pain_meds', qtyPerProc: 0.8 }],
    start_xray:      [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'xray_film', qtyPerProc: 1.5 }],
    start_bloodwork: [{ supplyId: 'gloves', qtyPerProc: 0.04 }, { supplyId: 'syringes', qtyPerProc: 1 }, { supplyId: 'blood_tubes', qtyPerProc: 3 }],
};

// ── Procedure Revenue (total visit revenue including typical add-ons) ──
// Each visit bundles the office visit + labs + dispensed meds + interpretation.
// Target: each DVM generates $5-7K/day on a busy surgery day.
export const PROCEDURE_REVENUE = {
    start_wellness:  250,   // office visit + vaccines + HW test + preventative markup
    start_sick:      275,   // office visit + diagnostics + dispensed meds
    start_vaccine:   130,   // office visit + vaccine series
    start_nail_trim: 35,    // tech service
    start_weight:    25,    // minimal or included
    start_suture:    85,    // post-op recheck + materials
    start_spay:      650,   // pre-op labs + anesthesia monitoring + meds + e-collar
    start_neuter:    500,   // same as spay, shorter procedure
    start_dental:    650,   // anesthesia + dental x-rays + common extractions
    start_xray:      375,   // digital images + DVM interpretation
    start_bloodwork: 275,   // CBC + full chemistry panel + urinalysis
};

// ── Per-procedure COGS beyond tracked inventory ──
// Reference lab fees, wholesale drug costs, equipment wear, outsourced services.
// Combined with supply catalog costs, targets ~30-40% total COGS on revenue.
export const PROCEDURE_COGS = {
    start_wellness:  55,    // heartworm test kit, fecal screen, multi-vaccine wholesale
    start_sick:      65,    // cytology, culture, dispensed Rx at wholesale
    start_vaccine:   25,    // vaccine wholesale beyond tracked doses
    start_nail_trim: 2,     // minimal
    start_weight:    0,
    start_suture:    10,    // wound care supplies
    start_spay:      120,   // surgical supplies, monitoring disposables, drug wholesale
    start_neuter:    90,    // same as spay, shorter
    start_dental:    130,   // dental x-ray sensor wear, extraction tools, drug wholesale
    start_xray:      45,    // digital sensor amortization, contrast if needed
    start_bloodwork: 85,    // reference lab CBC+chem panel fee
};

// ── Staff Hourly Rates ──
export const STAFF_RATES = {
    dvm: 75,
    rvt: 28,
    receptionist: 18,
};

/** Get supply list for a transition, or empty array */
export function getSuppliesForTransition(transitionId) {
    return TRANSITION_SUPPLIES[transitionId] || [];
}

/** Build a supply lookup map by id for fast access */
export function getSupplyCatalogMap() {
    const map = {};
    for (const s of SUPPLY_CATALOG) map[s.id] = s;
    return map;
}
