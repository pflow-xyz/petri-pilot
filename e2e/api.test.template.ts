// API Tests Template for {{APP_NAME}}
// Copilot: Replace {{PLACEHOLDERS}} with actual values

import { describe, it, expect, beforeAll } from 'vitest';

const BASE_URL = process.env.API_URL || 'http://localhost:8080';

describe('{{APP_NAME}} API', () => {
  let aggregateId: string;

  describe('POST /api/{{MODEL_NAME}}', () => {
    it('creates a new aggregate', async () => {
      const res = await fetch(`${BASE_URL}/api/{{MODEL_NAME}}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({})
      });

      expect(res.status).toBe(201);
      const data = await res.json();
      expect(data.aggregate_id).toBeDefined();
      expect(data.enabled_transitions).toContain('{{FIRST_TRANSITION}}');

      aggregateId = data.aggregate_id;
    });
  });

  describe('GET /api/{{MODEL_NAME}}/{id}', () => {
    it('returns aggregate state', async () => {
      const res = await fetch(`${BASE_URL}/api/{{MODEL_NAME}}/${aggregateId}`);

      expect(res.status).toBe(200);
      const data = await res.json();
      expect(data.aggregate_id).toBe(aggregateId);
      expect(data.places).toBeDefined();
    });

    it('returns 404 for unknown id', async () => {
      const res = await fetch(`${BASE_URL}/api/{{MODEL_NAME}}/unknown-id`);
      expect(res.status).toBe(404);
    });
  });

  // Copilot: Generate tests for each transition
  // {{#TRANSITIONS}}
  describe('POST /api/{{MODEL_NAME}}/{{TRANSITION_ID}}', () => {
    it('executes {{TRANSITION_ID}} transition', async () => {
      const res = await fetch(`${BASE_URL}/api/{{MODEL_NAME}}/{{TRANSITION_ID}}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          aggregate_id: aggregateId,
          data: {
            // Copilot: Add required transition data
          }
        })
      });

      // Copilot: Adjust expected status based on preconditions
      expect([200, 409]).toContain(res.status);
    });
  });
  // {{/TRANSITIONS}}

  describe('Access Control', () => {
    // Copilot: Generate tests for protected transitions
    it('requires authentication for protected transitions', async () => {
      const res = await fetch(`${BASE_URL}/api/{{MODEL_NAME}}/{{PROTECTED_TRANSITION}}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ aggregate_id: aggregateId })
      });

      expect(res.status).toBe(401);
    });
  });
});
