import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

// Glasshouse status chips: tinted fill + tinted border + readable text, never white-on-color.
const badgeVariants = cva(
  "inline-flex items-center gap-1.5 rounded-pill border px-2.5 py-0.5 text-2xs font-bold transition-colors focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-2",
  {
    variants: {
      variant: {
        default: "border-transparent bg-brand-soft text-brand-text",
        secondary: "border-glass-inset-border bg-glass-inset-gray text-ink-body",
        destructive: "border-danger-border bg-danger-fill text-danger-text",
        outline: "border-stroke text-ink-body",
        success: "border-success-border bg-success-fill text-success-text",
        warning: "border-warning-border bg-warning-fill text-warning-text",
        info: "border-transparent bg-brand-soft text-brand-text",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof badgeVariants> {}

function Badge({ className, variant, ...props }: BadgeProps) {
  return (
    <div className={cn(badgeVariants({ variant }), className)} {...props} />
  )
}

export { Badge, badgeVariants }
