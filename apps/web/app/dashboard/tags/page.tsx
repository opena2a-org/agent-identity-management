"use client";

import { useState, useEffect } from "react";
import { Plus, Search, Filter, Tag as TagIcon, X, Users } from "lucide-react";
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
import { api, Tag, TagCategory, Agent } from "@/lib/api";
import { toast } from "sonner";
import { TagList } from "@/components/tags/tag-list";
import { TagCreateModal } from "@/components/tags/tag-create-modal";
import { TagEditModal } from "@/components/tags/tag-edit-modal";
import { Badge } from "@/components/ui/badge";
import Link from "next/link";

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
  const [selectedTag, setSelectedTag] = useState<Tag | null>(null);
  const [tagAgents, setTagAgents] = useState<Agent[]>([]);
  const [isLoadingAgents, setIsLoadingAgents] = useState(false);

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

  const handleViewAgents = async (tag: Tag) => {
    setSelectedTag(tag);
    setIsLoadingAgents(true);
    try {
      const response = await api.getAgentsByTag(tag.id);
      setTagAgents(response.agents);
    } catch (error: any) {
      toast.error("Failed to load agents", {
        description: error.message || "Could not fetch agents for this tag",
      });
      setTagAgents([]);
    } finally {
      setIsLoadingAgents(false);
    }
  };

  const handleCloseAgentsView = () => {
    setSelectedTag(null);
    setTagAgents([]);
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
            onViewAgents={handleViewAgents}
          />
        </CardContent>
      </Card>

      {/* Agents with Selected Tag */}
      {selectedTag && (
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <div>
                <CardTitle className="flex items-center gap-2">
                  <Users className="h-5 w-5" />
                  Agents with Tag: {selectedTag.key}:{selectedTag.value}
                </CardTitle>
                <CardDescription>
                  Agents that have been tagged with this classification
                </CardDescription>
              </div>
              <Button variant="ghost" size="sm" onClick={handleCloseAgentsView}>
                <X className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            {isLoadingAgents ? (
              <div className="flex items-center justify-center py-12">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-gray-900 dark:border-gray-100"></div>
              </div>
            ) : !tagAgents || tagAgents.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-center">
                <Users className="h-12 w-12 text-gray-400 mb-4" />
                <p className="text-muted-foreground mb-2">No agents found</p>
                <p className="text-sm text-muted-foreground">
                  No agents have been tagged with "{selectedTag.key}:
                  {selectedTag.value}" yet
                </p>
              </div>
            ) : (
              <div className="space-y-4">
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                  {tagAgents.map((agent) => (
                    <Link
                      key={agent.id}
                      href={`/dashboard/agents/${agent.id}`}
                      className="block"
                    >
                      <Card className="hover:shadow-md transition-shadow cursor-pointer">
                        <CardHeader className="pb-3">
                          <div className="flex items-start justify-between">
                            <div className="flex-1">
                              <CardTitle className="text-base">
                                {agent.display_name || agent.name}
                              </CardTitle>
                              <p className="text-xs text-muted-foreground mt-1">
                                {agent.name}
                              </p>
                            </div>
                            <Badge
                              variant={
                                agent.status === "verified"
                                  ? "default"
                                  : agent.status === "pending"
                                    ? "secondary"
                                    : "destructive"
                              }
                            >
                              {agent.status}
                            </Badge>
                          </div>
                        </CardHeader>
                        <CardContent className="pb-3">
                          <p className="text-sm text-muted-foreground line-clamp-2">
                            {agent.description || "No description"}
                          </p>
                          <div className="flex items-center justify-between mt-3">
                            <span className="text-xs text-muted-foreground">
                              Trust Score
                            </span>
                            <span className="text-sm font-medium">
                              {Math.round(agent.trust_score * 100)}%
                            </span>
                          </div>
                        </CardContent>
                      </Card>
                    </Link>
                  ))}
                </div>
                <div className="text-sm text-muted-foreground text-center pt-4 border-t">
                  Showing {tagAgents?.length || 0} agent
                  {tagAgents?.length !== 1 ? "s" : ""} with this tag
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

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
