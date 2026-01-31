// Jest setup for E2E tests

// Increase timeout for all tests
jest.setTimeout(120000);

// Add custom matchers if needed
expect.extend({
  /**
   * Check if state has a token in a specific place.
   */
  toHaveTokenIn(received, placeName) {
    const places = received.places || received.state || received;
    const hasToken = places[placeName] > 0;

    return {
      pass: hasToken,
      message: () =>
        `expected state ${hasToken ? 'not ' : ''}to have token in "${placeName}"\n` +
        `Received places: ${JSON.stringify(places, null, 2)}`,
    };
  },

  /**
   * Check if a transition is enabled.
   */
  toHaveTransitionEnabled(received, transitionId) {
    const enabled = received.enabled || received.enabled_transitions || [];
    const isEnabled = enabled.includes(transitionId);

    return {
      pass: isEnabled,
      message: () =>
        `expected transition "${transitionId}" ${isEnabled ? 'not ' : ''}to be enabled\n` +
        `Enabled transitions: ${JSON.stringify(enabled)}`,
    };
  },
});
