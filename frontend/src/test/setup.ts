import '@testing-library/jest-dom';

// jsdom lacks ResizeObserver, which recharts' ResponsiveContainer needs.
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
const g = globalThis as unknown as { ResizeObserver?: unknown };
g.ResizeObserver = g.ResizeObserver || ResizeObserverStub;
