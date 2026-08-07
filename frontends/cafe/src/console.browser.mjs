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
  check(patience === '5', 'patience seeded from the abandon rate (60/12)')

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
  // The first extra barista must help materially — that is the claim. Beyond
  // that the shop stops being staffing-limited and the curve flattens into
  // stochastic noise, so demanding a monotonic rise would be asserting something
  // the model is right to refuse. The knee IS the answer.
  check(s.length === 3 && s[1] > s[0], 'the first extra barista serves more drinks')
  check(s.length === 3 && Math.abs(s[2] - s[1]) / s[1] < 0.1,
    'the second extra barista adds little — the shop is no longer staffing-limited')
  check(w.length === 3 && w[0] > w[1] && w[1] >= w[2], 'more baristas lose fewer customers')
  check(b.length === 3 && b[0] > b[1] && b[1] > b[2], 'utilization falls monotonically as staffing rises')

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
    // Patience is the abandon rate, not a colour on an order card.
    const minutes = Number(await page.locator('#patience').inputValue())
    check(Math.abs(body.rates['counter/abandon'] - 60 / minutes) < 1e-9,
      `a ${minutes}-minute patience is an abandon rate of 60/${minutes}`)
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
