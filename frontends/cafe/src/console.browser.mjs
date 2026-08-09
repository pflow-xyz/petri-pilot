/**
 * Browser check for the café console, driven by Playwright.
 *
 * This is the only check on this frontend, and it starts its own server.
 *
 * There used to be a second one — a DOM stub that imported console.js and
 * asserted the controls bound to names the model has. Every assertion it made,
 * this one makes by loading the real page, so it was a second implementation of
 * one definition. Worse, it was the *weaker* implementation: the console fetched
 * an absolute "/api/rates" while the app is mounted under "/cafe/", so every
 * request 404'd and the page rendered empty — and the stub passed throughout,
 * because it was always handed a base URL and so never exercised the default.
 * A check that cannot see the page cannot tell you the page is dead.
 *
 * The reason to keep it was that it needed no browser and so could run in CI.
 * It never did: CI runs `go test`, not `make test`. So it bought nothing that
 * this does not, and cost a way to be confidently wrong.
 *
 *   make e2e-install     # once: Playwright and its browser
 *   make test-browser    # builds, serves on a free port, checks, tears down
 *
 * Pass BASE (or argv[2]) to point at a server that is already running instead.
 */
import { createRequire } from 'node:module'
import { spawn } from 'node:child_process'
import { createServer } from 'node:net'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { existsSync } from 'node:fs'

const SHOT = process.env.SHOT_DIR || tmpdir()
const REPO = fileURLToPath(new URL('../../../', import.meta.url))

// Resolved from e2e/ by default; PLAYWRIGHT overrides for an install elsewhere.
const require = createRequire(import.meta.url)
const candidates = [
  process.env.PLAYWRIGHT,
  join(REPO, 'e2e/node_modules/playwright'),
  'playwright'
].filter(Boolean)

let chromium
for (const candidate of candidates) {
  try {
    ({ chromium } = require(candidate))
    break
  } catch { /* try the next one */ }
}
if (!chromium) {
  console.error('could not load Playwright. Install it once with:\n' +
    '  make e2e-install\n' +
    '(that also fetches the browser, which npm install alone does not), ' +
    'or point PLAYWRIGHT at an install elsewhere.')
  process.exit(2)
}

/** A port nothing is listening on. */
function freePort () {
  return new Promise((ok, err) => {
    const probe = createServer()
    probe.on('error', err)
    probe.listen(0, '127.0.0.1', () => {
      const { port } = probe.address()
      probe.close(() => ok(port))
    })
  })
}

/**
 * Start the café app and wait for it to answer.
 *
 * Serving it here rather than asking the operator to run `make dev-run` in
 * another shell is what makes this one command. A check that needs a second
 * terminal is a check that gets skipped.
 */
async function serveCafe () {
  const binary = resolve(REPO, 'petri-pilot')
  if (!existsSync(binary)) {
    console.error(`no ${binary} — run \`make build\` first (or \`make test-browser\`, which does).`)
    process.exit(2)
  }
  const port = await freePort()
  const child = spawn(binary, ['serve', '-port', String(port), 'cafe'], {
    cwd: REPO, stdio: ['ignore', 'pipe', 'pipe'], detached: false
  })
  let log = ''
  child.stdout.on('data', (d) => { log += d })
  child.stderr.on('data', (d) => { log += d })

  // Kill the server however this process ends — a failed assertion, an
  // exception, a Ctrl-C, or a closed pipe. Without this an interrupted run
  // leaves a listener behind, and the next one silently checks a stale binary.
  const stop = () => { if (child.exitCode === null) child.kill('SIGKILL') }
  process.on('exit', stop)
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => { stop(); process.exit(130) })
  }
  process.on('uncaughtException', (err) => { stop(); console.error(err); process.exit(1) })
  // `| head` closes stdout; without this the EPIPE is unhandled and the exit
  // hooks never run.
  process.stdout.on('error', () => { stop(); process.exit(0) })

  const base = `http://127.0.0.1:${port}/cafe/`
  const deadline = Date.now() + 30000
  while (Date.now() < deadline) {
    if (child.exitCode !== null) {
      console.error(`the server exited with ${child.exitCode}:\n${log}`)
      process.exit(2)
    }
    try {
      const res = await fetch(base, { signal: AbortSignal.timeout(1000) })
      if (res.ok) return { base, stop }
    } catch { /* not up yet */ }
    await new Promise((r) => setTimeout(r, 200))
  }
  child.kill('SIGKILL')
  console.error(`the server never answered on ${base}:\n${log}`)
  process.exit(2)
}

