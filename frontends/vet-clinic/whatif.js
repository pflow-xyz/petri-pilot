/**
 * Vet-clinic what-if console.
 *
 * Every control binds to something the Petri net actually has: the staffing
 * sliders are token counts in the resource pools, the patience slider is the
 * rate of the walk-in abandon transitions, and each disruption is a marking
 * override or a rate schedule. Nothing here is a display-only decoration.
 *
 * Everything is a POST to /api/scenario on the generated backend. Those
 * endpoints are pure reads: they take a marking, run a copy of the model
 * forward, and return a trajectory. Asking a question cannot change the
 * clinic. The in-browser floor-plan simulation on the other tab is a separate,
 * older engine; this console only trusts the server.
 */

// The app is served under a prefix — /vet-clinic/ — so an absolute "/api/rates"
// asks the server root, which does not have it. Resolve against
// document.baseURI so the page works under any prefix, including none.
const API_BASE = (window.API_BASE || '').replace(/\/$/, '')
const url = (path) => (API_BASE
  ? `${API_BASE}${path}`
  : new URL(path.replace(/^\//, ''), document.baseURI).toString())

const el = (id) => document.getElementById(id)

/** The model's declared rates and initial marking, fetched once. */
let model = { rates: {}, initial: {} }

/**
 * Patient queues, discovered from the model rather than hardcoded. wait_lab is
 * excluded: it holds blood samples, not waiting clients, and counting it would
 * report a queue no one is standing in.
 */
let queues = []

/** Walk-in abandon transitions — the ones the patience slider owns. Surgery
 * and dental reschedule at the model's own declared rates: a booked procedure
 * and a walk-in do not share a patience. */
let walkInAbandons = []

async function loadModel () {
  const res = await fetch(url('/api/rates'))
  if (!res.ok) throw new Error(`GET /api/rates: HTTP ${res.status}`)
  model = await res.json()

  queues = Object.keys(model.initial)
    .filter((p) => p.startsWith('wait_') && p !== 'wait_lab')
    .sort()
  walkInAbandons = Object.keys(model.rates)
    .filter((id) => ['abandon_exam', 'abandon_tech', 'abandon_diag'].includes(id))
    .sort()

  // Seed the controls from the net, so the page opens showing what the model
  // actually says rather than a number someone typed into the HTML.
  const seed = (id, place) => {
    if (model.initial[place] != null) setSlider(id, model.initial[place])
  }
  seed('dvms', 'dvm_avail')
  seed('rvts', 'rvt_avail')
  seed('receptionists', 'receptionist_avail')
  if (model.rates.patient_arrives > 0) setSlider('arrivals', Math.round(model.rates.patient_arrives))
  const abandon = model.rates[walkInAbandons[0]]
  if (abandon > 0) setSlider('patience', Math.max(5, Math.round(60 / abandon)))
}

function setSlider (id, value) {
  const input = el(id)
  if (!input) return
  input.value = String(value)
  syncOutput(id)
}

function syncOutput (id) {
  const input = el(id)
  const out = el(`${id}-out`)
  if (!input || !out) return
  out.textContent = id === 'patience' ? `${input.value} min` : input.value
}

/**
 * Build a scenario from the controls.
 *
 * The disruptions are the reason this console exists. Emergencies at the door
 * are tokens in wait_emergency; a wave is a rate schedule on the
 * emergency_arrives source (a constant rate cannot say "between hour 1 and
 * 3"); closing the surgery gate zeroes the read-arc place; a broken x-ray is
 * an empty radiology pool.
 */
function buildScenario (name, staffing) {
  const hours = Number(el('hours').value)
  const patienceMinutes = Number(el('patience').value)
  const emergencies = Number(el('emergencies').value)

  const marking = {
    dvm_avail: staffing.dvms,
    rvt_avail: staffing.rvts,
    receptionist_avail: Number(el('receptionists').value)
  }
  if (emergencies > 0) marking.wait_emergency = emergencies
  if (el('no-surgery').checked) marking.surgery_day = 0
  if (el('xray-down').checked) marking.radiology_free = 0

  const rates = { patient_arrives: Number(el('arrivals').value) }
  for (const id of walkInAbandons) {
    rates[id] = 60 / Math.max(1, patienceMinutes)
  }

  const scenario = {
    name,
    marking,
    rates,
    hours,
    samples: 80,
    realizations: 16,
    seed: 20260808
  }

  if (el('emergency-stream').checked) {
    scenario.schedule = {
      emergency_arrives: [
        { until: Math.min(1, hours), value: 0 },
        { until: Math.min(3, hours), value: 2 },
        { until: hours, value: 0 }
      ]
    }
  }

  return scenario
}

async function postJSON (path, body) {
  const res = await fetch(url(path), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  })
  const text = await res.text()
  if (!res.ok) throw new Error(text || `HTTP ${res.status}`)
  return JSON.parse(text)
}

function busy (message) {
  el('status').textContent = message || ''
  el('run').disabled = Boolean(message)
  el('compare').disabled = Boolean(message)
}

async function runOne () {
  busy('running…')
  try {
    const staffing = { dvms: Number(el('dvms').value), rvts: Number(el('rvts').value) }
    const result = await postJSON('/api/scenario', buildScenario('this scenario', staffing))
    render([{ name: `${staffing.dvms} DVM / ${staffing.rvts} RVT`, result }])
  } catch (err) {
    showError(err)
  } finally {
    busy('')
  }
}

async function runCompare () {
  busy('running three scenarios…')
  try {
    const dvms = Number(el('dvms').value)
    const rvts = Number(el('rvts').value)
    const ladder = [
      { name: `${dvms} DVM / ${rvts} RVT`, dvms, rvts },
      { name: `${dvms + 1} DVM / ${rvts} RVT`, dvms: dvms + 1, rvts },
      { name: `${dvms + 1} DVM / ${rvts + 1} RVT`, dvms: dvms + 1, rvts: rvts + 1 }
    ]
    // One request, not three: the scenarios have to share a seed to be
    // comparable at all, and the server is where that can be enforced.
    const { scenarios: results } = await postJSON('/api/scenario/compare', {
      scenarios: ladder.map((s) => buildScenario(s.name, s))
    })
    render(results.map((s) => ({ name: s.name, result: s.result })))
  } catch (err) {
    showError(err)
  } finally {
    busy('')
  }
}

function showError (err) {
  el('results').innerHTML = `<div class="whatif-error"><b>That scenario was refused.</b><br>${escapeHTML(String(err.message || err))}</div>`
}

function escapeHTML (s) {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
}

const round = (v) => (v == null ? '—' : Math.round(v).toLocaleString())
const round1 = (v) => (v == null ? '—' : v.toFixed(1))

/**
 * What kept patients waiting, and for how much of the day. This is the
 * engine's Contended list; queue places are dropped because "waiting for
 * patients" is true but is not a shortage anyone can staff their way out of —
 * the engine classifies supply structurally (kind), so this is a filter, not
 * a re-ranking.
 */
function waitedOn (result) {
  const shortages = (result.contended || [])
    .filter((c) => c.kind === 'conserved' || c.kind === 'bounded')
    .slice(0, 3)
  if (shortages.length === 0) return 'nothing'
  return shortages
    .map((c) => `${escapeHTML(c.place)} ${Math.round(c.fraction * 100)}% of the day`)
    .join('<br>')
}

function sumQueues (values) {
  return queues.reduce((sum, place) => sum + (values?.[place] || 0), 0)
}

/** Peak of the total queue across the shared time grid — mean over runs, so
 * the peak of the average day, not the worst single run. */
function peakQueue (result) {
  const series = queues
    .map((place) => (result.series || []).find((s) => s.place === place))
    .filter(Boolean)
  if (series.length === 0) return null
  const total = series[0].values.map((_, i) =>
    series.reduce((sum, s) => sum + (s.values[i] || 0), 0))
  return total.reduce((top, v) => (v > top ? v : top), 0)
}

/** The three queues with the worst 5% moments. No total: percentiles are not
 * additive, and the queues do not hit their bad moments together. */
function worstQueues (result) {
  const p95 = result.metrics?.p95
  if (!p95) return '—'
  return queues
    .map((place) => ({ place, v: p95[place] || 0 }))
    .sort((a, b) => b.v - a.v)
    .slice(0, 3)
    .map(({ place, v }) => `${escapeHTML(place.replace('wait_', ''))} ${round1(v)}`)
    .join('<br>')
}

function surgeriesDone (result) {
  const th = result.metrics?.throughput
  if (!th) return '—'
  return round((th.finish_spay || 0) + (th.finish_neuter || 0) + (th.finish_dental || 0))
}

function render (runs) {
  const rows = [
    ['Patients discharged', (r) => round(r.final.discharged)],
    ['Walked out unseen', (r) => round(r.final.walked_out)],
    ['Emergencies diverted', (r) => round(r.final.diverted)],
    ['Procedures completed', surgeriesDone],
    ['Queue — typical', (r) => round1(sumQueues(r.metrics?.mean))],
    ['Queue — peak, average day', (r) => round1(peakQueue(r))],
    ['Queue — worst 5%, top three', worstQueues],
    ['Waiting on', waitedOn]
  ]

  const head = runs.map((run) => `<th scope="col">${escapeHTML(run.name)}</th>`).join('')
  const body = rows.map(([label, get]) => `
    <tr>
      <th scope="row">${label}</th>
      ${runs.map((run) => `<td>${get(run.result)}</td>`).join('')}
    </tr>`).join('')

  // Caveats are constraints this model declares that the run could not
  // enforce (empty is a claim, so only shown when present). Assumptions are
  // what the engine assumes whatever the model says — every SSA run carries
  // the exponential-durations note, and no edit to the net removes it.
  const caveats = [...new Set(runs.flatMap((run) => run.result.caveats || []))]
  const assumptions = [...new Set(runs.flatMap((run) => run.result.assumptions || []))]

  el('results').innerHTML = `
    <div class="table-scroll"><table class="whatif-compare">
      <thead><tr><th scope="col"></th>${head}</tr></thead>
      <tbody>${body}</tbody>
    </table></div>
    <p class="whatif-note">
      Discrete engine, averaged over 16 runs on one seed — a single run of a queue is an anecdote.
      “Worst 5%” is what the client standing in that queue experiences on a bad day.
    </p>
    ${caveats.length ? `<div class="whatif-caveat"><b>Not enforced in this run:</b><ul>${
      caveats.map((c) => `<li>${escapeHTML(c)}</li>`).join('')}</ul></div>` : ''}
    ${assumptions.length ? `<div class="whatif-caveat whatif-assumption"><b>What this method assumes:</b><ul>${
      assumptions.map((a) => `<li>${escapeHTML(a)}</li>`).join('')}</ul></div>` : ''}`
}

function wire () {
  for (const id of ['dvms', 'rvts', 'receptionists', 'arrivals', 'patience', 'hours']) {
    el(id)?.addEventListener('input', () => syncOutput(id))
    syncOutput(id)
  }
  el('run')?.addEventListener('click', runOne)
  el('compare')?.addEventListener('click', runCompare)
}

wire()
loadModel()
  .then(runCompare)
  .catch((err) => {
    showError(new Error(`could not read the model from /api/rates — ${err.message}`))
  })
