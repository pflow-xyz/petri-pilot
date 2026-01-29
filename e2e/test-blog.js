const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.connectOverCDP('http://localhost:9222');
  const contexts = browser.contexts();
  const page = contexts[0].pages().find(p => p.url().includes('blog-post'));

  if (!page) {
    console.log('Blog page not found');
    await browser.close();
    return;
  }

  // Clear cache and reload
  const client = await page.context().newCDPSession(page);
  await client.send('Network.clearBrowserCache');
  await page.goto('https://pilot.pflow.xyz/blog-post/', { waitUntil: 'networkidle' });
  await page.waitForTimeout(2000);

  console.log('=== 1. LIST VIEW ===');
  const posts = await page.$$('.post-card');
  console.log('Posts found:', posts.length);
  for (const post of posts) {
    const title = await post.$eval('.post-title', el => el.textContent);
    const author = await post.$eval('.post-meta span:first-child', el => el.textContent);
    console.log('  -', title, '|', author);
  }

  console.log('\n=== 2. CREATE NEW POST ===');
  await page.click('#new-post-link');
  await page.waitForTimeout(500);

  // Fill in the form
  await page.fill('#post-title-input', 'Test Post from Playwright');
  await page.fill('#post-content-input', 'This is automated test content created by Playwright.');
  await page.fill('#post-tags-input', 'test, playwright, automation');

  // Save
  await page.click('#save-btn');
  await page.waitForTimeout(3000);

  // Check if we're back at list view with new post
  const newPosts = await page.$$('.post-card');
  console.log('Posts after create:', newPosts.length);

  // Find our new post
  const titles = await page.$$eval('.post-title', els => els.map(e => e.textContent));
  console.log('Titles:', titles);

  const hasNewPost = titles.includes('Test Post from Playwright');
  console.log('New post visible:', hasNewPost);

  console.log('\n=== 3. VIEW POST DETAIL ===');
  // Click on the last post card (most recently created)
  const lastPostCard = await page.$('.post-card:last-child');
  if (lastPostCard) {
    await lastPostCard.click();
    await page.waitForTimeout(1000);

    const detailTitle = await page.$eval('#detail-title', el => el.textContent).catch(() => 'not found');
    const detailContent = await page.$eval('#detail-content', el => el.textContent).catch(() => 'not found');
    const detailAuthor = await page.$eval('#detail-author', el => el.textContent).catch(() => 'not found');

    console.log('Detail title:', detailTitle);
    console.log('Detail content:', detailContent.substring(0, 60) + '...');
    console.log('Detail author:', detailAuthor);

    // Check actions
    const actions = await page.$$eval('#detail-actions button', els => els.map(e => e.textContent));
    console.log('Available actions:', actions);

    console.log('\n=== 4. TEST SUBMIT FOR REVIEW ===');
    const submitBtn = await page.$('button[data-action="submit"]');
    if (submitBtn) {
      await submitBtn.click();
      await page.waitForTimeout(1000);

      const newStatus = await page.$eval('.post-status', el => el.textContent).catch(() => 'unknown');
      console.log('Status after submit:', newStatus);

      const newActions = await page.$$eval('#detail-actions button', els => els.map(e => e.textContent));
      console.log('New actions:', newActions);
    }
  }

  console.log('\n=== DONE ===');
  await browser.close();
})();
