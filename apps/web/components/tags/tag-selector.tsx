"use client";

import { useState, useEffect } from "react";
import { Tag } from "@/lib/api";
import { Plus, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { ScrollArea } from "@/components/ui/scroll-area";

interface TagSelectorProps {
  availableTags: Tag[];
  selectedTags: Tag[];
  suggestions?: Tag[];
  onAdd: (tagId: string) => void;
  onRemove: (tagId: string) => void;
  isLoading?: boolean;
}

export function TagSelector({
  availableTags,
  selectedTags,
  suggestions = [],
  onAdd,
  onRemove,
  isLoading = false,
}: TagSelectorProps) {
  const [searchQuery, setSearchQuery] = useState("");
  const [filteredTags, setFilteredTags] = useState<Tag[]>([]);
  const [isOpen, setIsOpen] = useState(false);

  useEffect(() => {
    // Filter out already selected tags
    const selectedIds = new Set(selectedTags.map((t) => t.id));
    let filtered = availableTags.filter((t) => !selectedIds.has(t.id));

    // Apply search filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (tag) =>
          tag.key.toLowerCase().includes(query) ||
          tag.value.toLowerCase().includes(query)
      );
    }

    setFilteredTags(filtered);
  }, [availableTags, selectedTags, searchQuery]);

  const getCategoryColor = (category: string) => {
    switch (category) {
      case "resource_type":
        return "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-300";
      case "environment":
        return "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300";
      case "agent_type":
        return "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-300";
      case "data_classification":
        return "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-300";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300";
    }
  };

  const handleAddTag = (tagId: string) => {
    onAdd(tagId);
    setSearchQuery("");
  };

  // Get suggestions that aren't already selected
  const selectedIds = new Set(selectedTags.map((t) => t.id));
  const availableSuggestions = suggestions.filter((s) => !selectedIds.has(s.id));

  return (
    <div className="space-y-3">
      {/* Selected Tags */}
      {selectedTags.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {selectedTags.map((tag) => {
            const customStyle = tag.color
              ? { backgroundColor: `${tag.color}20`, color: tag.color, borderColor: tag.color }
              : {};

            return (
              <Badge
                key={tag.id}
                variant="outline"
                className={`${
                  tag.color ? "" : getCategoryColor(tag.category)
                } inline-flex items-center gap-1.5`}
                style={customStyle}
              >
                <span className="font-medium">{tag.key}:</span>
                <span>{tag.value}</span>
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-auto p-0 ml-1 hover:bg-transparent"
                  onClick={() => onRemove(tag.id)}
                  disabled={isLoading}
                >
                  <X className="h-3 w-3" />
                </Button>
              </Badge>
            );
          })}
        </div>
      )}

      {/* Suggestions */}
      {availableSuggestions.length > 0 && (
        <div className="space-y-2">
          <p className="text-sm text-muted-foreground">Suggested tags:</p>
          <div className="flex flex-wrap gap-2">
            {availableSuggestions.map((tag) => {
              const customStyle = tag.color
                ? { backgroundColor: `${tag.color}20`, color: tag.color, borderColor: tag.color }
                : {};

              return (
                <Badge
                  key={tag.id}
                  variant="outline"
                  className={`${
                    tag.color ? "" : getCategoryColor(tag.category)
                  } inline-flex items-center gap-1.5 cursor-pointer hover:opacity-80`}
                  style={customStyle}
                  onClick={() => handleAddTag(tag.id)}
                >
                  <span className="font-medium">{tag.key}:</span>
                  <span>{tag.value}</span>
                  <Plus className="h-3 w-3 ml-1" />
                </Badge>
              );
            })}
          </div>
        </div>
      )}

      {/* Add Tag Button */}
      <Popover open={isOpen} onOpenChange={setIsOpen}>
        <PopoverTrigger asChild>
          <Button variant="outline" size="sm" disabled={isLoading}>
            <Plus className="mr-2 h-4 w-4" />
            Add Tag
          </Button>
        </PopoverTrigger>
        <PopoverContent className="w-80" align="start">
          <div className="space-y-3">
            <div>
              <h4 className="font-medium text-sm mb-2">Add Tags</h4>
              <Input
                placeholder="Search tags..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="h-8"
              />
            </div>

            <ScrollArea className="h-[200px]">
              <div className="space-y-1">
                {filteredTags.length === 0 ? (
                  <p className="text-sm text-muted-foreground text-center py-4">
                    No tags available
                  </p>
                ) : (
                  filteredTags.map((tag) => {
                    const customStyle = tag.color
                      ? { backgroundColor: `${tag.color}20`, color: tag.color, borderColor: tag.color }
                      : {};

                    return (
                      <button
                        key={tag.id}
                        onClick={() => {
                          handleAddTag(tag.id);
                          setIsOpen(false);
                        }}
                        className="w-full text-left px-2 py-1.5 rounded hover:bg-muted transition-colors"
                      >
                        <Badge
                          variant="outline"
                          className={`${
                            tag.color ? "" : getCategoryColor(tag.category)
                          } inline-flex items-center gap-1.5`}
                          style={customStyle}
                        >
                          <span className="font-medium">{tag.key}:</span>
                          <span>{tag.value}</span>
                        </Badge>
                        {tag.description && (
                          <p className="text-xs text-muted-foreground mt-1">
                            {tag.description}
                          </p>
                        )}
                      </button>
                    );
                  })
                )}
              </div>
            </ScrollArea>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
