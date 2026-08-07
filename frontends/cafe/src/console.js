/**
 * Café what-if console.
 *
 * Every control here binds to something the Petri net actually has. The barista
 * slider is the token count in `staff/available`; the patience slider is the
 * rate of the `abandon` transition; the rush toggle is a piecewise rate
 * schedule. Nothing is a display-only decoration sitting beside a model that
 * cannot express it — which is the trap the older coffee-shop dashboard fell
 * into, where the barista was an emoji with a CSS class and the "rush hour"
 * preset had to make the barista work *faster* to compensate for a net with no
 * capacity in it at all.
 *
 * Everything is a POST to /api/scenario. Those endpoints are pure reads: they
 * take a marking, run a copy forward, and return a trajectory. Asking a
 * question cannot change the shop.
 */

/**
 * Resolve an API path against wherever this page is mounted.
 *
 * The app is served under a prefix — /cafe/ — so an absolute "/api/rates" asks
 * the *server root*, which does not have it. That 404s, the console shows an
 * empty page, and nothing in a headless binding check notices, because a check
 * that is handed a base URL never exercises the default. Resolve against
 * document.baseURI instead, so the page works under any prefix, including none.
 */
const API_BASE = (window.API_BASE || '').replace(/\/$/, '')
const url = (path) => (API_BASE
  ? `${API_BASE}${path}`
  : new URL(path.replace(/^\//, ''), document.baseURI).toString())

const el = (id) => document.getElementById(id)

/** The model's declared rates and initial marking, fetched once. */
let model = { rates: {}, initial: {} }

/** Order transitions, discovered from the model rather than hardcoded. */
let orderTransitions = []

async function loadModel () {
  const res = await fetch(url('/api/rates'))
  if (!res.ok) throw new Error(`GET /api/rates: HTTP ${res.status}`)
  model = await res.json()

  orderTransitions = Object.keys(model.rates)
    .filter((id) => id.includes('order_'))
    .sort()

  // Seed the controls from the net, so the page opens showing what the model
  // actually says rather than a number someone typed into the HTML.
  const arrivals = orderTransitions.reduce((sum, id) => sum + (model.rates[id] || 0), 0)
  if (arrivals > 0) setSlider('arrivals', Math.round(arrivals))
  if (model.initial['staff/available'] != null) {
    setSlider('baristas', model.initial['staff/available'])
  }
  const abandon = model.rates['counter/abandon']
  if (abandon > 0) setSlider('patience', Math.max(1, Math.round(60 / abandon)))
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
 * Arrival rate is distributed across the order transitions in the proportion
 * the model declares. Scaling them together is the honest reading of "more
 * customers": the mix of what people order does not change because the shop got
 * busier.
 */
function buildScenario (name, baristas) {
  const hours = Number(el('hours').value)
  const arrivals = Number(el('arrivals').value)
  const patienceMinutes = Number(el('patience').value)

  const declared = orderTransitions.reduce((sum, id) => sum + (model.rates[id] || 0), 0)
  const rates = {}
  for (const id of orderTransitions) {
    const share = declared > 0 ? (model.rates[id] || 0) / declared : 1 / orderTransitions.length
    rates[id] = arrivals * share
  }
  // A customer who waits `patienceMinutes` leaves: as a rate, that is how many
  // times an hour they would give up.
  rates['counter/abandon'] = 60 / Math.max(1, patienceMinutes)

  const scenario = {
    name,
    marking: { 'staff/available': baristas },
    rates,
    hours,
    samples: 80,
    realizations: 16,
    seed: 20260806
  }

  if (el('rush').checked) {
    // A rush is the one thing a constant rate cannot say. Same customers,
    // arriving together — and whether the queue recovers afterwards is exactly
    // what averaging them out would hide.
    const rushHours = Math.min(2, hours)
    scenario.schedule = {}
    for (const id of orderTransitions) {
      const base = rates[id]
      scenario.schedule[id] = [
        { until: rushHours, value: base * 3 },
        { until: hours, value: base * 0.4 }
      ]
      delete scenario.rates[id]
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
    const baristas = Number(el('baristas').value)
    const result = await postJSON('/api/scenario', buildScenario('this scenario', baristas))
    render([{ name: `${baristas} barista${baristas === 1 ? '' : 's'}`, result }])
  } catch (err) {
    showError(err)
  } finally {
    busy('')
  }
}

async function runCompare () {
  busy('running three scenarios…')
  try {
    const baristas = Number(el('baristas').value)
    const scenarios = [0, 1, 2].map((extra) => {
      const n = baristas + extra
      return buildScenario(`${n} barista${n === 1 ? '' : 's'}`, n)
    })
    // One request, not three: the scenarios have to share a seed to be
    // comparable at all, and the server is where that can be enforced. Three
    // separate calls would differ by the dice as much as by the staffing.
    const { scenarios: results } = await postJSON('/api/scenario/compare', { scenarios })
    render(results.map((s) => ({ name: s.name, result: s.result })))
  } catch (err) {
    showError(err)
  } finally {
    busy('')
  }
}

function showError (err) {
  el('results').innerHTML = `<div class="error"><b>That scenario was refused.</b><br>${escapeHTML(String(err.message || err))}</div>`
}

function escapeHTML (s) {
  return s.replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]))
}

const round = (v) => (v == null ? '—' : Math.round(v).toLocaleString())

/**
 * Depletions worth calling "ran out", which is not all of them.
 *
 * A resource pool reaching zero means every barista is busy — a normal moment
 * in a working shop, and it refills itself by construction. Stock reaching zero
 * means the shop cannot make the drink. Both hit the floor, so the engine
 * reports both; listing them together under "Ran out" told the owner their
 * staff had run out. The pool halves are already answered by the utilization
 * row, so they are dropped here rather than explained twice.
 */
function stockDepletions (result) {
  const pools = Object.keys(result.metrics?.utilization || {})
  const poolPlaces = new Set(pools.flatMap((p) => [
    `${p}/available`, `${p}/busy`, `${p}/in_use`, 'available', 'busy', 'in_use'
  ]))
  return (result.depleted || []).filter((d) => !poolPlaces.has(d.place))
}

/** Peak of a place's trajectory — what "how bad did it get" means. */
function peak (result, place) {
  const series = (result.series || []).find((s) => s.place === place)
  if (!series) return null
  return series.values.reduce((top, v) => (v > top ? v : top), 0)
}

function render (runs) {
  const rows = [
    ['Drinks served', (r) => round(r.final['counter/orders_complete'])],
    ['Customers who left', (r) => round(r.final['counter/walked_out'])],
    ['Queue — typical', (r) => round(r.metrics?.mean?.['counter/orders_pending'])],
    ['Queue — worst 5%', (r) => round(r.metrics?.p95?.['counter/orders_pending'])],
    ['Queue — peak', (r) => round(peak(r, 'counter/orders_pending'))],
    ['Baristas busy', (r) => {
      const u = r.metrics?.utilization?.staff
      return u == null ? '—' : `${Math.round(u * 100)}%`
    }],
    ['Beans left', (r) => round(r.final['pantry/coffee_beans'])],
    ['Ran out', (r) => {
      const out = stockDepletions(r)
      return out.length
        ? out.map((d) => `${d.place.split('/').pop()} at ~${d.at.toFixed(1)}h${d.recovered ? ' (restocked)' : ''}`).join(', ')
        : 'nothing'
    }]
  ]

  const head = runs.map((run) => `<th scope="col">${escapeHTML(run.name)}</th>`).join('')
  const body = rows.map(([label, get]) => `
    <tr>
      <th scope="row">${label}</th>
      ${runs.map((run) => `<td>${get(run.result)}</td>`).join('')}
    </tr>`).join('')

  // Caveats are the model telling you what this run could not enforce. An empty
  // list is a claim, so it is worth showing when it is not empty.
  const caveats = [...new Set(runs.flatMap((run) => run.result.caveats || []))]
  const refused = runs.filter((run) => run.result.diverged)

  el('results').innerHTML = `
    <div class="table-scroll"><table class="compare">
      <thead><tr><th scope="col"></th>${head}</tr></thead>
      <tbody>${body}</tbody>
    </table></div>
    <p class="note">
      ${runs[0].result.method === 'ssa' ? 'Discrete engine, averaged over 16 runs on one seed.' : 'Continuous engine.'}
      A single run of a queue is an anecdote, so these are means; “worst 5%” is what the person
      standing in the queue experiences.
    </p>
    ${refused.length ? `<div class="caveat"><b>Refused:</b> ${escapeHTML(refused[0].result.reason)}</div>` : ''}
    ${caveats.length ? `<div class="caveat"><b>Not enforced in this run:</b><ul>${
      caveats.map((c) => `<li>${escapeHTML(c)}</li>`).join('')}</ul></div>` : ''}`
}

function wire () {
  for (const id of ['baristas', 'arrivals', 'patience', 'hours']) {
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