const external = process.argv[2] || process.env.BASE
const server = external ? { base: external, stop: () => {} } : await serveCafe()
const BASE = server.base

const fail = []
const note = (m) => console.log(m)
const check = (ok, m) => { if (!ok) fail.push(m); console.log(`${ok ? 'PASS' : 'FAIL'}  ${m}`) }

// A dead page makes the later steps time out rather than fail cleanly, so
// report what did fail before dying. Without this the run ends on a raw
// TimeoutError and the assertions that already caught the problem scroll past
// unread.
const summarise = (err) => {
  if (err) console.error(`\n${err.stack || err}`)
  console.log(`\n${fail.length === 0 ? 'ALL CHECKS PASSED' : `${fail.length} FAILED:`}`)
  fail.forEach((f) => console.log(`  - ${f}`))
  process.exit(fail.length === 0 && !err ? 0 : 1)
}
process.on('unhandledRejection', summarise)

const browser = await chromium.launch()
const page = await browser.newPage({ viewport: { width: 1280, height: 1000 } })

const consoleErrors = []
const failedRequests = []
page.on('console', (m) => { if (m.type() === 'error') consoleErrors.push(m.text()) })
page.on('pageerror', (e) => consoleErrors.push(`pageerror: ${e.message}`))
page.on('requestfailed', (r) => failedRequests.push(`${r.method()} ${r.url()} — ${r.failure()?.errorText}`))

const posts = []
page.on('request', (r) => { if (r.method() === 'POST') posts.push({ url: r.url(), body: r.postData() }) })
const responses = []
page.on('response', (r) => responses.push({ url: r.url(), status: r.status() }))

