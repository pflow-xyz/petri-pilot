/**
 * E2E tests for the Texas Hold'em frontend.
 * 
 * Tests the custom poker interface with ODE-based strategic analysis.
 * Unlike other tests, this tests the custom frontend in frontends/texas-holdem/
 */

const { TestHarness } = require('../lib/test-harness');

describe('texas-holdem frontend', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('texas-holdem');
    await harness.setup();
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('basic game flow', () => {
    beforeAll(async () => {
      // Login with required roles
      await harness.login(['admin', 'user']);
    });

    test('should load poker table', async () => {
      // Navigate to custom frontend
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Wait for page to load
      await harness.page.waitForSelector('.poker-table', { timeout: 10000 });
      
      const pokerTable = await harness.page.$('.poker-table');
      expect(pokerTable).toBeTruthy();
      
      // Check for 5 player seats
      const seats = await harness.page.$$('.player-seat');
      expect(seats.length).toBe(5);
    });

    test('should create new game', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Click new game button
      await harness.page.waitForSelector('#new-game-btn', { timeout: 10000 });
      await harness.page.click('#new-game-btn');
      
      // Wait for game to be created
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      
      // Verify start hand button is visible
      const startHandBtn = await harness.page.$('#start-hand-btn');
      const isVisible = await startHandBtn.isIntersectingViewport();
      expect(isVisible).toBe(true);
    });

    test('should display initial game state', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Create game
      await harness.page.click('#new-game-btn');
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      
      // Check pot display
      const potDisplay = await harness.page.$eval('#pot-display', el => el.textContent);
      expect(potDisplay).toContain('$');
      
      // Check round indicator
      const roundIndicator = await harness.page.$eval('#round-indicator', el => el.textContent);
      expect(roundIndicator).toContain('WAITING');
      
      // Check all players have $1000
      const chipAmounts = await harness.page.$$eval('.player-chips', els => 
        els.map(el => el.textContent)
      );
      expect(chipAmounts.length).toBe(5);
      chipAmounts.forEach(amount => {
        expect(amount).toContain('$1000');
      });
    });

    test('should start hand and deal preflop', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Create game and start hand
      await harness.page.click('#new-game-btn');
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      await harness.page.click('#start-hand-btn');
      
      // Wait for preflop to be dealt (round should change)
      await harness.page.waitForTimeout(2000);
      
      // Check that round changed from waiting
      const roundIndicator = await harness.page.$eval('#round-indicator', el => el.textContent);
      expect(roundIndicator).not.toContain('WAITING');
    });

    test('should display community card placeholders', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Check for 5 community card slots
      const cards = await harness.page.$$('#community-cards .card');
      expect(cards.length).toBe(5);
      
      // Initially should be placeholders
      const hasPlaceholders = await harness.page.$('#community-cards .card-placeholder');
      expect(hasPlaceholders).toBeTruthy();
    });

    test('should show dealer button', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      await harness.page.click('#new-game-btn');
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      
      // Check that at least one dealer button is visible
      const dealerButtons = await harness.page.$$('.dealer-button');
      expect(dealerButtons.length).toBeGreaterThan(0);
    });
  });

  describe('ODE integration', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'user']);
    });

    test('should have ODE solver loaded', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Check that Solver module is loaded
      const hasSolver = await harness.page.evaluate(() => {
        return typeof window.Solver !== 'undefined';
      });
      
      // Note: The Solver is imported as a module, so it might not be on window
      // Instead, check if the ODE functions are available
      const hasODEFunctions = await harness.page.evaluate(() => {
        return typeof window.runODESimulation === 'function' &&
               typeof window.buildPokerODEPetriNet === 'function' &&
               typeof window.solveODE === 'function';
      });
      
      expect(hasODEFunctions).toBe(true);
    });

    test('should toggle ODE mode', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Get initial mode
      const initialMode = await harness.page.$eval('#ode-mode', el => el.textContent);
      expect(initialMode).toBe('Local');
      
      // Toggle mode
      await harness.page.click('#toggle-ode-btn');
      
      // Check mode changed
      const newMode = await harness.page.$eval('#ode-mode', el => el.textContent);
      expect(newMode).toBe('API');
    });

    test('should build Petri net model', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Create a game first
      await harness.page.click('#new-game-btn');
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      
      // Call buildPokerODEPetriNet
      const model = await harness.page.evaluate(() => {
        const action = { id: 'p0_fold', type: 'fold', amount: 0 };
        return window.buildPokerODEPetriNet(window.gameState, action);
      });
      
      expect(model).toBeDefined();
      expect(model['@type']).toBe('PetriNet');
      expect(model.places).toBeDefined();
      expect(model.transitions).toBeDefined();
      expect(model.arcs).toBeDefined();
    });
  });

  describe('UI interactions', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'user']);
    });

    test('should toggle heatmap overlay', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Click toggle heatmap button
      await harness.page.click('#toggle-heatmap-btn');
      
      // Check overlay is visible
      const overlay = await harness.page.$('#heatmap-overlay');
      const isVisible = await overlay.isIntersectingViewport();
      expect(isVisible).toBe(true);
      
      // Click again to hide
      await harness.page.click('#toggle-heatmap-btn');
      
      // Check overlay is hidden
      const display = await harness.page.$eval('#heatmap-overlay', el => 
        window.getComputedStyle(el).display
      );
      expect(display).toBe('none');
    });

    test('should display game state info', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      await harness.page.click('#new-game-btn');
      await harness.page.waitForSelector('#start-hand-btn', { timeout: 5000 });
      
      // Check game state info is updated
      const gameStateInfo = await harness.page.$eval('#game-state-info', el => el.textContent);
      expect(gameStateInfo).toContain('Game ID');
      expect(gameStateInfo).toContain('Round');
      expect(gameStateInfo).toContain('Pot');
    });

    test('should show player seats with correct layout', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Check each seat has the correct class
      for (let i = 0; i < 5; i++) {
        const seat = await harness.page.$(`#seat-${i}`);
        expect(seat).toBeTruthy();
        
        const hasCorrectClass = await harness.page.evaluate((index) => {
          const seat = document.getElementById(`seat-${index}`);
          return seat.classList.contains(`seat-${index}`);
        }, i);
        
        expect(hasCorrectClass).toBe(true);
      }
    });
  });

  describe('responsive design', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'user']);
    });

    test('should work on mobile viewport', async () => {
      await harness.page.goto(`${harness.server.baseUrl}/frontends/texas-holdem/`, {
        waitUntil: 'networkidle0'
      });

      // Set mobile viewport
      await harness.page.setViewport({ width: 375, height: 667 });
      
      // Check poker table is still visible
      const pokerTable = await harness.page.$('.poker-table');
      const isVisible = await pokerTable.isIntersectingViewport();
      expect(isVisible).toBe(true);
      
      // Restore viewport
      await harness.page.setViewport({ width: 1280, height: 720 });
    });
  });
});
