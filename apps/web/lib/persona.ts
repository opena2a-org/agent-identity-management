"use client";

/**
 * Persona lens: a PRESENTATION preference, never an authorization boundary.
 *
 * The lens reorders and emphasizes what an already-authorized user sees first
 * (developer / security practitioner / executive). It must never be consulted
 * to decide whether a route, object or action is permitted: that stays with
 * `getDashboardPermissions()` in lib/permissions.ts and the route gates. A
 * surface that shows something in one lens that the role cannot reach in
 * another is a defect (see lib/persona.test.ts).
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type Persona = "developer" | "security" | "executive";

export const PERSONAS: ReadonlyArray<{ value: Persona; label: string; short: string; description: string }> = [
  { value: "developer", label: "Developer", short: "Dev", description: "Your agents, the quickstart and your keys first." },
  { value: "security", label: "Security", short: "Sec", description: "What needs attention, the verification stream and policy first." },
  { value: "executive", label: "Executive", short: "Exec", description: "Posture, coverage and one recommended action first." },
];

export function isPersona(value: unknown): value is Persona {
  return value === "developer" || value === "security" || value === "executive";
}

/** Maps the role collected at signup (`profile.role`) to the initial lens. */
export function personaFromSignupRole(role?: string | null): Persona {
  switch (role) {
    case "security-engineer":
      return "security";
    case "founder-or-exec":
      return "executive";
    case "developer":
    case "student-or-researcher":
    default:
      return "developer";
  }
}

type PersonaSource = "default" | "signup" | "user";

interface PersonaState {
  persona: Persona;
  source: PersonaSource;
  /** Explicit user choice (the switcher). Always wins and is remembered. */
  setPersona: (persona: Persona) => void;
  /** Seeds the lens from the signup role unless the user already picked one. */
  seedFromSignupRole: (role?: string | null) => void;
}

export const usePersona = create<PersonaState>()(
  persist(
    (set, get) => ({
      persona: "developer",
      source: "default",
      setPersona: (persona) => set({ persona, source: "user" }),
      seedFromSignupRole: (role) => {
        if (get().source === "user") return;
        set({ persona: personaFromSignupRole(role), source: "signup" });
      },
    }),
    {
      name: "aim.persona",
      version: 1,
      partialize: (state) => ({ persona: state.persona, source: state.source }),
    }
  )
);
