'use client'

import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'
import type { DetectionMethod } from '@/lib/api'
import { Brain, FileCode2, Package, Play, Terminal, Zap } from 'lucide-react'

interface DetectionMethodBadgeProps {
  method: DetectionMethod
  className?: string
  showIcon?: boolean
}

const detectionMethodConfig: Record<
  DetectionMethod,
  {
    label: string
    description: string
    icon: React.ElementType
    variant: 'default' | 'secondary' | 'success' | 'info' | 'warning'
    color: string
  }
> = {
  manual: {
    label: 'Manual',
    description: 'Manually registered by user',
    icon: Terminal,
    variant: 'secondary',
    color: 'bg-gray-500',
  },
  claude_config: {
    label: 'Claude Config',
    description: 'Detected from Claude Desktop configuration',
    icon: FileCode2,
    variant: 'info',
    color: 'bg-blue-500',
  },
  sdk_import: {
    label: 'SDK Import',
    description: 'Detected from SDK import analysis',
    icon: Package,
    variant: 'success',
    color: 'bg-green-500',
  },
  sdk_runtime: {
    label: 'SDK Runtime',
    description: 'Detected at runtime by SDK',
    icon: Play,
    variant: 'success',
    color: 'bg-emerald-500',
  },
  direct_api: {
    label: 'Direct API',
    description: 'Reported directly via API',
    icon: Brain,
    variant: 'default',
    color: 'bg-purple-500',
  },
  sdk_integration: {
    label: 'SDK Integration',
    description: 'SDK successfully integrated with agent',
    icon: Zap,
    variant: 'success',
    color: 'bg-green-600',
  },
}

export function DetectionMethodBadge({
  method,
  className,
  showIcon = true,
}: DetectionMethodBadgeProps) {
  const config = detectionMethodConfig[method]
  const Icon = config.icon

  return (
    <Badge
      variant={config.variant}
      className={cn('gap-1', className)}
      title={config.description}
    >
      {showIcon && <Icon className="h-3 w-3" />}
      <span>{config.label}</span>
    </Badge>
  )
}

// Multiple detection methods indicator
export function DetectionMethodsBadges({
  methods,
  className,
  maxDisplay = 3,
}: {
  methods: DetectionMethod[]
  className?: string
  maxDisplay?: number
}) {
  const displayMethods = methods.slice(0, maxDisplay)
  const remaining = methods.length - maxDisplay

  return (
    <div className={cn('flex flex-wrap items-center gap-1', className)}>
      {displayMethods.map((method) => (
        <DetectionMethodBadge key={method} method={method} showIcon={false} />
      ))}
      {remaining > 0 && (
        <Badge variant="outline" className="text-xs">
          +{remaining}
        </Badge>
      )}
    </div>
  )
}

// Confidence score indicator with color
export function ConfidenceBadge({
  score,
  className,
}: {
  score: number
  className?: string
}) {
  const getVariant = (score: number) => {
    if (score >= 90) return 'success'
    if (score >= 75) return 'info'
    if (score >= 60) return 'warning'
    return 'secondary'
  }

  const getLabel = (score: number) => {
    if (score >= 90) return 'High'
    if (score >= 75) return 'Good'
    if (score >= 60) return 'Medium'
    return 'Low'
  }

  return (
    <Badge variant={getVariant(score)} className={cn('gap-1', className)}>
      <span className="font-mono text-xs">{score.toFixed(0)}%</span>
      <span className="text-xs opacity-75">({getLabel(score)})</span>
    </Badge>
  )
}
