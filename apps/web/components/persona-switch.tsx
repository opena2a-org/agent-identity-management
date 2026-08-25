"use client";

import { PERSONAS, usePersona } from "@/lib/persona";
import { cn } from "@/lib/utils";

/**
 * One-interaction lens switch (Dev / Sec / Exec). Presentation only; see lib/persona.ts.
 */
export function PersonaSwitch({
  className,
  long = false,
  label = "Viewing as",
}: {
  className?: string;
  long?: boolean;
  label?: string | null;
}) {
  const persona = usePersona((s) => s.persona);
  const setPersona = usePersona((s) => s.setPersona);

  return (
    <div className={cn("flex flex-col gap-1.5", className)}>
      {label ? <span className="text-overline pl-1">{label}</span> : null}
      <div role="radiogroup" aria-label="Lens" className="glass-segment w-full">
        {PERSONAS.map((p) => {
          const active = persona === p.value;
          return (
            <button
              key={p.value}
              type="button"
              role="radio"
              aria-checked={active}
              data-active={active}
              title={p.description}
              onClick={() => setPersona(p.value)}
              className="glass-segment-item flex-1 text-center focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              {long ? p.label : p.short}
            </button>
          );
        })}
      </div>
    </div>
  );
}
