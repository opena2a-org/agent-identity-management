"use client";

import { useState, useEffect, useRef, useCallback } from "react";
import { useRouter } from "next/navigation";
import {
  MessageCircle,
  Send,
  Bot,
  User,
  MoreVertical,
  Trash2,
  Loader2,
  Plus,
} from "lucide-react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Badge } from "@/components/ui/badge";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { ScrollArea } from "@/components/ui/scroll-area";
import { api } from "@/lib/api";
import { AuthGuard } from "@/components/auth-guard";
import { Skeleton } from "@/components/ui/skeleton";
import { useToast } from "@/hooks/use-toast";

interface ChatConversation {
  id: string;
  title: string;
  agent_id: string;
  agent?: {
    id: string;
    name: string;
    display_name: string;
  };
  last_message_at?: string;
  created_at: string;
  updated_at: string;
  message_count: number;
}

interface ChatMessage {
  id: string;
  content: string;
  role: "user" | "agent" | "system";
  message_type: string;
  created_at: string;
  is_edited: boolean;
}

interface Agent {
  id: string;
  name: string;
  display_name: string;
}

interface DailyLimits {
  message_count: number;
  daily_limit: number;
  is_limit_exceeded: boolean;
  remaining_messages: number;
}

function ChatPageContent() {
  const router = useRouter();
  const { toast } = useToast();
  const [conversations, setConversations] = useState<ChatConversation[]>([]);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [dailyLimits, setDailyLimits] = useState<DailyLimits | null>(null);
  const [selectedConversation, setSelectedConversation] = useState<
    string | null
  >(null);
  const [newMessage, setNewMessage] = useState("");
  const [isLoading, setIsLoading] = useState(true);
  const [isSending, setIsSending] = useState(false);
  const [isWaitingForResponse, setIsWaitingForResponse] = useState(false);
  const [isLoadingMessages, setIsLoadingMessages] = useState(false);
  const [isCreatingConversation, setIsCreatingConversation] = useState(false);
  const [showNewChatDialog, setShowNewChatDialog] = useState(false);
  const [selectedAgentId, setSelectedAgentId] = useState<string>("");
  const [newChatTitle, setNewChatTitle] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Load initial data
  useEffect(() => {
    loadInitialData();
  }, []);

  // Load messages when conversation changes
  useEffect(() => {
    if (selectedConversation && !isLoadingMessages) {
      loadMessages(selectedConversation);
    }
  }, [selectedConversation]);

  // Auto-scroll to bottom when messages change
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const loadInitialData = async () => {
    try {
      setIsLoading(true);

      const [conversationsResponse, agentsResponse, limitsResponse] =
        await Promise.all([
          api.getChatConversations(),
          api.listAgents(),
          api.getChatDailyLimits(),
        ]);

      setConversations(conversationsResponse.conversations || []);
      setAgents(agentsResponse.agents || []);
      setDailyLimits(limitsResponse);

      // Auto-select first conversation if available
      if (
        conversationsResponse.conversations &&
        conversationsResponse.conversations.length > 0
      ) {
        handleConversationSelect(conversationsResponse.conversations[0].id);
      }
    } catch (err) {
      console.error("Error loading chat data:", err);
      toast({
        title: "Error",
        description: "Failed to load chat data",
        variant: "destructive",
      });
    } finally {
      setIsLoading(false);
    }
  };

  const loadMessages = useCallback(
    async (conversationId: string) => {
      // Prevent duplicate calls
      if (isLoadingMessages) {
        console.log("Already loading messages, skipping duplicate call");
        return;
      }

      try {
        setIsLoadingMessages(true);
        console.log("Loading messages for conversation:", conversationId);
        const response = await api.getChatMessages(conversationId);
        setMessages(response.messages || []);
      } catch (err) {
        console.error("Error loading messages:", err);
        toast({
          title: "Error",
          description: "Failed to load messages",
          variant: "destructive",
        });
      } finally {
        setIsLoadingMessages(false);
      }
    },
    [isLoadingMessages, toast]
  );

  const handleConversationSelect = useCallback(
    (conversationId: string) => {
      // Only load if conversation is different
      if (selectedConversation === conversationId) {
        console.log("Same conversation already selected, skipping");
        return;
      }
      console.log("Selecting conversation:", conversationId);
      setSelectedConversation(conversationId);
      // loadMessages will be called by useEffect when selectedConversation changes
    },
    [selectedConversation]
  );

  const handleDeleteConversation = async (conversationId: string) => {
    if (
      !confirm(
        "Are you sure you want to delete this conversation? This action cannot be undone."
      )
    ) {
      return;
    }

    try {
      await api.deleteChatConversation(conversationId);

      // Remove from list
      setConversations((prev) => prev.filter((c) => c.id !== conversationId));

      // If the deleted conversation was selected, clear selection
      if (selectedConversation === conversationId) {
        setSelectedConversation(null);
        setMessages([]);
      }

      toast({
        title: "Success",
        description: "Conversation deleted successfully",
      });
    } catch (err: any) {
      console.error("Error deleting conversation:", err);
      toast({
        title: "Error",
        description: "Failed to delete conversation. Please try again.",
        variant: "destructive",
      });
    }
  };

  const handleStartNewChat = () => {
    setShowNewChatDialog(true);
    setSelectedAgentId("");
    setNewChatTitle("");
  };

  const handleCreateConversation = async () => {
    if (!selectedAgentId || !newChatTitle.trim()) {
      toast({
        title: "Validation Error",
        description: "Please select an agent and enter a title",
        variant: "destructive",
      });
      return;
    }

    try {
      setIsCreatingConversation(true);

      const response = await api.createChatConversation({
        agent_id: selectedAgentId,
        title: newChatTitle.trim(),
      });

      // Backend returns the conversation directly (not wrapped in { conversation: {...} })
      const conversationData: any = response.conversation || response;

      // Find agent info
      const agent = agents.find((a) => a.id === conversationData.agent_id);

      // Add new conversation to list
      const newConv: ChatConversation = {
        id: conversationData.id,
        title: conversationData.title,
        agent_id: conversationData.agent_id,
        agent: agent
          ? {
              id: agent.id,
              name: agent.name,
              display_name: agent.display_name,
            }
          : undefined,
        created_at: conversationData.created_at,
        updated_at: conversationData.updated_at || conversationData.created_at,
        message_count: 0,
        last_message_at: conversationData.last_message_at || null,
      };

      setConversations((prev) => [newConv, ...prev]);
      setShowNewChatDialog(false);
      setNewChatTitle("");
      setSelectedAgentId("");

      // Select the new conversation (this will load messages)
      handleConversationSelect(conversationData.id);

      toast({
        title: "Success",
        description: "Conversation created successfully",
      });
    } catch (err: any) {
      console.error("Error creating conversation:", err);
      const errorMessage =
        err.response?.data?.error ||
        err.message ||
        "Failed to create conversation. Please try again.";
      toast({
        title: "Error",
        description: errorMessage,
        variant: "destructive",
      });
    } finally {
      setIsCreatingConversation(false);
    }
  };

  const handleSendMessage = async () => {
    if (!newMessage.trim() || !selectedConversation || isSending) return;

    const messageContent = newMessage.trim();
    setNewMessage(""); // Clear input immediately

    try {
      setIsSending(true);
      setIsWaitingForResponse(true);

      // Add user message to UI immediately
      const userMessage: ChatMessage = {
        id: `temp-user-${Date.now()}`,
        content: messageContent,
        role: "user",
        message_type: "text",
        created_at: new Date().toISOString(),
        is_edited: false,
      };

      setMessages((prev) => [...prev, userMessage]);

      // Send message to backend
      const response = await api.sendChatMessage({
        conversation_id: selectedConversation,
        content: messageContent,
        message_type: "text",
      });

      setIsWaitingForResponse(false);

      // Add agent response with typing effect
      if (response.message) {
        const agentMessage = response.message;
        const agentMessageId = `temp-agent-${Date.now()}`;

        // Add empty agent message first
        const emptyAgentMessage: ChatMessage = {
          id: agentMessageId,
          content: "",
          role: "agent",
          message_type: "text",
          created_at: new Date().toISOString(),
          is_edited: false,
        };

        setMessages((prev) => [...prev, emptyAgentMessage]);

        // Typing effect
        const words = agentMessage.content.split(" ");
        let currentText = "";

        for (let i = 0; i < words.length; i++) {
          currentText += (i > 0 ? " " : "") + words[i];

          setMessages((prev) =>
            prev.map((msg) =>
              msg.id === agentMessageId ? { ...msg, content: currentText } : msg
            )
          );

          // Delay between words (adjust speed here)
          await new Promise((resolve) => setTimeout(resolve, 30));
        }

        // Update with final message from backend
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === agentMessageId
              ? { ...agentMessage, id: agentMessage.id }
              : msg
          )
        );
      }

      // Refresh daily limits
      try {
        const limitsResponse = await api.getChatDailyLimits();
        setDailyLimits(limitsResponse);
      } catch (limitsErr) {
        console.error("Failed to refresh daily limits:", limitsErr);
      }
    } catch (err: any) {
      console.error("Error sending message:", err);

      // Check if backend returned limit exceeded error
      const errorMessage = err.response?.data?.error || err.message || "";
      const isLimitExceeded =
        err.response?.status === 429 ||
        errorMessage.toLowerCase().includes("limit exceeded") ||
        errorMessage.toLowerCase().includes("daily message limit");

      if (isLimitExceeded) {
        toast({
          title: "Daily Limit Exceeded",
          description: `You've reached your daily limit of ${dailyLimits?.daily_limit || 5000} messages. Please try again tomorrow.`,
          variant: "destructive",
        });
        // Refresh limits to update UI
        try {
          const limitsResponse = await api.getChatDailyLimits();
          setDailyLimits(limitsResponse);
        } catch (limitsErr) {
          console.error("Failed to refresh daily limits:", limitsErr);
        }
      } else {
        // Show the actual error message from backend or generic message
        const displayError =
          err.response?.data?.error ||
          "Failed to send message. Please try again.";
        toast({
          title: "Error",
          description: displayError,
          variant: "destructive",
        });
      }
    } finally {
      setIsSending(false);
      setIsWaitingForResponse(false);
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
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

  if (isLoading) {
    return (
      <div className="h-[calc(100vh-8rem)] flex">
        <div className="w-80 border-r border-gray-200 dark:border-gray-800 p-4">
          <Skeleton className="h-10 w-full mb-4" />
          <div className="space-y-2">
            {[...Array(5)].map((_, i) => (
              <Skeleton key={i} className="h-16 w-full" />
            ))}
          </div>
        </div>
        <div className="flex-1 flex items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-gray-400" />
        </div>
      </div>
    );
  }

  return (
    <div className="h-[calc(100vh-8rem)] flex flex-col">
      {/* Compact Header */}
      <div className="flex items-center justify-between mb-4 px-1">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
            Chat
          </h1>
          <p className="text-sm text-gray-500 dark:text-gray-400 mt-0.5">
            Communicate with AI agents
          </p>
        </div>
        {dailyLimits && (
          <Badge
            variant={dailyLimits.is_limit_exceeded ? "destructive" : "outline"}
            className="text-xs font-medium px-3 py-1.5"
          >
            {dailyLimits.remaining_messages} / {dailyLimits.daily_limit}
          </Badge>
        )}
      </div>

      {/* Main Chat Layout */}
      <div className="flex-1 flex gap-4 min-h-0">
        {/* Sidebar - Conversations */}
        <div className="w-80 flex flex-col border border-gray-200 dark:border-gray-800 rounded-lg bg-white dark:bg-gray-950">
          <div className="p-4 border-b border-gray-200 dark:border-gray-800">
            <button
              onClick={handleStartNewChat}
              className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-blue-600 text-white text-sm font-medium rounded-md hover:bg-blue-700 transition-colors"
            >
              <Plus className="h-4 w-4" />
              New Chat
            </button>
          </div>

          <ScrollArea className="flex-1">
            <div className="p-2">
              {conversations.length === 0 ? (
                <div className="text-center py-12 px-4">
                  <MessageCircle className="h-10 w-10 mx-auto mb-3 text-gray-300 dark:text-gray-700" />
                  <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                    No conversations yet
                  </p>
                  <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                    Click "New Chat" to start
                  </p>
                </div>
              ) : (
                conversations.map((conversation) => (
                  <div
                    key={conversation.id}
                    className={`group relative p-3 rounded-md cursor-pointer transition-colors mb-1 ${
                      selectedConversation === conversation.id
                        ? "bg-blue-50 dark:bg-blue-950/30 border border-blue-200 dark:border-blue-900"
                        : "hover:bg-gray-50 dark:hover:bg-gray-900 border border-transparent"
                    }`}
                    onClick={() => handleConversationSelect(conversation.id)}
                  >
                    <div className="flex items-start justify-between gap-2">
                      <div className="flex-1 min-w-0">
                        <p className="text-sm font-medium text-gray-900 dark:text-gray-100 truncate">
                          {conversation.title}
                        </p>
                        <p className="text-xs text-gray-500 dark:text-gray-400 truncate mt-0.5">
                          {conversation.agent?.display_name ||
                            conversation.agent?.name ||
                            "Unknown Agent"}
                        </p>
                        {conversation.last_message_at && (
                          <p className="text-xs text-gray-400 dark:text-gray-500 mt-1">
                            {formatTime(conversation.last_message_at)}
                          </p>
                        )}
                      </div>
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <button
                            className="opacity-0 group-hover:opacity-100 h-6 w-6 rounded hover:bg-gray-200 dark:hover:bg-gray-700 transition-all flex items-center justify-center"
                            onClick={(e) => e.stopPropagation()}
                          >
                            <MoreVertical className="h-3.5 w-3.5 text-gray-600 dark:text-gray-400" />
                          </button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-40">
                          <DropdownMenuItem
                            className="text-red-600 dark:text-red-400 text-sm"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleDeleteConversation(conversation.id);
                            }}
                          >
                            <Trash2 className="h-3.5 w-3.5 mr-2" />
                            Delete
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    </div>
                  </div>
                ))
              )}
            </div>
          </ScrollArea>
        </div>

        {/* Main Chat Area */}
        <div className="flex-1 flex flex-col border border-gray-200 dark:border-gray-800 rounded-lg bg-white dark:bg-gray-950 min-w-0">
          {selectedConversation ? (
            <>
              {/* Messages */}
              <ScrollArea className="flex-1 p-6">
                <div className="space-y-4 max-w-4xl mx-auto">
                  {isLoadingMessages ? (
                    <div className="space-y-4">
                      {[...Array(3)].map((_, i) => (
                        <div
                          key={i}
                          className={`flex ${i % 2 === 0 ? "justify-end" : "justify-start"}`}
                        >
                          <Skeleton className="h-20 w-80" />
                        </div>
                      ))}
                    </div>
                  ) : messages.length === 0 ? (
                    <div className="text-center py-20">
                      <MessageCircle className="h-12 w-12 mx-auto mb-4 text-gray-300 dark:text-gray-700" />
                      <p className="text-sm font-medium text-gray-900 dark:text-gray-100">
                        No messages yet
                      </p>
                      <p className="text-xs text-gray-500 dark:text-gray-400 mt-1">
                        Start the conversation below
                      </p>
                    </div>
                  ) : (
                    <>
                      {messages.map((message) => (
                        <div
                          key={message.id}
                          className={`flex ${message.role === "user" ? "justify-end" : "justify-start"}`}
                        >
                          <div
                            className={`max-w-[75%] rounded-lg px-4 py-3 ${
                              message.role === "user"
                                ? "bg-blue-600 text-white"
                                : "bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100"
                            }`}
                          >
                            <div className="flex items-start gap-2">
                              {message.role === "agent" && (
                                <Bot className="h-4 w-4 mt-0.5 flex-shrink-0 opacity-70" />
                              )}
                              {message.role === "user" && (
                                <User className="h-4 w-4 mt-0.5 flex-shrink-0 opacity-70" />
                              )}
                              <div className="flex-1 min-w-0">
                                <p className="text-sm whitespace-pre-wrap break-words">
                                  {message.content}
                                </p>
                                <div className="flex items-center gap-2 mt-2">
                                  <span
                                    className={`text-xs ${message.role === "user" ? "text-blue-100" : "text-gray-500 dark:text-gray-400"}`}
                                  >
                                    {formatTime(message.created_at)}
                                  </span>
                                  {message.is_edited && (
                                    <span
                                      className={`text-xs ${message.role === "user" ? "text-blue-200" : "text-gray-400 dark:text-gray-500"}`}
                                    >
                                      (edited)
                                    </span>
                                  )}
                                </div>
                              </div>
                            </div>
                          </div>
                        </div>
                      ))}

                      {/* Typing indicator - only show while waiting for backend response */}
                      {isWaitingForResponse && (
                        <div className="flex justify-start">
                          <div className="max-w-[75%] rounded-lg px-4 py-3 bg-gray-100 dark:bg-gray-800">
                            <div className="flex items-center gap-2">
                              <Bot className="h-4 w-4 flex-shrink-0 text-gray-600 dark:text-gray-400" />
                              <div className="flex gap-1">
                                <div
                                  className="w-2 h-2 bg-gray-400 dark:bg-gray-600 rounded-full animate-bounce"
                                  style={{ animationDelay: "0ms" }}
                                ></div>
                                <div
                                  className="w-2 h-2 bg-gray-400 dark:bg-gray-600 rounded-full animate-bounce"
                                  style={{ animationDelay: "150ms" }}
                                ></div>
                                <div
                                  className="w-2 h-2 bg-gray-400 dark:bg-gray-600 rounded-full animate-bounce"
                                  style={{ animationDelay: "300ms" }}
                                ></div>
                              </div>
                            </div>
                          </div>
                        </div>
                      )}
                    </>
                  )}
                  <div ref={messagesEndRef} />
                </div>
              </ScrollArea>

              {/* Message Input */}
              <div className="border-t border-gray-200 dark:border-gray-800 p-4">
                <div className="max-w-4xl mx-auto">
                  <div className="flex gap-2">
                    <Textarea
                      value={newMessage}
                      onChange={(e) => setNewMessage(e.target.value)}
                      onKeyPress={handleKeyPress}
                      placeholder="Type your message..."
                      className="min-h-[44px] max-h-32 resize-none text-sm"
                      disabled={isSending}
                    />
                    <button
                      onClick={handleSendMessage}
                      disabled={!newMessage.trim() || isSending}
                      className="px-4 h-[44px] bg-blue-600 text-white rounded-md hover:bg-blue-700 transition-colors disabled:bg-gray-300 dark:disabled:bg-gray-700 disabled:cursor-not-allowed flex items-center justify-center flex-shrink-0"
                    >
                      {isSending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Send className="h-4 w-4" />
                      )}
                    </button>
                  </div>
                  {dailyLimits && (
                    <p className="text-xs text-gray-500 dark:text-gray-400 mt-2">
                      {dailyLimits.remaining_messages} messages remaining today
                    </p>
                  )}
                </div>
              </div>
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center">
                <MessageCircle className="h-16 w-16 mx-auto mb-4 text-gray-300 dark:text-gray-700" />
                <p className="text-base font-medium text-gray-900 dark:text-gray-100">
                  Select a conversation
                </p>
                <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                  Choose a conversation from the sidebar or start a new one
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* New Chat Dialog */}
      {showNewChatDialog && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
          <Card className="w-full max-w-md">
            <CardHeader>
              <CardTitle className="text-lg">Start New Chat</CardTitle>
              <CardDescription className="text-sm">
                Choose an agent to start a conversation with
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div>
                <label className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  Agent
                </label>
                <select
                  value={selectedAgentId}
                  onChange={(e) => setSelectedAgentId(e.target.value)}
                  disabled={isCreatingConversation}
                  className="w-full mt-1.5 p-2.5 text-sm border border-gray-300 dark:border-gray-600 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-transparent bg-white dark:bg-gray-900 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  <option value="">Select an agent...</option>
                  {agents.map((agent) => (
                    <option key={agent.id} value={agent.id}>
                      {agent.display_name} ({agent.name})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-sm font-medium text-gray-900 dark:text-gray-100">
                  Chat Title
                </label>
                <Input
                  value={newChatTitle}
                  onChange={(e) => setNewChatTitle(e.target.value)}
                  placeholder="Enter a title for this conversation..."
                  disabled={isCreatingConversation}
                  className="mt-1.5 text-sm"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  onClick={() => setShowNewChatDialog(false)}
                  disabled={isCreatingConversation}
                  className="px-4 py-2 text-sm border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 rounded-md hover:bg-gray-50 dark:hover:bg-gray-800 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Cancel
                </button>
                <button
                  onClick={handleCreateConversation}
                  disabled={
                    !selectedAgentId ||
                    !newChatTitle.trim() ||
                    isCreatingConversation
                  }
                  className="flex items-center gap-2 px-4 py-2 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:bg-gray-300 dark:disabled:bg-gray-700 disabled:cursor-not-allowed transition-colors"
                >
                  {isCreatingConversation ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      Creating...
                    </>
                  ) : (
                    "Start Chat"
                  )}
                </button>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}

export default function ChatPage() {
  return (
    <AuthGuard>
      <ChatPageContent />
    </AuthGuard>
  );
}
