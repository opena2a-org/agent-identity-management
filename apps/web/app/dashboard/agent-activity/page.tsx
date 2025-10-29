"use client";

import { useState, useEffect } from "react";
import { useRouter } from "next/navigation";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { ScrollArea } from "@/components/ui/scroll-area";
import { api } from "@/lib/api";
import { AuthGuard } from "@/components/auth-guard";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Activity,
  MessageCircle,
  Bot,
  Clock,
  User,
  AlertCircle,
  TrendingUp,
  Calendar,
  Hash,
} from "lucide-react";

interface Agent {
  id: string;
  name: string;
  display_name: string;
  status: string;
  trust_score: number;
}

interface AgentActivity {
  id: string;
  agent_id: string;
  user_id: string;
  conversation_id: string;
  activity_type: string;
  activity_data: Record<string, any>;
  created_at: string;
  ip_address?: string;
  user_agent?: string;
}

interface AgentStats {
  total_messages: number;
  messages_today: number;
  total_conversations: number;
  active_conversations: number;
  avg_response_time?: number;
}

function AgentActivityContent() {
  const router = useRouter();
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);
  const [activities, setActivities] = useState<AgentActivity[]>([]);
  const [stats, setStats] = useState<AgentStats | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isLoadingActivities, setIsLoadingActivities] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load agents on mount
  useEffect(() => {
    loadAgents();
  }, []);

  // Load activities when agent is selected
  useEffect(() => {
    if (selectedAgent) {
      loadAgentActivities(selectedAgent);
    }
  }, [selectedAgent]);

  const loadAgents = async () => {
    try {
      setIsLoading(true);
      setError(null);
      const response = await api.listAgents();
      setAgents(response.agents || []);

      // Auto-select first agent if available
      if (response.agents && response.agents.length > 0) {
        setSelectedAgent(response.agents[0].id);
      }
    } catch (err: any) {
      setError("Failed to load agents. Please try again.");
      console.error("Error loading agents:", err);
    } finally {
      setIsLoading(false);
    }
  };

  const loadAgentActivities = async (agentId: string) => {
    try {
      setIsLoadingActivities(true);
      setError(null);

      // Load activities
      const activitiesResponse = await api.getChatAgentActivity(agentId);
      setActivities(activitiesResponse.activities || []);

      // Calculate stats from activities
      calculateStats(activitiesResponse.activities || []);
    } catch (err: any) {
      setError("Failed to load agent activities. Please try again.");
      console.error("Error loading activities:", err);
    } finally {
      setIsLoadingActivities(false);
    }
  };

  const calculateStats = (activities: AgentActivity[]) => {
    const today = new Date();
    today.setHours(0, 0, 0, 0);

    const messageSentActivities = activities.filter(
      (a) =>
        a.activity_type === "message_sent" ||
        a.activity_type === "agent_response_generated"
    );

    const todayMessages = messageSentActivities.filter((a) => {
      const activityDate = new Date(a.created_at);
      activityDate.setHours(0, 0, 0, 0);
      return activityDate.getTime() === today.getTime();
    });

    const conversationIds = new Set(
      activities.map((a) => a.conversation_id).filter(Boolean)
    );

    setStats({
      total_messages: messageSentActivities.length,
      messages_today: todayMessages.length,
      total_conversations: conversationIds.size,
      active_conversations: conversationIds.size, // Simplified for now
    });
  };

  const formatTime = (timestamp: string) => {
    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return "Just now";
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;
    return date.toLocaleDateString();
  };

  const getActivityIcon = (activityType: string) => {
    switch (activityType) {
      case "message_sent":
        return <MessageCircle className="h-4 w-4 text-blue-500" />;
      case "agent_response_generated":
        return <Bot className="h-4 w-4 text-green-500" />;
      case "conversation_started":
        return <MessageCircle className="h-4 w-4 text-purple-500" />;
      case "conversation_ended":
        return <MessageCircle className="h-4 w-4 text-gray-500" />;
      default:
        return <Activity className="h-4 w-4 text-gray-500" />;
    }
  };

  const getActivityLabel = (activityType: string) => {
    switch (activityType) {
      case "message_sent":
        return "Message Sent";
      case "agent_response_generated":
        return "Agent Response";
      case "conversation_started":
        return "Conversation Started";
      case "conversation_ended":
        return "Conversation Ended";
      case "limit_exceeded":
        return "Limit Exceeded";
      default:
        return activityType
          .replace(/_/g, " ")
          .replace(/\b\w/g, (l) => l.toUpperCase());
    }
  };

  const selectedAgentData = agents.find((a) => a.id === selectedAgent);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Agent Activity</h1>
        <p className="text-muted-foreground mt-2">
          Monitor and analyze agent chat activities and interactions
        </p>
      </div>

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Agent Selector */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center space-x-2">
            <Bot className="h-5 w-5" />
            <span>Select Agent</span>
          </CardTitle>
          <CardDescription>
            Choose an agent to view its activity history
          </CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-2">
              <Skeleton className="h-12 w-full" />
            </div>
          ) : agents.length === 0 ? (
            <div className="text-center py-8 text-muted-foreground">
              <Bot className="h-12 w-12 mx-auto mb-4 opacity-50" />
              <p className="font-medium">No agents found</p>
              <p className="text-sm mt-2">
                Create an agent to start tracking activity
              </p>
            </div>
          ) : (
            <select
              value={selectedAgent || ""}
              onChange={(e) => setSelectedAgent(e.target.value)}
              className="w-full p-3 border border-gray-300 dark:border-gray-600 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-800"
            >
              {agents.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {agent.display_name} ({agent.name}) - Trust Score:{" "}
                  {agent.trust_score}
                </option>
              ))}
            </select>
          )}
        </CardContent>
      </Card>

      {selectedAgent && selectedAgentData && (
        <>
          {/* Statistics Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center space-x-2">
                  <Hash className="h-4 w-4" />
                  <span>Total Messages</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                {isLoadingActivities ? (
                  <Skeleton className="h-8 w-20" />
                ) : (
                  <div className="text-2xl font-bold">
                    {stats?.total_messages || 0}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center space-x-2">
                  <Calendar className="h-4 w-4" />
                  <span>Messages Today</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                {isLoadingActivities ? (
                  <Skeleton className="h-8 w-20" />
                ) : (
                  <div className="text-2xl font-bold text-blue-600">
                    {stats?.messages_today || 0}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center space-x-2">
                  <MessageCircle className="h-4 w-4" />
                  <span>Total Conversations</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                {isLoadingActivities ? (
                  <Skeleton className="h-8 w-20" />
                ) : (
                  <div className="text-2xl font-bold">
                    {stats?.total_conversations || 0}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader className="pb-2">
                <CardTitle className="text-sm font-medium text-muted-foreground flex items-center space-x-2">
                  <TrendingUp className="h-4 w-4" />
                  <span>Trust Score</span>
                </CardTitle>
              </CardHeader>
              <CardContent>
                <div className="text-2xl font-bold text-green-600">
                  {selectedAgentData.trust_score}
                </div>
              </CardContent>
            </Card>
          </div>

          {/* Activity Timeline */}
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <Activity className="h-5 w-5" />
                <span>Activity Timeline</span>
              </CardTitle>
              <CardDescription>
                Recent activities for {selectedAgentData.display_name}
              </CardDescription>
            </CardHeader>
            <CardContent>
              <ScrollArea className="h-[500px] pr-4">
                {isLoadingActivities ? (
                  <div className="space-y-4">
                    {[...Array(5)].map((_, i) => (
                      <div key={i} className="flex items-start space-x-4">
                        <Skeleton className="h-10 w-10 rounded-full" />
                        <div className="flex-1 space-y-2">
                          <Skeleton className="h-4 w-3/4" />
                          <Skeleton className="h-3 w-1/2" />
                        </div>
                      </div>
                    ))}
                  </div>
                ) : activities.length === 0 ? (
                  <div className="text-center py-12 text-muted-foreground">
                    <Activity className="h-12 w-12 mx-auto mb-4 opacity-50" />
                    <p className="font-medium">No activity yet</p>
                    <p className="text-sm mt-2">
                      This agent hasn't had any interactions yet
                    </p>
                  </div>
                ) : (
                  <div className="space-y-4">
                    {activities.map((activity) => (
                      <div
                        key={activity.id}
                        className="flex items-start space-x-4 p-4 rounded-lg border border-gray-200 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors"
                      >
                        <div className="flex-shrink-0 mt-1">
                          <div className="h-10 w-10 rounded-full bg-gray-100 dark:bg-gray-800 flex items-center justify-center">
                            {getActivityIcon(activity.activity_type)}
                          </div>
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center justify-between">
                            <p className="font-medium text-sm">
                              {getActivityLabel(activity.activity_type)}
                            </p>
                            <div className="flex items-center space-x-1 text-xs text-muted-foreground">
                              <Clock className="h-3 w-3" />
                              <span>{formatTime(activity.created_at)}</span>
                            </div>
                          </div>

                          {/* Activity Details */}
                          {activity.activity_data &&
                            Object.keys(activity.activity_data).length > 0 && (
                              <div className="mt-2 text-sm text-muted-foreground">
                                {activity.activity_data.message_id && (
                                  <p className="truncate">
                                    Message ID:{" "}
                                    {activity.activity_data.message_id}
                                  </p>
                                )}
                                {activity.activity_data.content_length && (
                                  <p>
                                    Content Length:{" "}
                                    {activity.activity_data.content_length}{" "}
                                    chars
                                  </p>
                                )}
                                {activity.activity_data.response_type && (
                                  <p>
                                    Response Type:{" "}
                                    {activity.activity_data.response_type}
                                  </p>
                                )}
                                {activity.activity_data.model && (
                                  <p>Model: {activity.activity_data.model}</p>
                                )}
                              </div>
                            )}

                          {/* Conversation Link */}
                          {activity.conversation_id && (
                            <button
                              onClick={() =>
                                router.push(
                                  `/dashboard/chat?conversation=${activity.conversation_id}`
                                )
                              }
                              className="mt-2 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                            >
                              View Conversation →
                            </button>
                          )}

                          {/* Timestamp */}
                          <p className="text-xs text-muted-foreground mt-1">
                            {new Date(activity.created_at).toLocaleString()}
                          </p>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </ScrollArea>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  );
}

export default function AgentActivityPage() {
  return (
    <AuthGuard>
      <AgentActivityContent />
    </AuthGuard>
  );
}
