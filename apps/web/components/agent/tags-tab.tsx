"use client";

import { useState, useEffect } from "react";
import { Tag, api } from "@/lib/api";
import { toast } from "sonner";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { TagSelector } from "@/components/tags/tag-selector";

interface AgentTagsTabProps {
  agentId: string;
}

export function AgentTagsTab({ agentId }: AgentTagsTabProps) {
  const [allTags, setAllTags] = useState<Tag[]>([]);
  const [agentTags, setAgentTags] = useState<Tag[]>([]);
  const [suggestions, setSuggestions] = useState<Tag[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isUpdating, setIsUpdating] = useState(false);

  const loadData = async () => {
    try {
      setIsLoading(true);
      const [tags, currentTags, suggestedTags] = await Promise.all([
        api.listTags(),
        api.getAgentTags(agentId),
        api.suggestTagsForAgent(agentId),
      ]);
      setAllTags(tags);
      setAgentTags(currentTags);
      setSuggestions(suggestedTags);
    } catch (error: any) {
      toast.error("Failed to load tags", {
        description: error.message || "Could not fetch tag data",
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [agentId]);

  const handleAddTag = async (tagId: string) => {
    try {
      setIsUpdating(true);
      await api.addTagsToAgent(agentId, [tagId]);
      toast.success("Tag added successfully");
      await loadData();
    } catch (error: any) {
      toast.error("Failed to add tag", {
        description: error.message || "Could not add tag to agent",
      });
    } finally {
      setIsUpdating(false);
    }
  };

  const handleRemoveTag = async (tagId: string) => {
    try {
      setIsUpdating(true);
      await api.removeTagFromAgent(agentId, tagId);
      toast.success("Tag removed successfully");
      await loadData();
    } catch (error: any) {
      toast.error("Failed to remove tag", {
        description: error.message || "Could not remove tag from agent",
      });
    } finally {
      setIsUpdating(false);
    }
  };

  if (isLoading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Agent Tags</CardTitle>
          <CardDescription>Loading tags...</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-center py-8">
            <div className="text-muted-foreground">Loading...</div>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Agent Tags</CardTitle>
        <CardDescription>
          Organize this agent with tags for easier filtering and categorization
        </CardDescription>
      </CardHeader>
      <CardContent>
        <TagSelector
          availableTags={allTags}
          selectedTags={agentTags}
          suggestions={suggestions}
          onAdd={handleAddTag}
          onRemove={handleRemoveTag}
          isLoading={isUpdating}
        />

        {agentTags.length === 0 && !isUpdating && (
          <div className="mt-6 text-center py-8 border-2 border-dashed rounded-lg">
            <p className="text-muted-foreground mb-2">No tags assigned</p>
            <p className="text-sm text-muted-foreground">
              Add tags to organize and categorize this agent
            </p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
