'use client'

import { useState, useEffect } from 'react'
import { Plus, Lightbulb } from 'lucide-react'
import { Tag, TagCategory } from '@/lib/api'
import { TagChip } from './tag-chip'
import { Button } from './button'
import { cn } from '@/lib/utils'

interface TagSelectorProps {
  selectedTags: Tag[]
  availableTags: Tag[]
  suggestedTags?: Tag[]
  maxTags?: number
  onTagsChange: (tags: Tag[]) => void
  onCreateTag?: (tag: { key: string; value: string; category: TagCategory; description?: string; color?: string }) => Promise<void>
  className?: string
}

export function TagSelector({
  selectedTags,
  availableTags,
  suggestedTags = [],
  maxTags,
  onTagsChange,
  onCreateTag,
  className,
}: TagSelectorProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [searchQuery, setSearchQuery] = useState('')
  const [showSuggestions, setShowSuggestions] = useState(true)

  const filteredTags = availableTags.filter(
    (tag) =>
      !selectedTags.some((t) => t.id === tag.id) &&
      (searchQuery === '' ||
        tag.key.toLowerCase().includes(searchQuery.toLowerCase()) ||
        tag.value.toLowerCase().includes(searchQuery.toLowerCase()))
  )

  const availableSuggestions = suggestedTags.filter(
    (tag) => !selectedTags.some((t) => t.id === tag.id)
  )

  const canAddMore = maxTags ? selectedTags.length < maxTags : true

  const handleAddTag = (tag: Tag) => {
    if (canAddMore) {
      onTagsChange([...selectedTags, tag])
      setSearchQuery('')
    }
  }

  const handleRemoveTag = (tagId: string) => {
    onTagsChange(selectedTags.filter((t) => t.id !== tagId))
  }

  return (
    <div className={cn('space-y-3', className)}>
      {/* Selected Tags */}
      <div className="flex flex-wrap gap-2">
        {selectedTags.map((tag) => (
          <TagChip
            key={tag.id}
            tag={tag}
            onRemove={() => handleRemoveTag(tag.id)}
          />
        ))}
        {canAddMore && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => setIsOpen(!isOpen)}
            className="h-7"
          >
            <Plus className="h-3 w-3 mr-1" />
            Add Tag
          </Button>
        )}
      </div>

      {/* Tag Selector Dropdown */}
      {isOpen && canAddMore && (
        <div className="border rounded-lg p-4 space-y-3 bg-card">
          {/* Search */}
          <input
            type="text"
            placeholder="Search tags..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full px-3 py-2 border rounded-md text-sm focus:outline-none focus:ring-2 focus:ring-primary"
            autoFocus
          />

          {/* Smart Suggestions */}
          {showSuggestions && availableSuggestions.length > 0 && (
            <div className="space-y-2">
              <div className="flex items-center gap-2 text-sm font-medium text-muted-foreground">
                <Lightbulb className="h-4 w-4 text-yellow-500" />
                <span>Suggested Tags</span>
                <button
                  onClick={() => setShowSuggestions(false)}
                  className="ml-auto text-xs hover:underline"
                >
                  Hide
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                {availableSuggestions.map((tag) => (
                  <button
                    key={tag.id}
                    onClick={() => handleAddTag(tag)}
                    className="group"
                  >
                    <TagChip tag={tag} size="sm" className="cursor-pointer group-hover:opacity-70 transition-opacity" />
                  </button>
                ))}
              </div>
            </div>
          )}

          {/* Available Tags */}
          {filteredTags.length > 0 ? (
            <div className="space-y-2">
              <div className="text-sm font-medium text-muted-foreground">
                All Tags
              </div>
              <div className="max-h-48 overflow-y-auto space-y-1">
                {filteredTags.map((tag) => (
                  <button
                    key={tag.id}
                    onClick={() => handleAddTag(tag)}
                    className="w-full flex items-center gap-2 p-2 rounded hover:bg-accent transition-colors text-left group"
                  >
                    <TagChip tag={tag} size="sm" className="group-hover:opacity-70 transition-opacity" />
                    {tag.description && (
                      <span className="text-xs text-muted-foreground truncate">
                        {tag.description}
                      </span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className="text-sm text-muted-foreground text-center py-4">
              {searchQuery
                ? 'No tags found'
                : 'All available tags are already selected'}
            </div>
          )}

          {/* Create New Tag (if callback provided) */}
          {onCreateTag && searchQuery && filteredTags.length === 0 && (
            <div className="pt-2 border-t">
              <Button
                variant="ghost"
                size="sm"
                onClick={async () => {
                  const [key, value] = searchQuery.includes(':')
                    ? searchQuery.split(':', 2)
                    : [searchQuery, searchQuery]

                  await onCreateTag({
                    key: key.trim(),
                    value: value.trim(),
                    category: 'custom',
                    description: `Custom tag: ${searchQuery}`,
                  })
                  setSearchQuery('')
                }}
                className="w-full"
              >
                <Plus className="h-4 w-4 mr-2" />
                Create Tag "{searchQuery}"
              </Button>
            </div>
          )}

          {/* Close Button */}
          <div className="flex justify-end">
            <Button
              variant="outline"
              size="sm"
              onClick={() => setIsOpen(false)}
            >
              Done
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
