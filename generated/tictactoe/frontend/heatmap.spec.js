import { test, expect } from '@playwright/test'

test.describe('ODE Heatmap Tests', () => {
  test('JS ODE matches Go API values', async ({ page }) => {
    // Navigate to test page
    await page.goto('/test.html')

    // Wait for page to load
    await page.waitForLoadState('networkidle')

    // Click "Compare JS vs API" button
    await page.click('button:has-text("Compare JS vs API")')

    // Wait for results (ODE computation takes a few seconds)
    await page.waitForFunction(
      () => document.getElementById('output').textContent.includes('Position'),
      { timeout: 30000 }
    )

    // Get the output text
    const output = await page.locator('#output').textContent()
    console.log('\n' + output)

    // Parse results and check each position
    const lines = output.split('\n')
    const results = []

    for (const line of lines) {
      const match = line.match(/^\s*(\d\d)\s*\|\s*([\d.-]+)\s*\|\s*([\d.-]+)\s*\|\s*([\d.]+)\s*([✓✗])/)
      if (match) {
        results.push({
          pos: match[1],
          jsVal: parseFloat(match[2]),
          apiVal: parseFloat(match[3]),
          diff: parseFloat(match[4]),
          passed: match[5] === '✓'
        })
      }
    }

    // Verify we got results for all 9 positions
    expect(results.length).toBe(9)

    // Check that all positions match within tolerance
    for (const r of results) {
      console.log(`Position ${r.pos}: JS=${r.jsVal.toFixed(4)}, API=${r.apiVal.toFixed(4)}, diff=${r.diff.toFixed(6)}`)
      expect(r.diff).toBeLessThan(0.01)
    }

    // Verify expected value ranges
    const center = results.find(r => r.pos === '11')
    const corner = results.find(r => r.pos === '00')
    const edge = results.find(r => r.pos === '01')

    expect(center.jsVal).toBeGreaterThan(0.4)
    expect(corner.jsVal).toBeGreaterThan(0.3)
    expect(edge.jsVal).toBeGreaterThan(0.2)

    // Verify ordering: center > corner > edge
    expect(center.jsVal).toBeGreaterThan(corner.jsVal)
    expect(corner.jsVal).toBeGreaterThan(edge.jsVal)
  })

  test('Run full test suite', async ({ page }) => {
    await page.goto('/test.html')
    await page.waitForLoadState('networkidle')

    // Click "Run Tests" button
    await page.click('button:has-text("Run Tests")')

    // Wait for tests to complete
    await page.waitForFunction(
      () => document.getElementById('output').textContent.includes('Tests Complete'),
      { timeout: 30000 }
    )

    const output = await page.locator('#output').textContent()
    console.log('\n' + output)

    // Check all tests passed
    expect(output).toContain('Test 1 PASSED')
    expect(output).toContain('Test 2 PASSED')
    expect(output).toContain('Test 3 PASSED')
    expect(output).toContain('Test 4 PASSED')
    expect(output).not.toContain('FAILED')
  })
})
