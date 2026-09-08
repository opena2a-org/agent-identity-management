"use client";

import { ThemeProvider as NextThemesProvider } from "next-themes";
import type { ComponentProps } from "react";

/**
 * Light/dark theme provider. Adds the `dark` class to <html> (Tailwind darkMode: 'class')
 * and persists the user's pick in localStorage under the key `theme`. Mount once in
 * app/layout.tsx.
 *
 * The default stays "light" and the OS preference is not followed until every core surface
 * has been moved onto the design tokens: pages still carrying raw palette classes render as
 * light islands inside a dark shell. Flip `defaultTheme` to "system" and set `enableSystem`
 * in the change that completes that sweep.
 */
export function ThemeProvider({ children, ...props }: ComponentProps<typeof NextThemesProvider>) {
  return (
    <NextThemesProvider
      attribute="class"
      defaultTheme="light"
      enableSystem={false}
      disableTransitionOnChange
      {...props}
    >
      {children}
    </NextThemesProvider>
  );
}
