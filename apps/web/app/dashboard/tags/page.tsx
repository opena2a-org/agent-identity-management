"use client";

import { useState, useEffect } from "react";
import { Plus, Search, Filter, Tag as TagIcon } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { api, Tag, TagCategory } from "@/lib/api";
import { toast } from "sonner";
import { TagList } from "@/components/tags/tag-list";
import { TagCreateModal } from "@/components/tags/tag-create-modal";
import { TagEditModal } from "@/components/tags/tag-edit-modal";

export default function TagsPage() {
  const [tags, setTags] = useState<Tag[]>([]);
  const [filteredTags, setFilteredTags] = useState<Tag[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [categoryFilter, setCategoryFilter] = useState<TagCategory | "all">(
    "all"
  );
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [editingTag, setEditingTag] = useState<Tag | null>(null);

  // Load tags
  const loadTags = async () => {
    try {
      setIsLoading(true);
      const loadedTags = await api.listTags();
      setTags(loadedTags);
      setFilteredTags(loadedTags);
    } catch (error: any) {
      toast.error("Failed to load tags", {
        description: error.message || "Could not fetch tags from server",
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadTags();
  }, []);

  // Filter tags based on search and category
  useEffect(() => {
    let filtered = tags;

    // Filter by category
    if (categoryFilter !== "all") {
      filtered = filtered.filter((tag) => tag.category === categoryFilter);
    }

    // Filter by search query
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      filtered = filtered.filter(
        (tag) =>
          tag.key.toLowerCase().includes(query) ||
          tag.value.toLowerCase().includes(query) ||
          tag.description?.toLowerCase().includes(query)
      );
    }

    setFilteredTags(filtered);
  }, [tags, searchQuery, categoryFilter]);

  const handleCreateTag = async (tagData: any) => {
    try {
      await api.createTag(tagData);
      toast.success("Tag created successfully");
      setIsCreateModalOpen(false);
      loadTags();
    } catch (error: any) {
      toast.error("Failed to create tag", {
        description: error.message || "Could not create tag",
      });
    }
  };

  const handleDeleteTag = async (tagId: string) => {
    try {
      await api.deleteTag(tagId);
      toast.success("Tag deleted successfully");
      loadTags();
    } catch (error: any) {
      toast.error("Failed to delete tag", {
        description: error.message || "Could not delete tag",
      });
    }
  };

  const handleEditTag = (tag: Tag) => {
    setEditingTag(tag);
  };

  const handleUpdateTag = async (tagId: string, tagData: any) => {
    try {
      await api.updateTag(tagId, tagData);
      toast.success("Tag updated successfully");
      setEditingTag(null);
      loadTags();
    } catch (error: any) {
      toast.error("Failed to update tag", {
        description: error.message || "Could not update tag",
      });
    }
  };

  const categoryOptions: { value: TagCategory | "all"; label: string }[] = [
    { value: "all", label: "All Categories" },
    { value: "resource_type", label: "Resource Type" },
    { value: "environment", label: "Environment" },
    { value: "agent_type", label: "Agent Type" },
    { value: "data_classification", label: "Data Classification" },
    { value: "custom", label: "Custom" },
  ];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Tags Management
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
            Organize agents and MCP servers with tags
          </p>
        </div>
        <Button onClick={() => setIsCreateModalOpen(true)}>
          <Plus className="mr-2 h-4 w-4" />
          Create Tag
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-900 dark:text-gray-100">
              Total Tags
            </CardTitle>
            <TagIcon className="h-4 w-4 text-gray-400" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="animate-pulse">
                <div className="h-8 w-16 bg-gray-200 dark:bg-gray-700 rounded mb-2"></div>
                <div className="h-3 w-32 bg-gray-200 dark:bg-gray-700 rounded"></div>
              </div>
            ) : (
              <>
                <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">
                  {tags.length}
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Across all categories
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-900 dark:text-gray-100">
              Categories
            </CardTitle>
            <Filter className="h-4 w-4 text-gray-400" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="animate-pulse">
                <div className="h-8 w-16 bg-gray-200 dark:bg-gray-700 rounded mb-2"></div>
                <div className="h-3 w-32 bg-gray-200 dark:bg-gray-700 rounded"></div>
              </div>
            ) : (
              <>
                <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">
                  {new Set(tags.map((t) => t.category)).size}
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Active tag categories
                </p>
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium text-gray-900 dark:text-gray-100">
              Filtered Results
            </CardTitle>
            <Search className="h-4 w-4 text-gray-400" />
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="animate-pulse">
                <div className="h-8 w-16 bg-gray-200 dark:bg-gray-700 rounded mb-2"></div>
                <div className="h-3 w-32 bg-gray-200 dark:bg-gray-700 rounded"></div>
              </div>
            ) : (
              <>
                <div className="text-2xl font-bold text-gray-900 dark:text-gray-100">
                  {filteredTags.length}
                </div>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Matching current filters
                </p>
              </>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle className="text-lg font-medium text-gray-900 dark:text-gray-100">
            Filter Tags
          </CardTitle>
          <CardDescription className="text-sm text-gray-500 dark:text-gray-400">
            Search and filter tags by category
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex gap-4">
            <div className="flex-1">
              <div className="relative">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-muted-foreground" />
                <Input
                  type="search"
                  placeholder="Search tags by key, value, or description..."
                  className="pl-8"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>
            <Select
              value={categoryFilter}
              onValueChange={(value) =>
                setCategoryFilter(value as TagCategory | "all")
              }
            >
              <SelectTrigger className="w-[200px]">
                <SelectValue placeholder="Select category" />
              </SelectTrigger>
              <SelectContent>
                {categoryOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>
                    {option.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </CardContent>
      </Card>

      {/* Tags List */}
      <Card>
        <CardHeader>
          <CardTitle>Tags ({filteredTags.length})</CardTitle>
          <CardDescription>Manage and organize your tags</CardDescription>
        </CardHeader>
        <CardContent>
          <TagList
            tags={filteredTags}
            isLoading={isLoading}
            onEdit={handleEditTag}
            onDelete={handleDeleteTag}
          />
        </CardContent>
      </Card>

      {/* Modals */}
      <TagCreateModal
        isOpen={isCreateModalOpen}
        onClose={() => setIsCreateModalOpen(false)}
        onSubmit={handleCreateTag}
      />

      {editingTag && (
        <TagEditModal
          tag={editingTag}
          isOpen={!!editingTag}
          onClose={() => setEditingTag(null)}
          onSubmit={(data) => handleUpdateTag(editingTag.id, data)}
        />
      )}
    </div>
  );
}
