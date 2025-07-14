import 'vitest';

interface CustomMatchers<R = unknown> {
  toDeepEqualAny(...arrays: string[][]): R;
}

declare module 'vitest' {
  // eslint-disable-next-line
  interface Assertion<T = any> extends CustomMatchers<T> {}
  // eslint-disable-next-line
  interface AsymmetricMatchersContaining extends CustomMatchers {}
}