// A top-level await rejection in ESM bypasses both unhandledRejection and
// uncaughtException, so the run would end on a raw stack with the assertions
// that already caught the problem scrolled off. Catch it here instead.
try {
  note(`\n--- loading ${BASE}`)
  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })

  // The page runs a comparison on load. Wait for the table it renders.
  await page.waitForSelector('table.compare', { timeout: 20000 }).catch(() => {})

  check(await page.locator('table.compare').count() > 0, 'comparison table rendered on load')
  check(consoleErrors.length === 0, `no console errors (${consoleErrors.length})`)
  consoleErrors.forEach((e) => note(`      ! ${e}`))
  check(failedRequests.length === 0, `no failed requests (${failedRequests.length})`)
  failedRequests.forEach((e) => note(`      ! ${e}`))

  // Requests must land under the mount prefix. An absolute "/api/rates" asks the
  // server root, which does not have it — the page comes up empty and every
  // control still looks fine. This is the assertion the DOM-stub check could not
  // make, because it was always handed a base URL.
  const apiCalls = responses.filter((r) => r.url.includes('/api/'))
  check(apiCalls.length > 0, 'the page called the API at all')
  check(apiCalls.every((r) => new URL(r.url).pathname.startsWith('/cafe/')),
    'API calls go under the /cafe/ mount, not the server root')
  apiCalls.filter((r) => !new URL(r.url).pathname.startsWith('/cafe/'))
    .forEach((r) => note(`      ! ${r.url}`))

  const bad = responses.filter((r) => r.status >= 400)
  check(bad.length === 0, `no 4xx/5xx responses (${bad.length})`)
  bad.forEach((r) => note(`      ! ${r.status} ${r.url}`))

  // Sliders must be seeded from /api/rates, not from the HTML defaults.
  const baristas = await page.locator('#baristas').inputValue()
  const arrivals = await page.locator('#arrivals').inputValue()
  const patience = await page.locator('#patience').inputValue()
  note(`      controls after load: baristas=${baristas} arrivals=${arrivals} patience=${patience}`)
  // The HTML ships 1/20/10 on purpose; the model says 2/33/5. These assertions
  // are only meaningful because the two differ.
  check(baristas === '2', 'baristas seeded from the model (initial staff/available)')
  check(arrivals === '33', 'arrivals seeded from the model (10+15+8)')
  check(patience === '5', 'patience seeded from an abandon_* rate (60/12)')

  // The comparison must show three columns with different answers.
  const headers = await page.locator('table.compare thead th').allTextContents()
  note(`      columns: ${JSON.stringify(headers)}`)
  check(headers.length === 4, 'three scenarios plus the row-label column')

  const rowValues = async (label) => {
    const row = page.locator('table.compare tbody tr', { has: page.locator(`th:text-is("${label}")`) })
    return (await row.locator('td').allTextContents()).map((s) => s.trim())
  }
  const served = await rowValues('Drinks served')
  const walked = await rowValues('Customers who left')
  const busy = await rowValues('Baristas busy')
  note(`      served:  ${served.join('  ')}`)
  note(`      walked:  ${walked.join('  ')}`)
  note(`      busy:    ${busy.join('  ')}`)

  const nums = (a) => a.map((s) => Number(s.replace(/[^0-9.]/g, '')))
  const s = nums(served); const w = nums(walked); const b = nums(busy)
  // The first extra barista must help materially, and the second must help
  // less. Diminishing returns is the whole claim — it is what makes the table a
  // budget rather than an argument for hiring forever — and it is a comparison
  // between the two hires rather than a threshold on the second, because where
  // the knee falls moves with the arrival rate the slider is set to. It used to
  // be asserted as "under 10%", which was a true statement about a shop whose
  // milk supply capped it at 220 drinks however many people were behind the
  // counter.
  check(s.length === 3 && s[1] > s[0], 'the first extra barista serves more drinks')
  check(s.length === 3 && (s[2] - s[1]) < (s[1] - s[0]) * 0.7,
    `the second extra barista adds less than the first (+${s[2] - s[1]} against +${s[1] - s[0]})`)
  check(w.length === 3 && w[0] > w[1] && w[1] >= w[2], 'more baristas lose fewer customers')
  check(b.length === 3 && b[0] > b[1] && b[1] > b[2], 'utilization falls monotonically as staffing rises')

  // The served mix has to track the ordered mix. With one fungible queue it did
  // not have to: every start_X raced on the same place, so which drink got made
  // was decided by whichever recipe's ingredients multiplied out largest — the
  // shop served more espressos than were ordered and made almost no cappuccinos.
  // A queue per drink makes that arithmetically impossible rather than unlikely,
  // so this is a structural claim and not a tolerance on noise.
  const mix = await rowValues('Ordered → served')
  note(`      ordered → served: ${JSON.stringify(mix)}`)
  const pairs = mix.flatMap((cell) => [...cell.matchAll(/([a-z]+)\s+([\d,]+)\s*→\s*([\d,]+)/g)]
    .map(([, drink, ordered, served]) => ({ drink, ordered: Number(ordered.replace(/,/g, '')), served: Number(served.replace(/,/g, '')) })))
  check(pairs.length === 9, `the mix row breaks out three drinks per scenario (${pairs.length} of 9)`)
  check(pairs.length > 0 && pairs.every((p) => p.served <= p.ordered),
    'no drink is served more often than it was ordered')
  check(pairs.length > 0 && pairs.every((p) => p.served > 0),
    'every drink on the menu actually gets made')

  // What the shop was short of, which the table had no way of saying until the
  // engine started measuring it. "Ran out" only catches a place that empties and
  // stays empty, so the shipped café's milk — consumed as fast as it was
  // delivered, average never near zero — showed up nowhere, and the honest
  // reading of the table was "eight idle baristas losing half the customers,
  // cause unknown". At the model's staffing the answer has to be the baristas,
  // and no ingredient may appear at all.
  const waiting = await rowValues('Waiting on')
  note(`      waiting on: ${JSON.stringify(waiting)}`)
  check(waiting.length === 3 && /available/.test(waiting[0]),
    `the smallest shop is waiting on its baristas (${waiting[0]})`)
  check(waiting.every((cell) => !/milk|beans|cups/.test(cell)),
    'no scenario is waiting on the pantry — this shop is staffing-limited, as intended')
  check(waiting.every((cell) => !/pending_|_ready|brewing_/.test(cell)),
    'no scenario is "waiting on" an empty queue — that is a quiet shop, not a constraint')

  // The row above is a filter on the engine's classification, so check the
  // classification itself rather than trusting the row to have hidden a bad
  // one. The console used to do this by dropping anything whose place ID began
  // with the queue subnet's prefix, which is a naming convention standing in
  // for a structural fact — it works for this bundle and silently classifies
  // every place in a single-net model as a queue. Now `kind` is on the wire and
  // the ordering is the engine's: capacity constraints first, whatever the
  // fractions say. At this staffing the empty order queues run 80-90% against a
  // staff pool nearer 30%, so a list ranked on fraction alone would put the
  // shop's idleness above the thing deciding its throughput.
  const contended = await page.evaluate(async () => {
    const res = await fetch('api/scenario', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ marking: { 'staff/available': 2 }, hours: 8, realizations: 8, seed: 20260807 })
    })
    return (await res.json()).contended || []
  })
  note(`      contended: ${JSON.stringify(contended.map((c) => `${c.place} ${c.kind} ${Math.round(c.fraction * 100)}%`))}`)
  check(contended.length > 0, 'the run reports what it was waiting for')
  check(contended.every((c) => ['conserved', 'bounded', 'queue', 'state'].includes(c.kind)),
    'every contention carries a supply kind')
  check(contended[0]?.place === 'staff/available' && contended[0]?.kind === 'conserved',
    `the staff pool is the top finding (${contended[0]?.place} / ${contended[0]?.kind})`)
  const isCapacity = (c) => c.kind === 'conserved' || c.kind === 'bounded'
  const firstQueue = contended.findIndex((c) => !isCapacity(c))
  const lastCapacity = contended.map(isCapacity).lastIndexOf(true)
  check(firstQueue === -1 || lastCapacity < firstQueue,
    'no queue outranks a capacity constraint')
  check(contended.some((c) => c.kind === 'queue' && c.fraction > contended[0].fraction),
    'and the ranking is doing work — some queue waited longer than the top capacity finding')

  // Two admissions, two headings. The exponential-service note is a property of
  // the Gillespie engine, true of every model it runs and unfixable by editing
  // the net — so it is an assumption, not a constraint the run failed to
  // enforce. It used to arrive in `caveats` and render under "Not enforced in
  // this run:", which both mislabelled it and, because every SSA scenario
  // carries one, meant that heading appeared on every result and an empty
  // caveat list became unreachable. The café's model is one the engine fully
  // honours, so the honest rendering has the caveat block absent entirely.
  const admissions = await page.evaluate(async () => {
    const res = await fetch('api/scenario', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ marking: { 'staff/available': 2 }, hours: 1, realizations: 2, seed: 20260807 })
    })
    const body = await res.json()
    return { caveats: body.caveats || [], assumptions: body.assumptions || [] }
  })
  note(`      caveats: ${admissions.caveats.length}  assumptions: ${admissions.assumptions.length}`)
  check(admissions.assumptions.some((a) => /exponentially distributed/.test(a)),
    'the service-time note is reported as a method assumption')
  check(admissions.caveats.length === 0,
    `nothing is filed as unenforced for a model the engine fully honours (${admissions.caveats.length})`)

  const headings = await page.locator('.caveat b').allTextContents()
  note(`      admission headings: ${JSON.stringify(headings)}`)
  check(headings.some((h) => /What this method assumes/.test(h)),
    'the assumption renders under its own heading')
  check(!headings.some((h) => /Not enforced/.test(h)),
    'and not under "Not enforced in this run", which this run has nothing to say under')

  await page.screenshot({ path: join(SHOT, 'cafe-load.png'), fullPage: true })

  // --- move a control and re-run -------------------------------------------
  note('\n--- setting baristas to 1 and running a single scenario')
  await page.locator('#baristas').fill('1')
  await page.locator('#baristas').dispatchEvent('input')
  check((await page.locator('#baristas-out').textContent()).trim() === '1', 'the output label follows the slider')

  posts.length = 0
  await page.locator('#run').click()
  await page.waitForFunction(() => document.querySelector('#status')?.textContent === '', null, { timeout: 20000 })
  await page.waitForSelector('table.compare', { timeout: 20000 })

  check(posts.length === 1, `one POST for a single run (${posts.length})`)
  if (posts[0]) {
    const body = JSON.parse(posts[0].body)
    note(`      POST ${new URL(posts[0].url).pathname}`)
    note(`      marking: ${JSON.stringify(body.marking)}`)
    check(body.marking['staff/available'] === 1, 'the slider reached the request as staff/available')
    check(!body.schedule, 'no schedule when the rush toggle is off')
  }

  // Every name the console sends must exist in the model. A rename in the net
  // that the UI did not follow is otherwise invisible: the request succeeds in
  // shape and the server is the only thing that objects.
  const declared = await page.evaluate(async () => {
    const res = await fetch('api/rates')
    const { rates, initial } = await res.json()
    return { rates: Object.keys(rates), places: Object.keys(initial) }
  })
  if (posts[0]) {
    const body = JSON.parse(posts[0].body)
    const names = [...Object.keys(body.rates || {}), ...Object.keys(body.schedule || {})]
    const unknownRates = names.filter((n) => !declared.rates.includes(n))
    const unknownPlaces = Object.keys(body.marking || {}).filter((p) => !declared.places.includes(p))
    check(unknownRates.length === 0, `every rate the console sets is a real transition${unknownRates.length ? `: ${unknownRates}` : ''}`)
    check(unknownPlaces.length === 0, `every place the console sets is a real place${unknownPlaces.length ? `: ${unknownPlaces}` : ''}`)
    // Patience is the abandon rate, not a colour on an order card — and there is
    // one queue per drink, so it has to reach all three. Checking a single key
    // would pass while two queues quietly kept the model's declared patience,
    // which is the failure mode a per-drink split introduces.
    const minutes = Number(await page.locator('#patience').inputValue())
    const abandons = Object.keys(body.rates || {}).filter((n) => n.includes('abandon'))
    note(`      abandon rates set: ${JSON.stringify(abandons)}`)
    check(abandons.length === 3, `patience reaches every queue (${abandons.length} of 3)`)
    check(abandons.length > 0 && abandons.every((n) => Math.abs(body.rates[n] - 60 / minutes) < 1e-9),
      `a ${minutes}-minute patience is an abandon rate of 60/${minutes} on every queue`)
  }

  // --- the rush toggle ------------------------------------------------------
  note('\n--- enabling the morning rush')
  await page.locator('#rush').check()
  posts.length = 0
  await page.locator('#compare').click()
  await page.waitForFunction(() => document.querySelector('#status')?.textContent === '', null, { timeout: 30000 })

  check(posts.length === 1, `one POST for a comparison (${posts.length})`)
  if (posts[0]) {
    const body = JSON.parse(posts[0].body)
    const first = body.scenarios?.[0]
    check(Boolean(first?.schedule), 'the rush toggle produced a schedule')
    const segs = first?.schedule ? Object.values(first.schedule)[0] : null
    if (segs) {
      note(`      schedule segment 0: ${JSON.stringify(segs[0])}  segment 1: ${JSON.stringify(segs[1])}`)
      check(segs.length === 2 && segs[0].value > segs[1].value, 'the rush segment is busier than the lull')
    }
    const seeds = new Set(body.scenarios.map((x) => x.seed))
    check(seeds.size === 1, `all scenarios share one seed (${[...seeds].join(', ')})`)
  }

  const rushServed = await rowValues('Drinks served')
  const rushWalked = await rowValues('Customers who left')
  note(`      under a rush — served: ${rushServed.join('  ')}   walked: ${rushWalked.join('  ')}`)

  // A rush is the interval where staffing binds, so this box is the one place
  // the question is actually being asked — and it was the one path that could
  // not answer it. A scheduled run is assembled segment by segment, and the
  // assembly never carried the contention ledger across: every rush reported
  // "waiting on nothing" for a shop at 87% utilization, which is exactly the
  // silence — an empty list, and nothing anywhere naming the cause — that the
  // row was added to eliminate.
  const rushWaiting = await rowValues('Waiting on')
  note(`      under a rush — waiting on: ${JSON.stringify(rushWaiting)}`)
  check(rushWaiting.length === 3 && rushWaiting.every((cell) => cell !== 'nothing'),
    'a scheduled run still names what it was waiting for')
  check(/available/.test(rushWaiting[0]), `the smallest shop is waiting on its baristas (${rushWaiting[0]})`)
  await page.screenshot({ path: join(SHOT, 'cafe-rush.png'), fullPage: true })

  // --- a refused scenario should be shown, not swallowed --------------------
  note('\n--- a scenario the server refuses')
  const refusal = await page.evaluate(async () => {
    const res = await fetch('api/scenario', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ marking: { 'staff/baristas': 3 }, hours: 1 })
    })
    return { status: res.status, body: await res.text() }
  })
  check(refusal.status === 400, `an unknown place is a 400 (got ${refusal.status})`)
  check(refusal.body.includes('staff/baristas'), 'the refusal names the offending place')

  // A pool emptying is every barista being busy, not the shop running out of
  // staff. It belongs in the utilization row, and must not be listed as stock.
  const ranOut = await rowValues('Ran out')
  note(`      ran out: ${JSON.stringify(ranOut)}`)
  check(!ranOut.some((v) => v.includes('available') || v.includes('busy')),
    'the barista pool is not reported as stock that ran out')

  // --- narrow viewport ------------------------------------------------------
  await page.setViewportSize({ width: 390, height: 844 })
  await page.waitForTimeout(300)
  const overflow = await page.evaluate(() =>
    document.documentElement.scrollWidth - document.documentElement.clientWidth)
  check(overflow <= 1, `no horizontal page scroll at 390px (overflow ${overflow}px)`)
  await page.screenshot({ path: join(SHOT, 'cafe-mobile.png'), fullPage: true })

  await browser.close()
  server.stop()
  summarise()
} catch (err) {
  await browser.close().catch(() => {})
  server.stop()
  summarise(err)
}
