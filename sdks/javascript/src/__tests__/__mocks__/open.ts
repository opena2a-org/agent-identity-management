// Mock implementation of the 'open' package for testing
export default jest.fn(async (url: string) => {
  console.log(`Mock: Opening URL ${url}`);
  return Promise.resolve();
});
