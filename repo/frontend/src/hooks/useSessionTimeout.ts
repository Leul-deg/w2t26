// useSessionTimeout tracks user inactivity and fires callbacks when the session
// is about to expire or has expired.
//
// Activity events (mousemove, keydown, click, scroll, touchstart) reset the
// idle timers. The server session extends on every authenticated API request,
// so client-side inactivity closely mirrors server-side inactivity.
//
// Usage:
//   const { isExpiringSoon, extendSession } = useSessionTimeout({
//     isActive: auth.status === 'authenticated',
//     onExpire: () => forceLogout(),
//   });

import { useCallback, useEffect, useRef, useState } from 'react';

const ACTIVITY_EVENTS = ['mousemove', 'keydown', 'click', 'scroll', 'touchstart'] as const;

export interface UseSessionTimeoutOptions {
  /** Whether to run timers. Pass true only when the user is authenticated. */
  isActive: boolean;
  /** Milliseconds of inactivity before the warning fires. Default: 25 minutes. */
  warnAfterMs?: number;
  /** Milliseconds of inactivity before onExpire fires. Default: 30 minutes. */
  expireAfterMs?: number;
  /** Called when the warn threshold is reached. Show a "session expiring" banner. */
  onWarn: () => void;
  /** Called when the expire threshold is reached. Trigger logout. */
  onExpire: () => void;
}

export interface UseSessionTimeoutResult {
  /** True while the warn timer has fired but expiry has not yet occurred. */
  isExpiringSoon: boolean;
  /** Resets both timers and clears the warning. Call when user extends the session. */
  extendSession: () => void;
}

export function useSessionTimeout({
  isActive,
  warnAfterMs = 25 * 60 * 1000,
  expireAfterMs = 30 * 60 * 1000,
  onWarn,
  onExpire,
}: UseSessionTimeoutOptions): UseSessionTimeoutResult {
  const [isExpiringSoon, setIsExpiringSoon] = useState(false);

  // Use refs for callbacks so timer closures always see the latest versions
  // without needing to re-register event listeners.
  const onWarnRef = useRef(onWarn);
  const onExpireRef = useRef(onExpire);
  useEffect(() => { onWarnRef.current = onWarn; });
  useEffect(() => { onExpireRef.current = onExpire; });

  const warnTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const expireTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const clearTimers = useCallback(() => {
    if (warnTimerRef.current !== null) {
      clearTimeout(warnTimerRef.current);
      warnTimerRef.current = null;
    }
    if (expireTimerRef.current !== null) {
      clearTimeout(expireTimerRef.current);
      expireTimerRef.current = null;
    }
  }, []);

  const resetTimers = useCallback(() => {
    clearTimers();
    setIsExpiringSoon(false);
    warnTimerRef.current = setTimeout(() => {
      setIsExpiringSoon(true);
      onWarnRef.current();
    }, warnAfterMs);
    expireTimerRef.current = setTimeout(() => {
      setIsExpiringSoon(false);
      onExpireRef.current();
    }, expireAfterMs);
  }, [clearTimers, warnAfterMs, expireAfterMs]);

  // extendSession is the public API: resets the timers and hides the warning.
  const extendSession = useCallback(() => {
    resetTimers();
  }, [resetTimers]);

  useEffect(() => {
    if (!isActive) {
      clearTimers();
      setIsExpiringSoon(false);
      return;
    }

    resetTimers();

    const handler = () => resetTimers();
    ACTIVITY_EVENTS.forEach((e) => document.addEventListener(e, handler, { passive: true }));

    return () => {
      ACTIVITY_EVENTS.forEach((e) => document.removeEventListener(e, handler));
      clearTimers();
    };
  }, [isActive, resetTimers, clearTimers]);

  return { isExpiringSoon, extendSession };
}
