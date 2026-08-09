/**
 * Browser check for the vet-clinic what-if console, driven by Playwright.
 *
 * Same shape as the café's console.browser.mjs and for the same reasons: it
 * starts its own server, loads the real page, and asserts against what the
 * page actually did — a check that is handed a base URL never exercises the
 * default mount, which is exactly where the café console once died silently.
 *
 *   make e2e-install         # once: Playwright and its browser
 *   make test-browser-vet    # builds, serves on a free port, checks, tears down
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
const REPO = fileURLToPath(new URL('../../', import.meta.url))

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
    'or point PLAYWRIGHT at an install elsewhere.')
  process.exit(2)
}

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

async function serveClinic () {
  const binary = resolve(REPO, 'petri-pilot')
  if (!existsSync(binary)) {
    console.error(`no ${binary} — run \`make build\` first (or \`make test-browser-vet\`, which does).`)
    process.exit(2)
  }
  const port = await freePort()
  const child = spawn(binary, ['serve', '-port', String(port), 'vet-clinic'], {
    cwd: REPO, stdio: ['ignore', 'pipe', 'pipe'], detached: false
  })
  let log = ''
  child.stdout.on('data', (d) => { log += d })
  child.stderr.on('data', (d) => { log += d })

  const stop = () => { if (child.exitCode === null) child.kill('SIGKILL') }
  process.on('exit', stop)
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => { stop(); process.exit(130) })
  }
  process.on('uncaughtException', (err) => { stop(); console.error(err); process.exit(1) })
  process.stdout.on('error', () => { stop(); process.exit(0) })

  const base = `http://127.0.0.1:${port}/vet-clinic/`
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
const server = external ? { base: external, stop: () => {} } : await serveClinic()
const BASE = server.base

const fail = []
const note = (m) => console.log(m)
const check = (ok, m) => { if (!ok) fail.push(m); console.log(`${ok ? 'PASS' : 'FAIL'}  ${m}`) }

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
const responses = []
page.on('response', (r) => responses.push({ url: r.url(), status: r.status() }))

try {
  note(`\n--- loading ${BASE}`)
  await page.goto(BASE, { waitUntil: 'networkidle', timeout: 30000 })

  // The console runs a comparison on load (even while its tab is hidden), so
  // the table should exist before we ever click the tab.
  await page.waitForSelector('table.whatif-compare', { timeout: 20000 }).catch(() => {})
  await page.click('#view-whatif-btn')

  check(await page.locator('table.whatif-compare').count() > 0, 'comparison table rendered on load')
  check(consoleErrors.length === 0, `no console errors (${consoleErrors.length})`)
  consoleErrors.forEach((e) => note(`      ! ${e}`))
  check(failedRequests.length === 0, `no failed requests (${failedRequests.length})`)
  failedRequests.forEach((e) => note(`      ! ${e}`))

  const apiCalls = responses.filter((r) => r.url.includes('/api/'))
  check(apiCalls.length > 0, 'the page called the API at all')
  check(apiCalls.every((r) => new URL(r.url).pathname.startsWith('/vet-clinic/')),
    'API calls go under the /vet-clinic/ mount, not the server root')
  apiCalls.filter((r) => !new URL(r.url).pathname.startsWith('/vet-clinic/'))
    .forEach((r) => note(`      ! ${r.url}`))

  const bad = responses.filter((r) => r.status >= 400)
  check(bad.length === 0, `no 4xx/5xx responses (${bad.length})`)
  bad.forEach((r) => note(`      ! ${r.status} ${r.url}`))

  // Sliders must be seeded from /api/rates, not from the HTML defaults. The
  // HTML ships 1 DVM / 1 RVT / 2 receptionists / 12 arrivals / 60 min on
  // purpose; the model says 2 / 3 / 1 / 6 / 30. These assertions are only
  // meaningful because the two differ.
  const val = (id) => page.locator(`#${id}`).inputValue()
  const dvms = await val('dvms')
  const rvts = await val('rvts')
  const receptionists = await val('receptionists')
  const arrivals = await val('arrivals')
  const patience = await val('patience')
  note(`      controls after load: dvms=${dvms} rvts=${rvts} recep=${receptionists} arrivals=${arrivals} patience=${patience}`)
  check(dvms === '2', 'DVMs seeded from the model (initial dvm_avail)')
  check(rvts === '3', 'RVTs seeded from the model (initial rvt_avail)')
  check(receptionists === '1', 'receptionists seeded from the model')
  check(arrivals === '6', 'arrivals seeded from the model (patient_arrives rate)')
  check(patience === '30', 'patience seeded from an abandon_* rate (60/2)')

  const headers = await page.locator('table.whatif-compare thead th').allTextContents()
  note(`      columns: ${JSON.stringify(headers)}`)
  check(headers.length === 4, 'three scenarios plus the row-label column')

  const rowValues = async (label) => {
    const row = page.locator('table.whatif-compare tbody tr', { has: page.locator(`th:text-is("${label}")`) })
    return (await row.locator('td').allTextContents()).map((s) => s.trim())
  }
  const nums = (a) => a.map((s) => Number(s.replace(/[^0-9.]/g, '')))

  const discharged = nums(await rowValues('Patients discharged'))
  const walked = nums(await rowValues('Walked out unseen'))
  note(`      discharged: ${discharged.join('  ')}  walked: ${walked.join('  ')}`)
  check(walked.length === 3 && walked[0] >= walked[1] && walked[1] >= walked[2],
    'more staff never loses more patients')
  check(walked.length === 3 && walked[0] > walked[2],
    'the staffing ladder moves walkouts at all')

  const waiting = await rowValues('Waiting on')
  note(`      waiting on: ${JSON.stringify(waiting)}`)
  check(waiting.every((cell) => !/wait_/.test(cell)),
    'no scenario is "waiting on" an empty queue')

  // Disruption round trip through the real controls: five emergencies at the
  // door plus the wave, run as a single scenario. The emergency machinery has
  // to show up in the table — diversions become possible and the routine day
  // gets worse, which is what the inhibitor arcs are for.
  const baselineWalked = walked[0]
  await page.fill('#emergencies', '5')
  await page.check('#emergency-stream')
  await page.click('#run')
  await page.waitForFunction(() => {
    const el = document.getElementById('status')
    return el && el.textContent === ''
  }, { timeout: 30000 })

  const crisisWalked = nums(await rowValues('Walked out unseen'))[0]
  const crisisDiverted = nums(await rowValues('Emergencies diverted'))[0]
  note(`      crisis: walked=${crisisWalked} (baseline ${baselineWalked}) diverted=${crisisDiverted}`)
  check(crisisWalked > baselineWalked, 'an emergency wave raises routine walkouts')
  check(!Number.isNaN(crisisDiverted), 'the diverted row renders under a crisis')

  // "No surgery today" must zero the procedures row — the read-arc gate, driven
  // from the UI.
  await page.fill('#emergencies', '0')
  await page.uncheck('#emergency-stream')
  await page.check('#no-surgery')
  await page.click('#run')
  await page.waitForFunction(() => {
    const el = document.getElementById('status')
    return el && el.textContent === ''
  }, { timeout: 30000 })
  const procedures = nums(await rowValues('Procedures completed'))[0]
  note(`      procedures with the gate closed: ${procedures}`)
  check(procedures === 0, 'closing the surgery gate completes zero procedures')

  // Nothing scrolls sideways at phone width.
  await page.setViewportSize({ width: 390, height: 844 })
  const overflow = await page.evaluate(() =>
    document.documentElement.scrollWidth - document.documentElement.clientWidth)
  check(overflow <= 0, `no horizontal scroll at 390px (overflow ${overflow}px)`)

  const shot = join(SHOT, 'vet-clinic-whatif.png')
  await page.setViewportSize({ width: 1280, height: 1000 })
  await page.screenshot({ path: shot, fullPage: false })
  note(`      screenshot: ${shot}`)

  await browser.close()
  server.stop()
  summarise()
} catch (err) {
  await browser.close().catch(() => {})
  server.stop()
  summarise(err)
}
