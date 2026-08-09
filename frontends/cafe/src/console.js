/**
 * Café what-if console.
 *
 * Every control here binds to something the Petri net actually has. The barista
 * slider is the token count in `staff/available`; the patience slider is the
 * rate of every `abandon_<drink>` transition — the queue is one place per drink,
 * so giving up is one transition per queue; the rush toggle is a piecewise rate
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

/**
 * Give-up transitions, one per queue, discovered the same way.
 *
 * There used to be exactly one, `counter/abandon`, and the console named it as a
 * literal. Splitting the queue per drink split it into three, and a literal
 * would have kept setting a rate that no longer exists: the slider would have
 * moved, the request would have been accepted, and two of the three queues would
 * have quietly kept the model's declared patience.
 */
let abandonTransitions = []

/**
 * The queue places, and the drink each one belongs to.
 *
 * One place per drink is the whole point of the split — with a single fungible
 * queue, which drink got made was decided by whichever recipe's ingredients
 * multiplied out largest, not by what anyone ordered.
 */
let queues = []

async function loadModel () {
  const res = await fetch(url('/api/rates'))
  if (!res.ok) throw new Error(`GET /api/rates: HTTP ${res.status}`)
  model = await res.json()

  orderTransitions = Object.keys(model.rates)
    .filter((id) => id.includes('order_'))
    .sort()
  abandonTransitions = Object.keys(model.rates)
    .filter((id) => id.includes('abandon'))
    .sort()
  queues = Object.keys(model.initial)
    .filter((p) => /(^|\/)pending_/.test(p))
    .sort()
    .map((place) => ({ place, drink: place.replace(/^.*pending_/, '') }))

  // Seed the controls from the net, so the page opens showing what the model
  // actually says rather than a number someone typed into the HTML.
  const arrivals = orderTransitions.reduce((sum, id) => sum + (model.rates[id] || 0), 0)
  if (arrivals > 0) setSlider('arrivals', Math.round(arrivals))
  if (model.initial['staff/available'] != null) {
    setSlider('baristas', model.initial['staff/available'])
  }
  // The queues declare the same patience, so any of them seeds the slider.
  const abandon = model.rates[abandonTransitions[0]]
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
  // times an hour they would give up. Patience is a property of the customer,
  // not of the drink, so every queue gets the same one.
  for (const id of abandonTransitions) {
    rates[id] = 60 / Math.max(1, patienceMinutes)
  }

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

// Queue lengths live below 1 in a shop that is keeping up, and rounding those to
// a column of zeroes hides the difference the staffing slider makes.
const round1 = (v) => (v == null ? '—' : v.toFixed(1))

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

/**
 * What kept orders waiting, and for how much of the day.
 *
 * "Ran out" cannot answer this and was the only thing here that tried. It
 * reports a place that empties and stays empty, so a resource consumed exactly
 * as fast as it is delivered — refilled, drained, refilled, its average never
 * near zero — is invisible to it. The café shipped in that state on milk, and
 * the console said so nowhere: eight baristas idle two thirds of the day, half
 * the customers lost, "Ran out: nothing". This row is the engine's Contended
 * list, which measures how much of the run each place spent being the only
 * thing a firing was waiting for.
 *
 * Queue places are dropped. An empty queue is also "the only thing missing",
 * and it is true — the shop was waiting for customers — but it is not a
 * shortage anyone can go and fix, and it sits at the top of the raw list every
 * time and buries the answer. Which places those are is the engine's call
 * (`kind`), not this console's: the filter here used to be "drop anything
 * prefixed with the queue places' own subnet", which happened to work for this
 * one composed bundle and classified every place in a single-net model as a
 * queue. The engine decides it structurally and already ranks capacity first,
 * so this is a filter and not a re-ranking.
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

/**
 * Peak of the total queue — what "how bad did it get" means.
 *
 * The three queues share one time grid, so summing them point by point gives the
 * trajectory of the total queue, and its maximum is that trajectory's peak.
 * Every series is already a mean over realizations, so this is the peak of the
 * average shop and not the worst moment any single run had — which is what the
 * row said when there was one queue, and still says now.
 */
function peakQueue (result) {
  const series = queues
    .map(({ place }) => (result.series || []).find((s) => s.place === place))
    .filter(Boolean)
  if (series.length === 0) return null
  const total = series[0].values.map((_, i) =>
    series.reduce((sum, s) => sum + (s.values[i] || 0), 0))
  return total.reduce((top, v) => (v > top ? v : top), 0)
}

/**
 * Mean of the total queue.
 *
 * Exact: expectation is linear, so the mean of the sum IS the sum of the means,
 * whatever the queues do to each other.
 */
function meanQueue (result) {
  const means = result.metrics?.mean
  if (!means) return null
  return queues.reduce((sum, { place }) => sum + (means[place] || 0), 0)
}

/**
 * Worst 5% per queue, reported per drink rather than as one number.
 *
 * There deliberately is no total here. A percentile is not additive — the three
 * queues do not hit their bad moments together, so summing their P95s overstates
 * the total and taking the largest understates it — and the engine returns only
 * per-place percentiles, so the P95 of the sum cannot be recovered from what we
 * have. Reporting three exact numbers beats reporting one invented one.
 */
function worstPerQueue (result) {
  const p95 = result.metrics?.p95
  if (!p95) return '—'
  return queues
    .map(({ place, drink }) => `${escapeHTML(drink)} ${round1(p95[place])}`)
    .join('<br>')
}

/**
 * What was ordered against what was served, per drink.
 *
 * This is the row the split exists for. Throughput counts firings, so
 * `order_<drink>` is demand and `serve_<drink>` is what the shop actually got
 * out — and under one fungible queue those two lists did not even have to
 * resemble each other, because which drink got brewed was settled by ingredient
 * arithmetic rather than by anyone asking for it.
 */
function orderedVersusServed (result) {
  const th = result.metrics?.throughput
  if (!th) return '—'
  const find = (verb, drink) =>
    Object.keys(th).find((id) => id.endsWith(`${verb}_${drink}`))
  return queues
    .map(({ drink }) => {
      const ordered = th[find('order', drink)]
      const served = th[find('serve', drink)]
      return `${escapeHTML(drink)} ${round(ordered)} → ${round(served)}`
    })
    .join('<br>')
}

function render (runs) {
  const rows = [
    ['Drinks served', (r) => round(r.final['counter/orders_complete'])],
    ['Ordered → served', orderedVersusServed],
    ['Customers who left', (r) => round(r.final['counter/walked_out'])],
    ['Queue — typical', (r) => round1(meanQueue(r))],
    // "peak" averages the runs and then takes the maximum; "worst 5%" takes the
    // percentile of the runs themselves, so it can and does come out larger.
    // Labelling them apart is cheaper than a reader deciding the table is wrong.
    ['Queue — peak, average day', (r) => round1(peakQueue(r))],
    ['Queue — worst 5%, per drink', worstPerQueue],
    ['Baristas busy', (r) => {
      const u = r.metrics?.utilization?.staff
      return u == null ? '—' : `${Math.round(u * 100)}%`
    }],
    ['Waiting on', waitedOn],
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

  // Two different admissions, under two different headings.
  //
  // Caveats are the model telling you what this run could not enforce — an
  // empty list is a claim, so it is worth showing when it is not empty.
  // Assumptions are what the arithmetic had to assume whatever the model said,
  // and no edit to the net removes one. They used to arrive in the same list
  // and render under "Not enforced in this run:", which read a correct
  // statement about the engine as a defect in the run — and, because every SSA
  // scenario carries one, meant that heading was never absent and so never
  // told anyone anything.
  const caveats = [...new Set(runs.flatMap((run) => run.result.caveats || []))]
  const assumptions = [...new Set(runs.flatMap((run) => run.result.assumptions || []))]
  const refused = runs.filter((run) => run.result.diverged)

  el('results').innerHTML = `
    <div class="table-scroll"><table class="compare">
      <thead><tr><th scope="col"></th>${head}</tr></thead>
      <tbody>${body}</tbody>
    </table></div>
    <p class="note">
      ${runs[0].result.method === 'ssa' ? 'Discrete engine, averaged over 16 runs on one seed.' : 'Continuous engine.'}
      A single run of a queue is an anecdote, so these are means; “worst 5%” is what the person
      standing in the queue experiences on a bad day, and it is per drink because there is a queue
      per drink — the three do not queue up at the same moment, so there is no honest total.
    </p>
    ${refused.length ? `<div class="caveat"><b>Refused:</b> ${escapeHTML(refused[0].result.reason)}</div>` : ''}
    ${caveats.length ? `<div class="caveat"><b>Not enforced in this run:</b><ul>${
      caveats.map((c) => `<li>${escapeHTML(c)}</li>`).join('')}</ul></div>` : ''}
    ${assumptions.length ? `<div class="caveat assumption"><b>What this method assumes:</b><ul>${
      assumptions.map((a) => `<li>${escapeHTML(a)}</li>`).join('')}</ul></div>` : ''}`
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
