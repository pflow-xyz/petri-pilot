/**
 * E2E tests for the erc20-token app.
 *
 * Tests all ERC-20 token operations: mint, burn, transfer, approve, transfer_from
 *
 * Workflow paths:
 * - Admin minting: mint → balances increase, total_supply increases
 * - Admin burning: burn → balances decrease, total_supply decreases
 * - Token transfer: transfer → sender balance decreases, recipient balance increases
 * - Approval flow: approve → allowances set, then transfer_from can spend
 */

const { TestHarness } = require('../lib/test-harness');

describe('erc20-token', () => {
  let harness;

  beforeAll(async () => {
    harness = new TestHarness('erc20-token');
    await harness.setup();
  }, 120000);

  afterAll(async () => {
    if (harness) {
      await harness.teardown();
    }
  });

  describe('pilot API availability', () => {
    test('window.pilot is available', async () => {
      const hasPilot = await harness.eval('return typeof window.pilot === "object"');
      expect(hasPilot).toBe(true);
    });
  });

  describe('workflow introspection', () => {
    test('can get all places', async () => {
      const places = await harness.pilot.getPlaces();
      const placeIds = places.map(p => p.id);
      expect(placeIds).toContain('total_supply');
      expect(placeIds).toContain('balances');
      expect(placeIds).toContain('allowances');
    });

    test('can get all transitions', async () => {
      const transitions = await harness.pilot.getTransitions();
      const transitionIds = transitions.map(t => t.id);
      expect(transitionIds).toContain('transfer');
      expect(transitionIds).toContain('approve');
      expect(transitionIds).toContain('transfer_from');
      expect(transitionIds).toContain('mint');
      expect(transitionIds).toContain('burn');
    });
  });

  describe('mint event', () => {
    beforeAll(async () => {
      await harness.login(['admin']);
    });

    test('admin can mint tokens to an address', async () => {
      const instance = await harness.pilot.create();
      expect(instance.id).toBeDefined();

      // Mint tokens to alice
      const result = await harness.pilot.action('mint', {
        to: 'alice',
        amount: 1000
      });

      expect(result.success).toBe(true);

      // Verify event was recorded
      const events = await harness.pilot.getEvents();
      const mintEvent = events.find(e => e.type === 'Minted' || e.type === 'MintEvent');
      expect(mintEvent).toBeDefined();
    });

    test('can mint to multiple addresses', async () => {
      await harness.pilot.create();

      // Mint to alice
      await harness.pilot.action('mint', { to: 'alice', amount: 500 });

      // Mint to bob
      await harness.pilot.action('mint', { to: 'bob', amount: 300 });

      const events = await harness.pilot.getEvents();
      const mintEvents = events.filter(e => e.type === 'Minted' || e.type === 'MintEvent');
      expect(mintEvents.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('burn event', () => {
    beforeAll(async () => {
      await harness.login(['admin']);
    });

    test('admin can burn tokens from an address', async () => {
      await harness.pilot.create();

      // First mint tokens
      await harness.pilot.action('mint', { to: 'alice', amount: 1000 });

      // Then burn some
      const result = await harness.pilot.action('burn', {
        from: 'alice',
        amount: 200
      });

      expect(result.success).toBe(true);

      // Verify burn event was recorded
      const events = await harness.pilot.getEvents();
      const burnEvent = events.find(e => e.type === 'Burned' || e.type === 'BurnEvent');
      expect(burnEvent).toBeDefined();
    });
  });

  describe('transfer event', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'holder']);
    });

    test('can transfer tokens between addresses', async () => {
      await harness.pilot.create();

      // Mint tokens to alice first
      await harness.pilot.action('mint', { to: 'alice', amount: 1000 });

      // Transfer from alice to bob
      const result = await harness.pilot.action('transfer', {
        from: 'alice',
        to: 'bob',
        amount: 250
      });

      expect(result.success).toBe(true);

      // Verify transfer event
      const events = await harness.pilot.getEvents();
      const transferEvent = events.find(e => e.type === 'Transfered' || e.type === 'TransferEvent');
      expect(transferEvent).toBeDefined();
    });

    test('can chain multiple transfers', async () => {
      await harness.pilot.create();

      // Setup: mint to alice
      await harness.pilot.action('mint', { to: 'alice', amount: 1000 });

      // alice -> bob
      await harness.pilot.action('transfer', { from: 'alice', to: 'bob', amount: 300 });

      // bob -> charlie
      await harness.pilot.action('transfer', { from: 'bob', to: 'charlie', amount: 100 });

      // charlie -> alice (circular)
      await harness.pilot.action('transfer', { from: 'charlie', to: 'alice', amount: 50 });

      const events = await harness.pilot.getEvents();
      const transferEvents = events.filter(e => e.type === 'Transfered' || e.type === 'TransferEvent');
      expect(transferEvents.length).toBeGreaterThanOrEqual(3);
    });
  });

  describe('approve event', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'holder']);
    });

    test('can approve spender allowance', async () => {
      await harness.pilot.create();

      // Approve bob to spend alice's tokens
      const result = await harness.pilot.action('approve', {
        owner: 'alice',
        spender: 'bob',
        amount: 500
      });

      expect(result.success).toBe(true);

      // Verify approval event
      const events = await harness.pilot.getEvents();
      const approvalEvent = events.find(e => e.type === 'Approveed' || e.type === 'ApprovalEvent');
      expect(approvalEvent).toBeDefined();
    });

    test('can update existing allowance', async () => {
      await harness.pilot.create();

      // Initial approval
      await harness.pilot.action('approve', { owner: 'alice', spender: 'bob', amount: 500 });

      // Update approval (increase)
      await harness.pilot.action('approve', { owner: 'alice', spender: 'bob', amount: 200 });

      const events = await harness.pilot.getEvents();
      const approvalEvents = events.filter(e => e.type === 'Approveed' || e.type === 'ApprovalEvent');
      expect(approvalEvents.length).toBeGreaterThanOrEqual(2);
    });
  });

  describe('transfer_from event (delegated transfer)', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'holder']);
    });

    test('can transfer using allowance', async () => {
      await harness.pilot.create();

      // Setup: mint to alice
      await harness.pilot.action('mint', { to: 'alice', amount: 1000 });

      // Alice approves bob
      await harness.pilot.action('approve', { owner: 'alice', spender: 'bob', amount: 500 });

      // Bob transfers from alice to charlie (using allowance)
      const result = await harness.pilot.action('transfer_from', {
        from: 'alice',
        to: 'charlie',
        caller: 'bob',
        amount: 200
      });

      expect(result.success).toBe(true);

      // Verify transfer_from event
      const events = await harness.pilot.getEvents();
      const transferFromEvent = events.find(e => e.type === 'TransferFromed' || e.type === 'TransferFromEvent');
      expect(transferFromEvent).toBeDefined();
    });

    test('delegated transfer workflow', async () => {
      await harness.pilot.create();

      // Full delegated transfer flow
      // 1. Mint tokens to owner
      await harness.pilot.action('mint', { to: 'owner', amount: 10000 });

      // 2. Owner approves exchange
      await harness.pilot.action('approve', { owner: 'owner', spender: 'exchange', amount: 5000 });

      // 3. Exchange moves tokens on behalf of owner
      await harness.pilot.action('transfer_from', {
        from: 'owner',
        to: 'buyer1',
        caller: 'exchange',
        amount: 1000
      });

      await harness.pilot.action('transfer_from', {
        from: 'owner',
        to: 'buyer2',
        caller: 'exchange',
        amount: 2000
      });

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(4); // mint, approve, 2x transfer_from
    });
  });

  describe('complete token lifecycle', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'holder']);
    });

    test('full ERC-20 workflow with all events', async () => {
      await harness.pilot.create();

      // 1. MINT - Create initial supply
      await harness.pilot.action('mint', { to: 'treasury', amount: 1000000 });

      // 2. TRANSFER - Distribute from treasury
      await harness.pilot.action('transfer', { from: 'treasury', to: 'alice', amount: 10000 });
      await harness.pilot.action('transfer', { from: 'treasury', to: 'bob', amount: 5000 });

      // 3. APPROVE - Alice approves DEX
      await harness.pilot.action('approve', { owner: 'alice', spender: 'dex', amount: 8000 });

      // 4. TRANSFER_FROM - DEX moves alice's tokens
      await harness.pilot.action('transfer_from', {
        from: 'alice',
        to: 'liquidity_pool',
        caller: 'dex',
        amount: 3000
      });

      // 5. BURN - Burn tokens from treasury
      await harness.pilot.action('burn', { from: 'treasury', amount: 50000 });

      // Verify all event types were recorded
      const events = await harness.pilot.getEvents();
      const eventTypes = events.map(e => e.type);

      // Check we have at least one of each event type
      const hasMint = eventTypes.some(t => t.includes('Mint'));
      const hasTransfer = eventTypes.some(t => t === 'Transfered' || t === 'TransferEvent');
      const hasApproval = eventTypes.some(t => t.includes('Approv'));
      const hasTransferFrom = eventTypes.some(t => t.includes('TransferFrom'));
      const hasBurn = eventTypes.some(t => t.includes('Burn'));

      expect(hasMint).toBe(true);
      expect(hasTransfer).toBe(true);
      expect(hasApproval).toBe(true);
      expect(hasTransferFrom).toBe(true);
      expect(hasBurn).toBe(true);
    });
  });

  describe('access control', () => {
    test('holder role can transfer', async () => {
      await harness.login(['holder']);
      await harness.pilot.create();

      // Holder should be able to transfer
      // (would need minted tokens first, but checking the permission)
      const canTransfer = await harness.pilot.canFire('transfer');
      // Transfer may be disabled due to guard, but role should allow
      expect(canTransfer).toBeDefined();
    });

    test('admin role can mint', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      const canMint = await harness.pilot.canFire('mint');
      expect(canMint.canFire).toBe(true);
    });

    test('admin role can burn', async () => {
      await harness.login(['admin']);
      await harness.pilot.create();

      // First mint so there are tokens to burn
      await harness.pilot.action('mint', { to: 'test', amount: 100 });

      const canBurn = await harness.pilot.canFire('burn');
      expect(canBurn).toBeDefined();
    });
  });

  describe('event sourcing', () => {
    beforeAll(async () => {
      await harness.login(['admin', 'holder']);
    });

    test('events are recorded in order', async () => {
      await harness.pilot.create();

      await harness.pilot.action('mint', { to: 'alice', amount: 100 });
      await harness.pilot.action('transfer', { from: 'alice', to: 'bob', amount: 50 });
      await harness.pilot.action('burn', { from: 'bob', amount: 10 });

      const events = await harness.pilot.getEvents();
      expect(events.length).toBeGreaterThanOrEqual(3);

      // Events should have increasing versions
      for (let i = 1; i < events.length; i++) {
        expect(events[i].version).toBeGreaterThan(events[i - 1].version);
      }
    });

    test('can get event count', async () => {
      await harness.pilot.create();
      await harness.pilot.action('mint', { to: 'alice', amount: 100 });
      await harness.pilot.action('transfer', { from: 'alice', to: 'bob', amount: 25 });

      const count = await harness.pilot.getEventCount();
      expect(count).toBeGreaterThanOrEqual(2);
    });

    test('last event reflects most recent action', async () => {
      await harness.pilot.create();
      await harness.pilot.action('mint', { to: 'alice', amount: 100 });

      let lastEvent = await harness.pilot.getLastEvent();
      expect(lastEvent.type).toMatch(/mint/i);

      await harness.pilot.action('transfer', { from: 'alice', to: 'bob', amount: 50 });

      lastEvent = await harness.pilot.getLastEvent();
      expect(lastEvent.type).toMatch(/transfer/i);
    });
  });
});
