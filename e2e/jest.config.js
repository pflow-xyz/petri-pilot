module.exports = {
  testEnvironment: 'node',
  testTimeout: 60000,
  testMatch: ['**/tests/**/*.test.js'],
  // Run tests serially since each test file starts its own server on the same port
  maxWorkers: 1,
};
