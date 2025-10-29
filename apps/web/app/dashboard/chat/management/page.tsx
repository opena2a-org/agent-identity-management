"use client";

import { useState, useEffect } from "react";
import {
  MessageCircle,
  Users,
  Bot,
  Activity,
  AlertTriangle,
  CheckCircle,
  Clock,
  BarChart3,
  Settings,
  RefreshCw,
  Filter,
  Search,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import { api } from "@/lib/api";
import { AuthGuard } from "@/components/auth-guard";

interface ChatStats {
  total_conversations: number;
  total_messages: number;
  active_conversations: number;
  daily_message_count: number;
  daily_limit: number;
  limit_exceeded_users: number;
}

interface UserActivity {
  user_id: string;
  user_name: string;
  user_email: string;
  message_count: number;
  daily_limit: number;
  is_limit_exceeded: boolean;
  last_activity: string;
}

interface AgentActivity {
  agent_id: string;
  agent_name: string;
  conversation_count: number;
  message_count: number;
  last_activity: string;
  trust_score: number;
}

interface SystemConfig {
  daily_message_limit: number;
  chat_enabled: boolean;
  max_conversations_per_user: number;
  message_retention_days: number;
}

function ChatManagementContent() {
  const [stats, setStats] = useState<ChatStats | null>(null);
  const [userActivities, setUserActivities] = useState<UserActivity[]>([]);
  const [agentActivities, setAgentActivities] = useState<AgentActivity[]>([]);
  const [systemConfig, setSystemConfig] = useState<SystemConfig | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [filterType, setFilterType] = useState<"all" | "exceeded" | "active">(
    "all"
  );

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    try {
      setIsLoading(true);
      const [statsRes, usersRes, agentsRes, configRes] = await Promise.all([
        api.get("/chat/stats"),
        api.get("/chat/activity/users"),
        api.get("/chat/activity/agents"),
        api.get("/chat/config"),
      ]);

      setStats(statsRes.data);
      setUserActivities(usersRes.data.users || []);
      setAgentActivities(agentsRes.data.agents || []);
      setSystemConfig(configRes.data);
    } catch (err) {
      console.error("Error loading chat management data:", err);
    } finally {
      setIsLoading(false);
    }
  };

  const filteredUserActivities = userActivities.filter((user) => {
    const matchesSearch =
      user.user_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      user.user_email.toLowerCase().includes(searchTerm.toLowerCase());

    const matchesFilter =
      filterType === "all" ||
      (filterType === "exceeded" && user.is_limit_exceeded) ||
      (filterType === "active" && !user.is_limit_exceeded);

    return matchesSearch && matchesFilter;
  });

  const formatTime = (timestamp: string) => {
    return new Date(timestamp).toLocaleString();
  };

  const getLimitStatus = (user: UserActivity) => {
    if (user.is_limit_exceeded) {
      return <Badge variant="destructive">Exceeded</Badge>;
    }
    if (user.message_count > user.daily_limit * 0.8) {
      return <Badge variant="secondary">Near Limit</Badge>;
    }
    return <Badge variant="outline">Normal</Badge>;
  };

  if (isLoading) {
    return (
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold">Chat Management</h1>
            <p className="text-muted-foreground">
              Monitor and manage chat system
            </p>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {[...Array(4)].map((_, i) => (
            <Card key={i}>
              <CardHeader>
                <Skeleton className="h-6 w-32" />
              </CardHeader>
              <CardContent>
                <Skeleton className="h-8 w-16" />
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold">Chat Management</h1>
          <p className="text-muted-foreground">
            Monitor and manage chat system
          </p>
        </div>
        <Button onClick={loadData} variant="outline">
          <RefreshCw className="h-4 w-4 mr-2" />
          Refresh
        </Button>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Conversations
            </CardTitle>
            <MessageCircle className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.total_conversations || 0}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats?.active_conversations || 0} active
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Total Messages
            </CardTitle>
            <Activity className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.total_messages || 0}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats?.daily_message_count || 0} today
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Daily Limit Usage
            </CardTitle>
            <BarChart3 className="h-4 w-4 text-muted-foreground" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {stats?.daily_message_count || 0}/{stats?.daily_limit || 0}
            </div>
            <p className="text-xs text-muted-foreground">
              {stats?.limit_exceeded_users || 0} users exceeded
            </p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">System Status</CardTitle>
            {systemConfig?.chat_enabled ? (
              <CheckCircle className="h-4 w-4 text-green-500" />
            ) : (
              <AlertTriangle className="h-4 w-4 text-yellow-500" />
            )}
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {systemConfig?.chat_enabled ? "Enabled" : "Disabled"}
            </div>
            <p className="text-xs text-muted-foreground">
              {systemConfig?.max_conversations_per_user || 0} max per user
            </p>
          </CardContent>
        </Card>
      </div>

      {/* Management Tabs */}
      <Tabs defaultValue="users" className="space-y-4">
        <TabsList>
          <TabsTrigger value="users">User Activity</TabsTrigger>
          <TabsTrigger value="agents">Agent Activity</TabsTrigger>
          <TabsTrigger value="config">Configuration</TabsTrigger>
        </TabsList>

        {/* User Activity Tab */}
        <TabsContent value="users" className="space-y-4">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <div>
                  <CardTitle>User Activity</CardTitle>
                  <CardDescription>
                    Monitor user message usage and daily limits
                  </CardDescription>
                </div>
                <div className="flex items-center space-x-2">
                  <div className="relative">
                    <Search className="absolute left-2 top-2.5 h-4 w-4 text-muted-foreground" />
                    <Input
                      placeholder="Search users..."
                      value={searchTerm}
                      onChange={(e) => setSearchTerm(e.target.value)}
                      className="pl-8 w-64"
                    />
                  </div>
                  <select
                    value={filterType}
                    onChange={(e) => setFilterType(e.target.value as any)}
                    className="px-3 py-2 border rounded-md"
                  >
                    <option value="all">All Users</option>
                    <option value="exceeded">Limit Exceeded</option>
                    <option value="active">Active</option>
                  </select>
                </div>
              </div>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>User</TableHead>
                    <TableHead>Messages Today</TableHead>
                    <TableHead>Daily Limit</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Last Activity</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filteredUserActivities.map((user) => (
                    <TableRow key={user.user_id}>
                      <TableCell>
                        <div>
                          <div className="font-medium">{user.user_name}</div>
                          <div className="text-sm text-muted-foreground">
                            {user.user_email}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{user.message_count}</TableCell>
                      <TableCell>{user.daily_limit}</TableCell>
                      <TableCell>{getLimitStatus(user)}</TableCell>
                      <TableCell>{formatTime(user.last_activity)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Agent Activity Tab */}
        <TabsContent value="agents" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>Agent Activity</CardTitle>
              <CardDescription>
                Monitor agent performance and chat activity
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Agent</TableHead>
                    <TableHead>Conversations</TableHead>
                    <TableHead>Messages</TableHead>
                    <TableHead>Trust Score</TableHead>
                    <TableHead>Last Activity</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {agentActivities.map((agent) => (
                    <TableRow key={agent.agent_id}>
                      <TableCell>
                        <div className="flex items-center space-x-2">
                          <Bot className="h-4 w-4" />
                          <div>
                            <div className="font-medium">
                              {agent.agent_name}
                            </div>
                            <div className="text-sm text-muted-foreground">
                              ID: {agent.agent_id}
                            </div>
                          </div>
                        </div>
                      </TableCell>
                      <TableCell>{agent.conversation_count}</TableCell>
                      <TableCell>{agent.message_count}</TableCell>
                      <TableCell>
                        <Badge
                          variant={
                            agent.trust_score > 0.8 ? "default" : "secondary"
                          }
                        >
                          {(agent.trust_score * 100).toFixed(0)}%
                        </Badge>
                      </TableCell>
                      <TableCell>{formatTime(agent.last_activity)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </TabsContent>

        {/* Configuration Tab */}
        <TabsContent value="config" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle>System Configuration</CardTitle>
              <CardDescription>
                Manage chat system settings and limits
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">
                    Daily Message Limit
                  </label>
                  <Input
                    type="number"
                    value={systemConfig?.daily_message_limit || 5000}
                    disabled
                    className="mt-1"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">
                    Max Conversations per User
                  </label>
                  <Input
                    type="number"
                    value={systemConfig?.max_conversations_per_user || 100}
                    disabled
                    className="mt-1"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">
                    Message Retention (Days)
                  </label>
                  <Input
                    type="number"
                    value={systemConfig?.message_retention_days || 30}
                    disabled
                    className="mt-1"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Chat Status</label>
                  <div className="mt-1">
                    <Badge
                      variant={
                        systemConfig?.chat_enabled ? "default" : "secondary"
                      }
                    >
                      {systemConfig?.chat_enabled ? "Enabled" : "Disabled"}
                    </Badge>
                  </div>
                </div>
              </div>
              <div className="pt-4">
                <Button disabled>
                  <Settings className="h-4 w-4 mr-2" />
                  Update Configuration
                </Button>
                <p className="text-xs text-muted-foreground mt-2">
                  Configuration updates require admin privileges
                </p>
              </div>
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}

export default function ChatManagementPage() {
  return (
    <AuthGuard requiredRole="admin">
      <ChatManagementContent />
    </AuthGuard>
  );
}
