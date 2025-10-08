'use client'

import { X } from 'lucide-react'
import { Tag } from '@/lib/api'
import { cn } from '@/lib/utils'

interface TagChipProps {
  tag: Tag
  onRemove?: () => void
  size?: 'sm' | 'md' | 'lg'
  className?: string
}

export function TagChip({ tag, onRemove, size = 'md', className }: TagChipProps) {
  const sizeClasses = {
    sm: 'text-xs px-2 py-0.5',
    md: 'text-sm px-2.5 py-1',
    lg: 'text-base px-3 py-1.5',
  }

  const backgroundColor = tag.color || getCategoryColor(tag.category)

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded-full font-medium transition-colors',
        sizeClasses[size],
        className
      )}
      style={{
        backgroundColor: `${backgroundColor}20`,
        color: backgroundColor,
        borderWidth: '1px',
        borderColor: `${backgroundColor}40`,
      }}
    >
      <span className="truncate max-w-[150px]" title={`${tag.key}: ${tag.value}`}>
        {tag.key}: {tag.value}
      </span>
      {onRemove && (
        <button
          onClick={(e) => {
            e.stopPropagation()
            onRemove()
          }}
          className="hover:opacity-70 transition-opacity"
          aria-label={`Remove tag ${tag.key}: ${tag.value}`}
        >
          <X className="h-3 w-3" />
        </button>
      )}
    </span>
  )
}

function getCategoryColor(category: string): string {
  const colors: Record<string, string> = {
    resource_type: '#3B82F6',       // Blue
    environment: '#10B981',          // Green
    agent_type: '#8B5CF6',           // Purple
    data_classification: '#F59E0B', // Amber
    custom: '#6B7280',               // Gray
  }
  return colors[category] || colors.custom
}
