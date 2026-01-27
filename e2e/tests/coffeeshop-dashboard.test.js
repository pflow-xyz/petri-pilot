/**
 * E2E tests for the coffeeshop dashboard app-level behavior.
 *
 * Tests the interactive dashboard features:
 * - Health state classification (healthy, busy, stressed, sla_crisis, inventory_crisis, critical)
 * - Simulation controls (play, pause, reset, speed)
 * - Restock functionality
 * - Presets and test state application
 * - Component initialization
 *
 * SKIPPED: The dashboardPilot API is not currently generated. These tests require
 * a custom dashboard implementation that was removed. See ROADMAP.md for plans
 * to reintroduce prediction/simulation dashboard features.
 */

const { TestHarness } = require('../lib/test-harness');

describe.skip('coffeeshop dashboard', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('coffeeshop');
    await harness.setup();

    // Navigate to dashboard and wait for initialization
    await harness.eval(`
      window.navigate('/');
      return true;
    `);

    // Wait for dashboard to initialize
    await harness.eval(`
      return new Promise((resolve) => {
        const check = () => {
          if (window.dashboardPilot && window.dashboardPilot.isInitialized()) {
            resolve(true);
          } else {
            setTimeout(check, 100);
          }
        };
        setTimeout(check, 500);
      });
    `);
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('dashboard initialization', () => {
    test('dashboardPilot API is available', async () => {
      const hasPilot = await harness.eval('return typeof window.dashboardPilot === "object"');
      expect(hasPilot).toBe(true);
    });

    test('dashboard is initialized', async () => {
      const isInit = await harness.eval('return window.dashboardPilot.isInitialized()');
      expect(isInit).toBe(true);
    });

    test('all components are present', async () => {
      const components = await harness.eval('return window.dashboardPilot.getComponents()');
      expect(components.scene).toBe(true);
      expect(components.gauges).toBe(true);
      expect(components.flow).toBe(true);
      expect(components.rate).toBe(true);
      expect(components.controls).toBe(true);
      expect(components.stress).toBe(true);
      expect(components.stats).toBe(true);
    });

    test('simulation state has instanceId', async () => {
      const state = await harness.eval('return window.dashboardPilot.getSimulationState()');
      expect(state.instanceId).toBeDefined();
      expect(state.instanceId).not.toBeNull();
    });
  });

  describe('health state classification', () => {
    beforeEach(async () => {
      // Pause simulation before each test to prevent state changes
      await harness.eval('return window.dashboardPilot.pause()');
    });

    test('healthy state - good resources, low queue', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('healthy');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('healthy');
      expect(result.state.coffee_beans).toBe(1000);
      expect(result.state.orders_pending).toBe(0);
    });

    test('busy state - queue > 5', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('busy');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('busy');
      expect(result.state.orders_pending).toBe(7);
    });

    test('stressed state - queue > 10', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('stressed');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('stressed');
      expect(result.state.orders_pending).toBe(12);
    });

    test('SLA crisis state - breach rate > 30%', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('sla_crisis');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('sla_crisis');
      // 20/(40+20) = 33% breach rate
      expect(result.state.orders_pending).toBe(20);
      expect(result.state.orders_complete).toBe(40);
    });

    test('inventory crisis state - resources < 10%', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('inventory_crisis');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('inventory_crisis');
      // 150/2000 = 7.5% coffee beans
      expect(result.state.coffee_beans).toBe(150);
    });

    test('critical state - resource depleted', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('critical');
      `);
      expect(result.health).toBeDefined();
      expect(result.health.key).toBe('critical');
      expect(result.state.coffee_beans).toBe(0);
    });

    test('health state has emoji, label, and description', async () => {
      const health = await harness.eval(`
        window.dashboardPilot.applyTestState('healthy');
        return window.dashboardPilot.getHealth();
      `);
      expect(health.emoji).toBeDefined();
      expect(health.label).toBeDefined();
      expect(health.description).toBeDefined();
      expect(health.severity).toBeDefined();
    });

    test('health states have increasing severity', async () => {
      const states = ['healthy', 'busy', 'stressed', 'critical'];
      const severities = [];

      for (const state of states) {
        const result = await harness.eval(`
          window.dashboardPilot.applyTestState('${state}');
          return window.dashboardPilot.getHealth();
        `);
        severities.push(result.severity);
      }

      // Each state should have equal or higher severity than previous
      for (let i = 1; i < severities.length; i++) {
        expect(severities[i]).toBeGreaterThanOrEqual(severities[i - 1]);
      }
    });
  });

  describe('simulation controls', () => {
    test('can pause simulation', async () => {
      const result = await harness.eval('return window.dashboardPilot.pause()');
      expect(result.playing).toBe(false);

      const isPlaying = await harness.eval('return window.dashboardPilot.isPlaying()');
      expect(isPlaying).toBe(false);
    });

    test('can play simulation', async () => {
      const result = await harness.eval('return window.dashboardPilot.play()');
      expect(result.playing).toBe(true);

      const isPlaying = await harness.eval('return window.dashboardPilot.isPlaying()');
      expect(isPlaying).toBe(true);

      // Pause again for other tests
      await harness.eval('return window.dashboardPilot.pause()');
    });

    test('can set simulation speed', async () => {
      const speeds = [1, 2, 5, 10];

      for (const speed of speeds) {
        const result = await harness.eval(`return window.dashboardPilot.setSpeed(${speed})`);
        expect(result.speed).toBe(speed);

        const state = await harness.eval('return window.dashboardPilot.getSimulationState()');
        expect(state.speed).toBe(speed);
      }
    });

    test('can reset simulation', async () => {
      // First, modify state
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 100,
          milk: 50,
          cups: 25
        });
        return true;
      `);

      const beforeReset = await harness.eval('return window.dashboardPilot.getResources()');
      expect(beforeReset.coffee_beans).toBe(100);

      // Reset
      const result = await harness.eval('return window.dashboardPilot.reset()');
      expect(result).toBeDefined();

      // After reset, resources should be at max
      const afterReset = await harness.eval('return window.dashboardPilot.getResources()');
      expect(afterReset.coffee_beans).toBe(2000);
      expect(afterReset.milk).toBe(1000);
      expect(afterReset.cups).toBe(500);
    });
  });

  describe('restock functionality', () => {
    beforeEach(async () => {
      // Pause and set low resources
      await harness.eval(`
        window.dashboardPilot.pause();
        window.dashboardPilot.setState({
          coffee_beans: 100,
          milk: 50,
          cups: 25
        });
        return true;
      `);
    });

    test('can restock coffee beans', async () => {
      const result = await harness.eval(`return window.dashboardPilot.restock('coffee_beans')`);
      expect(result.coffee_beans).toBe(2000);
      expect(result.milk).toBe(50); // Unchanged
      expect(result.cups).toBe(25); // Unchanged
    });

    test('can restock milk', async () => {
      const result = await harness.eval(`return window.dashboardPilot.restock('milk')`);
      expect(result.coffee_beans).toBe(100); // Unchanged
      expect(result.milk).toBe(1000);
      expect(result.cups).toBe(25); // Unchanged
    });

    test('can restock cups', async () => {
      const result = await harness.eval(`return window.dashboardPilot.restock('cups')`);
      expect(result.coffee_beans).toBe(100); // Unchanged
      expect(result.milk).toBe(50); // Unchanged
      expect(result.cups).toBe(500);
    });

    test('can restock all resources', async () => {
      const result = await harness.eval('return window.dashboardPilot.restockAll()');
      expect(result.coffee_beans).toBe(2000);
      expect(result.milk).toBe(1000);
      expect(result.cups).toBe(500);
    });

    test('restock updates UI without resetting simulation', async () => {
      // Set specific orders state
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 100,
          orders_pending: 5,
          orders_complete: 20
        });
        return true;
      `);

      // Restock coffee
      await harness.eval(`return window.dashboardPilot.restock('coffee_beans')`);

      // Orders should be unchanged
      const orders = await harness.eval('return window.dashboardPilot.getOrders()');
      expect(orders.orders_pending).toBe(5);
      expect(orders.orders_complete).toBe(20);
    });
  });

  describe('state manipulation', () => {
    beforeEach(async () => {
      await harness.eval('return window.dashboardPilot.pause()');
    });

    test('can set arbitrary state', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.setState({
          coffee_beans: 500,
          milk: 250,
          cups: 100,
          orders_pending: 10,
          orders_complete: 50
        });
      `);

      expect(result.currentState.coffee_beans).toBe(500);
      expect(result.currentState.milk).toBe(250);
      expect(result.currentState.cups).toBe(100);
      expect(result.currentState.orders_pending).toBe(10);
      expect(result.currentState.orders_complete).toBe(50);
    });

    test('state changes affect health classification', async () => {
      // Set healthy state
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000,
          milk: 500,
          cups: 200,
          orders_pending: 0,
          orders_complete: 50
        });
        return true;
      `);

      let health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('healthy');

      // Change to high queue (busy)
      await harness.eval(`
        window.dashboardPilot.setState({ orders_pending: 8 });
        return true;
      `);

      health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('busy');
    });
  });

  describe('event rates', () => {
    test('can get current rates', async () => {
      const rates = await harness.eval('return window.dashboardPilot.getRates()');
      expect(rates).toBeDefined();
      expect(typeof rates.order_espresso).toBe('number');
      expect(typeof rates.order_latte).toBe('number');
      expect(typeof rates.order_cappuccino).toBe('number');
    });

    test('can set specific rates', async () => {
      const newRates = {
        order_espresso: 100,
        order_latte: 50,
        order_cappuccino: 25
      };

      const result = await harness.eval(`
        return window.dashboardPilot.setRates(${JSON.stringify(newRates)});
      `);

      expect(result.order_espresso).toBe(100);
      expect(result.order_latte).toBe(50);
      expect(result.order_cappuccino).toBe(25);
    });

    test('rates persist after being set', async () => {
      await harness.eval(`
        window.dashboardPilot.setRates({ order_espresso: 200 });
        return true;
      `);

      const rates = await harness.eval('return window.dashboardPilot.getRates()');
      expect(rates.order_espresso).toBe(200);
    });
  });

  describe('presets', () => {
    test('can apply morning-rush preset', async () => {
      await harness.eval(`return window.dashboardPilot.applyPreset('morning-rush')`);
      const rates = await harness.eval('return window.dashboardPilot.getRates()');

      // Morning rush should have higher rates
      expect(rates.order_espresso).toBeGreaterThan(0);
      expect(rates.order_latte).toBeGreaterThan(0);
    });

    test('can apply steady preset', async () => {
      await harness.eval(`return window.dashboardPilot.applyPreset('steady')`);
      const rates = await harness.eval('return window.dashboardPilot.getRates()');

      expect(rates.order_espresso).toBeGreaterThan(0);
    });

    test('can apply quiet preset', async () => {
      await harness.eval(`return window.dashboardPilot.applyPreset('quiet')`);
      const rates = await harness.eval('return window.dashboardPilot.getRates()');

      // Quiet should have lower rates
      expect(rates.order_espresso).toBeGreaterThan(0);
    });
  });

  describe('test states for health verification', () => {
    test('all test states produce expected health classification', async () => {
      const testCases = [
        { state: 'healthy', expectedKey: 'healthy' },
        { state: 'busy', expectedKey: 'busy' },
        { state: 'stressed', expectedKey: 'stressed' },
        { state: 'sla_crisis', expectedKey: 'sla_crisis' },
        { state: 'inventory_crisis', expectedKey: 'inventory_crisis' },
        { state: 'critical', expectedKey: 'critical' }
      ];

      for (const { state, expectedKey } of testCases) {
        const result = await harness.eval(`
          return window.dashboardPilot.applyTestState('${state}');
        `);
        expect(result.health.key).toBe(expectedKey);
      }
    });

    test('invalid test state returns error', async () => {
      const result = await harness.eval(`
        return window.dashboardPilot.applyTestState('invalid_state');
      `);
      expect(result.error).toBeDefined();
      expect(result.error).toContain('Unknown test state');
    });
  });

  describe('stats tracking', () => {
    test('can get simulation stats', async () => {
      const stats = await harness.eval('return window.dashboardPilot.getStats()');
      expect(stats).toBeDefined();
      expect(typeof stats.drinksServed).toBe('number');
      expect(typeof stats.ordersPerHour).toBe('number');
    });

    test('stats are included in simulation state', async () => {
      const state = await harness.eval('return window.dashboardPilot.getSimulationState()');
      expect(state.stats).toBeDefined();
    });
  });

  describe('order flow', () => {
    beforeEach(async () => {
      // Reset and pause
      await harness.eval(`
        window.dashboardPilot.pause();
        window.dashboardPilot.reset();
        return true;
      `);
    });

    test('can get order state', async () => {
      const orders = await harness.eval('return window.dashboardPilot.getOrders()');
      expect(orders).toBeDefined();
      expect(typeof orders.orders_pending).toBe('number');
      expect(typeof orders.espresso_ready).toBe('number');
      expect(typeof orders.latte_ready).toBe('number');
      expect(typeof orders.cappuccino_ready).toBe('number');
      expect(typeof orders.orders_complete).toBe('number');
    });

    test('orders_pending affects health state', async () => {
      // Low pending orders - healthy
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000, milk: 500, cups: 200,
          orders_pending: 2, orders_complete: 100
        });
        return true;
      `);
      let health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('healthy');

      // High pending orders - busy
      await harness.eval(`
        window.dashboardPilot.setState({ orders_pending: 8 });
        return true;
      `);
      health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('busy');

      // Very high pending orders - stressed
      await harness.eval(`
        window.dashboardPilot.setState({ orders_pending: 15 });
        return true;
      `);
      health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('stressed');
    });
  });

  describe('resource depletion scenarios', () => {
    beforeEach(async () => {
      await harness.eval('return window.dashboardPilot.pause()');
    });

    test('depleted coffee beans triggers critical', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 0,
          milk: 500,
          cups: 200,
          orders_pending: 0,
          orders_complete: 10
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('critical');
    });

    test('depleted milk triggers critical', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000,
          milk: 0,
          cups: 200,
          orders_pending: 0,
          orders_complete: 10
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('critical');
    });

    test('depleted cups triggers critical', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000,
          milk: 500,
          cups: 0,
          orders_pending: 0,
          orders_complete: 10
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('critical');
    });

    test('low inventory (< 10%) triggers inventory crisis', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 150,  // 7.5% of 2000
          milk: 500,
          cups: 200,
          orders_pending: 0,
          orders_complete: 10
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('inventory_crisis');
    });
  });

  describe('SLA breach scenarios', () => {
    beforeEach(async () => {
      await harness.eval('return window.dashboardPilot.pause()');
    });

    test('high breach rate (> 30%) triggers SLA crisis', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000,
          milk: 500,
          cups: 200,
          orders_pending: 15,
          orders_complete: 30  // 15/(30+15) = 33% breach
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('sla_crisis');
    });

    test('moderate breach rate (> 5%) triggers busy', async () => {
      await harness.eval(`
        window.dashboardPilot.setState({
          coffee_beans: 1000,
          milk: 500,
          cups: 200,
          orders_pending: 2,
          orders_complete: 20  // 2/(20+2) = 9% breach
        });
        return true;
      `);

      const health = await harness.eval('return window.dashboardPilot.getHealth()');
      expect(health.key).toBe('busy');
    });
  });
});
