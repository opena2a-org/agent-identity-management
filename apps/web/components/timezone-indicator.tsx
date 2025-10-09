'use client';

import { Clock } from 'lucide-react';
import { useEffect, useState } from 'react';
import { getLocalTimezone, getTimezoneOffset } from '@/lib/date-utils';

/**
 * Displays the user's local timezone for transparency
 * Helps users understand what timezone timestamps are displayed in
 */
export function TimezoneIndicator() {
  const [timezone, setTimezone] = useState<string>('');
  const [offset, setOffset] = useState<string>('');
  const [currentTime, setCurrentTime] = useState<string>('');

  useEffect(() => {
    // Set timezone info on mount (client-side only)
    setTimezone(getLocalTimezone());
    setOffset(getTimezoneOffset());

    // Update current time every second
    const updateTime = () => {
      const now = new Date();
      setCurrentTime(now.toLocaleTimeString(undefined, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hour12: true
      }));
    };

    updateTime();
    const interval = setInterval(updateTime, 1000);

    return () => clearInterval(interval);
  }, []);

  if (!timezone) return null; // Don't render during SSR

  return (
    <div className="flex items-center gap-2 text-sm text-gray-600 dark:text-gray-400">
      <Clock className="h-4 w-4" />
      <span className="font-mono">{currentTime}</span>
      <span className="text-gray-400 dark:text-gray-600">|</span>
      <span>{timezone}</span>
      <span className="text-gray-400 dark:text-gray-600">({offset})</span>
    </div>
  );
}
