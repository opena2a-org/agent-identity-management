import { Tag } from "@/lib/api";
import { X } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

interface TagBadgeProps {
  tag: Tag;
  onRemove?: () => void;
  size?: "sm" | "md" | "lg";
}

export function TagBadge({ tag, onRemove, size = "md" }: TagBadgeProps) {
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
      case "custom":
        return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300";
      default:
        return "bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-300";
    }
  };

  const sizeClass = {
    sm: "text-xs px-2 py-0.5",
    md: "text-sm px-2.5 py-1",
    lg: "text-base px-3 py-1.5",
  }[size];

  const customStyle = tag.color
    ? { backgroundColor: `${tag.color}20`, color: tag.color, borderColor: tag.color }
    : {};

  return (
    <Badge
      variant="outline"
      className={`${tag.color ? "" : getCategoryColor(tag.category)} ${sizeClass} inline-flex items-center gap-1.5`}
      style={customStyle}
    >
      <span className="font-medium">{tag.key}:</span>
      <span>{tag.value}</span>
      {onRemove && (
        <Button
          variant="ghost"
          size="sm"
          className="h-auto p-0 ml-1 hover:bg-transparent"
          onClick={(e) => {
            e.stopPropagation();
            onRemove();
          }}
        >
          <X className="h-3 w-3" />
        </Button>
      )}
    </Badge>
  );
}
