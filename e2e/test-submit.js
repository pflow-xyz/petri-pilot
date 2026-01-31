const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.connectOverCDP('http://localhost:9222');
  const contexts = browser.contexts();
  const page = contexts[0].pages().find(p => p.url().includes('blog-post'));

  // Clear cache
  const client = await page.context().newCDPSession(page);
  await client.send('Network.clearBrowserCache');

  // Capture console
  const logs = [];
  page.on('console', msg => logs.push(msg.text()));

  await page.goto('https://pilot.pflow.xyz/blog-post/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  console.log('=== Posts ===');
  const titles = await page.$$eval('.post-title', els => els.map(e => e.textContent));
  console.log(titles);

  // Click first post
  await page.click('.post-card');
  await page.waitForTimeout(1000);

  const status = await page.$eval('.post-status', el => el.textContent);
  console.log('Current status:', status);

  // Try submit
  const submitBtn = await page.$('button[data-action="submit"]');
  if (submitBtn) {
    console.log('Clicking Submit...');
    await submitBtn.click();
    await page.waitForTimeout(2000);

    const newStatus = await page.$eval('.post-status', el => el.textContent);
    console.log('New status:', newStatus);

    // Check new actions
    const actions = await page.$$eval('#detail-actions button', els => els.map(e => e.textContent));
    console.log('Available actions:', actions);
  }

  if (logs.length > 0) {
    console.log('\nConsole logs:', logs);
  }

  await browser.close();
})();
