'use client';

import posthog from 'posthog-js';
import { PostHogProvider as PHProvider } from 'posthog-js/react';
import { useEffect } from 'react';

export function PostHogProvider({ children }: { children: React.ReactNode }) {
  const posthogHost = process.env.NEXT_PUBLIC_POSTHOG_HOST;
  const posthogPublicKey = process.env.NEXT_PUBLIC_POSTHOG_KEY;
  if (!posthogHost || !posthogPublicKey) {
    throw new Error('Posthog not configured');
  }
  useEffect(() => {
    posthog.init(posthogPublicKey), {
      api_host: posthogHost,
      persistence: 'memory',
      disable_persistence: true,
      autocapture: false,
      capture_pageview: true,
      disable_session_recording: true,
      person_profiles: 'never',
      anonymize_ip: true,
    }
  }, []);

  return (
    <PHProvider client={posthog}>
      {children}
    </PHProvider>
  );
}
