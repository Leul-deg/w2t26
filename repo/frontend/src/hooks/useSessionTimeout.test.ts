// Tests for useSessionTimeout: timer behavior, activity reset, and callbacks.

import { renderHook, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { useSessionTimeout } from './useSessionTimeout';

describe('useSessionTimeout', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it('does not fire callbacks when isActive is false', () => {
    const onWarn = vi.fn();
    const onExpire = vi.fn();

    renderHook(() =>
      useSessionTimeout({
        isActive: false,
        warnAfterMs: 1000,
        expireAfterMs: 2000,
        onWarn,
        onExpire,
      }),
    );

    act(() => { vi.advanceTimersByTime(5000); });

    expect(onWarn).not.toHaveBeenCalled();
    expect(onExpire).not.toHaveBeenCalled();
  });

  it('isExpiringSoon starts as false', () => {
    const { result } = renderHook(() =>
      useSessionTimeout({
        isActive: true,
        warnAfterMs: 5000,
        expireAfterMs: 10000,
        onWarn: vi.fn(),
        onExpire: vi.fn(),
      }),
    );

    expect(result.current.isExpiringSoon).toBe(false);
  });

  it('fires onWarn and sets isExpiringSoon after the warn threshold', async () => {
    const onWarn = vi.fn();

    const { result } = renderHook(() =>
      useSessionTimeout({
        isActive: true,
        warnAfterMs: 1000,
        expireAfterMs: 2000,
        onWarn,
        onExpire: vi.fn(),
      }),
    );

    act(() => { vi.advanceTimersByTime(1001); });

    expect(onWarn).toHaveBeenCalledOnce();
    expect(result.current.isExpiringSoon).toBe(true);
  });

  it('fires onExpire and clears isExpiringSoon after the expire threshold', () => {
    const onExpire = vi.fn();

    const { result } = renderHook(() =>
      useSessionTimeout({
        isActive: true,
        warnAfterMs: 1000,
        expireAfterMs: 2000,
        onWarn: vi.fn(),
        onExpire,
      }),
    );

    act(() => { vi.advanceTimersByTime(2001); });

    expect(onExpire).toHaveBeenCalledOnce();
    expect(result.current.isExpiringSoon).toBe(false);
  });

  it('extendSession resets isExpiringSoon and delays onExpire', () => {
    const onExpire = vi.fn();

    const { result } = renderHook(() =>
      useSessionTimeout({
        isActive: true,
        warnAfterMs: 1000,
        expireAfterMs: 2000,
        onWarn: vi.fn(),
        onExpire,
      }),
    );

    // Trigger the warn
    act(() => { vi.advanceTimersByTime(1001); });
    expect(result.current.isExpiringSoon).toBe(true);

    // Extend the session — resets timers
    act(() => { result.current.extendSession(); });
    expect(result.current.isExpiringSoon).toBe(false);

    // Expire timer was reset, so we need another full 2000ms to fire
    act(() => { vi.advanceTimersByTime(1999); });
    expect(onExpire).not.toHaveBeenCalled();

    act(() => { vi.advanceTimersByTime(2); });
    expect(onExpire).toHaveBeenCalledOnce();
  });

  it('does not fire callbacks after isActive becomes false', () => {
    const onWarn = vi.fn();
    const onExpire = vi.fn();
    let active = true;

    const { rerender } = renderHook(() =>
      useSessionTimeout({
        isActive: active,
        warnAfterMs: 1000,
        expireAfterMs: 2000,
        onWarn,
        onExpire,
      }),
    );

    // Set inactive before timers fire
    active = false;
    act(() => { rerender(); });

    act(() => { vi.advanceTimersByTime(3000); });

    expect(onWarn).not.toHaveBeenCalled();
    expect(onExpire).not.toHaveBeenCalled();
  });
});
