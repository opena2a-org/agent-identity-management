import { Tag } from "@/lib/api";
import { Edit, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { TagBadge } from "./tag-badge";

interface TagListProps {
  tags: Tag[];
  isLoading: boolean;
  onEdit: (tag: Tag) => void;
  onDelete: (tagId: string) => void;
}

export function TagList({ tags, isLoading, onEdit, onDelete }: TagListProps) {
  if (isLoading) {
    return (
      <div className="animate-pulse">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Tag</TableHead>
              <TableHead>Category</TableHead>
              <TableHead>Description</TableHead>
              <TableHead className="text-right">Actions</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {[1, 2, 3, 4, 5].map((i) => (
              <TableRow key={i}>
                <TableCell>
                  <div className="h-6 w-32 bg-gray-200 dark:bg-gray-700 rounded"></div>
                </TableCell>
                <TableCell>
                  <div className="h-4 w-24 bg-gray-200 dark:bg-gray-700 rounded"></div>
                </TableCell>
                <TableCell>
                  <div className="h-4 w-48 bg-gray-200 dark:bg-gray-700 rounded"></div>
                </TableCell>
                <TableCell className="text-right">
                  <div className="flex justify-end gap-2">
                    <div className="h-8 w-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
                    <div className="h-8 w-8 bg-gray-200 dark:bg-gray-700 rounded"></div>
                  </div>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    );
  }

  if (tags.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center py-12 text-center">
        <p className="text-muted-foreground mb-2">No tags found</p>
        <p className="text-sm text-muted-foreground">
          Create a tag to start organizing your agents and MCP servers
        </p>
      </div>
    );
  }

  const getCategoryLabel = (category: string) => {
    switch (category) {
      case "resource_type":
        return "Resource Type";
      case "environment":
        return "Environment";
      case "agent_type":
        return "Agent Type";
      case "data_classification":
        return "Data Classification";
      case "custom":
        return "Custom";
      default:
        return category;
    }
  };

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Tag</TableHead>
          <TableHead>Category</TableHead>
          <TableHead>Description</TableHead>
          <TableHead className="text-right">Actions</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {tags.map((tag) => (
          <TableRow key={tag.id}>
            <TableCell>
              <TagBadge tag={tag} />
            </TableCell>
            <TableCell>{getCategoryLabel(tag.category)}</TableCell>
            <TableCell>
              <span className="text-sm text-muted-foreground">
                {tag.description || "No description"}
              </span>
            </TableCell>
            <TableCell className="text-right">
              <div className="flex justify-end gap-2">
                <Button variant="ghost" size="sm" onClick={() => onEdit(tag)}>
                  <Edit className="h-4 w-4" />
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger asChild>
                    <Button variant="ghost" size="sm">
                      <Trash2 className="h-4 w-4 text-red-500" />
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>Delete Tag</AlertDialogTitle>
                      <AlertDialogDescription>
                        Are you sure you want to delete the tag "{tag.key}:
                        {tag.value}"? This action cannot be undone and will
                        remove the tag from all agents and MCP servers.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() => onDelete(tag.id)}
                        className="bg-red-500 hover:bg-red-600"
                      >
                        Delete
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
