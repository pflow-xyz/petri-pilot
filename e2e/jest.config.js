module.exports = {
  testEnvironment: 'node',
  testTimeout: 120000,
  testMatch: ['**/tests/**/*.test.js'],
  // Run tests serially since each test file starts its own server
  maxWorkers: 1,
  // Setup file for global configs
  setupFilesAfterEnv: ['./jest.setup.js'],
};
