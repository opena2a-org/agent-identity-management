"use client";

import { useState, useEffect, useMemo, Suspense } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  Code,
  Copy,
  CheckCircle,
  Search,
  ChevronRight,
  ChevronDown,
  Play,
  Filter,
  X,
  AlertCircle,
  Lock,
  Unlock,
  BookOpen,
  Braces,
  FileJson,
  Zap,
  Bot,
  Server,
  Key,
  Activity,
  Tag,
  Download,
  AlertTriangle,
  ShieldCheck,
  Users,
  CheckSquare,
  ClipboardCheck,
  BarChart3,
  Shield,
  Plug,
} from "lucide-react";
import { toast } from "sonner";
import {
  apiDocumentation,
  type APIEndpoint,
  type EndpointCategory,
} from "@/lib/api-documentation";
import { DevelopersPageSkeleton } from "@/components/ui/content-loaders";

// Icon map for categories
const categoryIcons: Record<string, any> = {
  Lock,
  User: Users,
  Bot,
  Plug,
  Shield,
  Server,
  Key,
  Activity,
  Tag,
  Search,
  Download,
  AlertTriangle,
  ShieldCheck,
  Users,
  CheckSquare,
  ClipboardCheck,
  BarChart3,
};

function DevelopersPageContent() {
  // State
  const [isLoading, setIsLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState("");
  const [selectedCategory, setSelectedCategory] = useState<string>(
    apiDocumentation[0].category
  );
  const [selectedEndpoint, setSelectedEndpoint] = useState<APIEndpoint | null>(
    null
  );
  const [copiedCode, setCopiedCode] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState("overview");
  const [collapsedCategories, setCollapsedCategories] = useState<Set<string>>(
    new Set()
  );

  // Filters
  const [filterMethod, setFilterMethod] = useState<string>("all");
  const [filterRole, setFilterRole] = useState<string>("all");
  const [showFilters, setShowFilters] = useState(false);

  // API Playground state
  const [isAuthenticated, setIsAuthenticated] = useState(false);
  const [userToken, setUserToken] = useState<string>("");
  const [manualToken, setManualToken] = useState<string>("");
  const [showTokenInput, setShowTokenInput] = useState(false);
  const [requestBody, setRequestBody] = useState<string>("{}");
  const [responseData, setResponseData] = useState<any>(null);
  const [isExecuting, setIsExecuting] = useState(false);
  const [executionError, setExecutionError] = useState<string | null>(null);
  const [responseMetadata, setResponseMetadata] = useState<{
    status: number;
    statusText: string;
    headers: Record<string, string>;
    duration: number;
  } | null>(null);

  // Check authentication status and initialize on mount
  useEffect(() => {
    const token = localStorage.getItem("auth_token");
    if (token) {
      setIsAuthenticated(true);
      setUserToken(token);
    }
    // Simulate initial load delay
    setTimeout(() => setIsLoading(false), 300);
  }, []);

  // Auto-select first endpoint when category changes
  useEffect(() => {
    const category = apiDocumentation.find(
      (c) => c.category === selectedCategory
    );
    if (category && category.endpoints.length > 0) {
      setSelectedEndpoint(category.endpoints[0]);
      setResponseData(null);
      setExecutionError(null);
    }
  }, [selectedCategory]);

  // Filtered categories and endpoints
  const filteredData = useMemo(() => {
    return apiDocumentation
      .map((category) => {
        const filteredEndpoints = category.endpoints.filter((endpoint) => {
          // Search filter
          const matchesSearch =
            searchTerm === "" ||
            endpoint.path.toLowerCase().includes(searchTerm.toLowerCase()) ||
            endpoint.summary.toLowerCase().includes(searchTerm.toLowerCase()) ||
            endpoint.description
              .toLowerCase()
              .includes(searchTerm.toLowerCase()) ||
            endpoint.tags.some((tag) =>
              tag.toLowerCase().includes(searchTerm.toLowerCase())
            );

          // Method filter
          const matchesMethod =
            filterMethod === "all" || endpoint.method === filterMethod;

          // Role filter
          const matchesRole =
            filterRole === "all" || endpoint.roleRequired === filterRole;

          return matchesSearch && matchesMethod && matchesRole;
        });

        return {
          ...category,
          endpoints: filteredEndpoints,
          matchCount: filteredEndpoints.length,
        };
      })
      .filter((category) => category.matchCount > 0);
  }, [searchTerm, filterMethod, filterRole]);

  // Total endpoint count
  const totalEndpoints = useMemo(() => {
    return filteredData.reduce((sum, cat) => sum + cat.matchCount, 0);
  }, [filteredData]);

  // Copy to clipboard
  const copyToClipboard = async (text: string, type: string) => {
    await navigator.clipboard.writeText(text);
    setCopiedCode(type);
    toast.success("Copied to clipboard!");
    setTimeout(() => setCopiedCode(null), 2000);
  };

  // Toggle category collapse
  const toggleCategory = (category: string) => {
    const newCollapsed = new Set(collapsedCategories);
    if (newCollapsed.has(category)) {
      newCollapsed.delete(category);
    } else {
      newCollapsed.add(category);
    }
    setCollapsedCategories(newCollapsed);
  };

  // Clear all filters
  const clearFilters = () => {
    setSearchTerm("");
    setFilterMethod("all");
    setFilterRole("all");
  };

  // Execute API request
  const executeRequest = async () => {
    if (!selectedEndpoint) return;

    setIsExecuting(true);
    setExecutionError(null);
    setResponseData(null);
    setResponseMetadata(null);

    const startTime = performance.now();

    try {
      const token = isAuthenticated ? userToken : manualToken;
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      };

      if (selectedEndpoint.requiresAuth && token) {
        headers["Authorization"] = `Bearer ${token}`;
      }

      const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
      let url = `${apiUrl}${selectedEndpoint.path}`;

      // Replace path parameters with actual values
      const pathParams = selectedEndpoint.path.match(/:(\w+)/g);
      if (pathParams) {
        try {
          const bodyData = JSON.parse(requestBody);
          pathParams.forEach((param) => {
            const paramName = param.substring(1);
            if (bodyData[paramName]) {
              url = url.replace(param, bodyData[paramName]);
              delete bodyData[paramName];
            }
          });
        } catch (e) {
          // Ignore JSON parse errors for now
        }
      }

      const options: RequestInit = {
        method: selectedEndpoint.method,
        headers,
      };

      if (selectedEndpoint.method !== "GET" && requestBody !== "{}") {
        options.body = requestBody;
      }

      const response = await fetch(url, options);
      const endTime = performance.now();
      const duration = Math.round(endTime - startTime);

      const data = await response.json();

      setResponseMetadata({
        status: response.status,
        statusText: response.statusText,
        headers: Object.fromEntries(response.headers.entries()),
        duration,
      });

      if (response.ok) {
        setResponseData(data);
        toast.success(`Request successful (${duration}ms)`);
      } else {
        setExecutionError(JSON.stringify(data, null, 2));
        toast.error(`Request failed: ${response.statusText}`);
      }
    } catch (error: any) {
      const endTime = performance.now();
      const duration = Math.round(endTime - startTime);
      setExecutionError(error.message || "Network error occurred");
      setResponseMetadata({
        status: 0,
        statusText: "Network Error",
        headers: {},
        duration,
      });
      toast.error("Request failed: " + error.message);
    } finally {
      setIsExecuting(false);
    }
  };

  // Generate cURL command
  const generateCurl = () => {
    if (!selectedEndpoint) return "";

    const token = isAuthenticated ? userToken : manualToken;
    const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    let curl = `curl -X ${selectedEndpoint.method} '${apiUrl}${selectedEndpoint.path}'`;

    if (selectedEndpoint.requiresAuth && token) {
      curl += ` \\\n  -H 'Authorization: Bearer ${token}'`;
    }

    curl += ` \\\n  -H 'Content-Type: application/json'`;

    if (selectedEndpoint.method !== "GET" && requestBody !== "{}") {
      curl += ` \\\n  -d '${requestBody}'`;
    }

    return curl;
  };

  // Method badge color
  const getMethodColor = (method: string) => {
    switch (method) {
      case "GET":
        return "bg-blue-500";
      case "POST":
        return "bg-green-500";
      case "PUT":
        return "bg-yellow-500";
      case "DELETE":
        return "bg-red-500";
      case "PATCH":
        return "bg-purple-500";
      default:
        return "bg-gray-500";
    }
  };

  // Active filters count
  const activeFiltersCount =
    (searchTerm !== "" ? 1 : 0) +
    (filterMethod !== "all" ? 1 : 0) +
    (filterRole !== "all" ? 1 : 0);

  if (isLoading) {
    return <DevelopersPageSkeleton />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-3xl font-bold text-gray-900 dark:text-white">
          API Documentation
        </h1>
        <p className="text-gray-600 dark:text-gray-400 mt-2">
          Complete AIM API reference with{" "}
          {apiDocumentation.reduce((sum, cat) => sum + cat.endpoints.length, 0)}{" "}
          endpoints
        </p>
      </div>

      {/* Search and Filters */}
      <Card>
        <CardContent className="pt-6">
          <div className="flex flex-col gap-4">
            {/* Search Bar */}
            <div className="flex gap-2">
              <div className="relative flex-1">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-gray-400" />
                <Input
                  type="text"
                  placeholder="Search endpoints, tags, descriptions..."
                  value={searchTerm}
                  onChange={(e) => setSearchTerm(e.target.value)}
                  className="pl-10"
                />
              </div>
              <Button
                variant={showFilters ? "default" : "outline"}
                onClick={() => setShowFilters(!showFilters)}
              >
                <Filter className="h-4 w-4 mr-2" />
                Filters
                {activeFiltersCount > 0 && (
                  <Badge variant="destructive" className="ml-2">
                    {activeFiltersCount}
                  </Badge>
                )}
              </Button>
            </div>

            {/* Advanced Filters */}
            {showFilters && (
              <div className="flex flex-wrap gap-4 p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                <div className="flex-1 min-w-[200px]">
                  <label className="text-sm font-medium mb-2 block">
                    HTTP Method
                  </label>
                  <select
                    value={filterMethod}
                    onChange={(e) => setFilterMethod(e.target.value)}
                    className="w-full px-3 py-2 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700"
                  >
                    <option value="all">All Methods</option>
                    <option value="GET">GET</option>
                    <option value="POST">POST</option>
                    <option value="PUT">PUT</option>
                    <option value="DELETE">DELETE</option>
                    <option value="PATCH">PATCH</option>
                  </select>
                </div>

                <div className="flex-1 min-w-[200px]">
                  <label className="text-sm font-medium mb-2 block">
                    Required Role
                  </label>
                  <select
                    value={filterRole}
                    onChange={(e) => setFilterRole(e.target.value)}
                    className="w-full px-3 py-2 rounded-md border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-700"
                  >
                    <option value="all">All Roles</option>
                    <option value="admin">Admin Only</option>
                    <option value="manager">Manager+</option>
                    <option value="member">Member+</option>
                    <option value="viewer">Viewer+</option>
                  </select>
                </div>

                {activeFiltersCount > 0 && (
                  <div className="flex items-end">
                    <Button variant="ghost" onClick={clearFilters}>
                      <X className="h-4 w-4 mr-2" />
                      Clear Filters
                    </Button>
                  </div>
                )}
              </div>
            )}

            {/* Results Count */}
            <div className="text-sm text-gray-600 dark:text-gray-400">
              Showing <span className="font-semibold">{totalEndpoints}</span> of{" "}
              <span className="font-semibold">
                {apiDocumentation.reduce(
                  (sum, cat) => sum + cat.endpoints.length,
                  0
                )}
              </span>{" "}
              endpoints
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Main Content: Sidebar + Details */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-6">
        {/* Left Sidebar: Categories & Endpoints */}
        <div className="lg:col-span-4 xl:col-span-3">
          <Card className="sticky top-20 max-h-[calc(100vh-8rem)] overflow-y-auto">
            <CardHeader>
              <CardTitle className="text-lg">Categories</CardTitle>
            </CardHeader>
            <CardContent className="space-y-2">
              {filteredData.map((category) => {
                const Icon = categoryIcons[category.icon] || BookOpen;
                const isCollapsed = collapsedCategories.has(category.category);

                return (
                  <div key={category.category} className="space-y-1">
                    {/* Category Header */}
                    <button
                      onClick={() => toggleCategory(category.category)}
                      className="w-full flex items-center justify-between p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
                    >
                      <div className="flex items-center gap-2">
                        <Icon className="h-4 w-4 text-blue-600 dark:text-blue-400" />
                        <span className="font-medium text-sm">
                          {category.category}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary" className="text-xs">
                          {category.matchCount}
                        </Badge>
                        {isCollapsed ? (
                          <ChevronRight className="h-4 w-4" />
                        ) : (
                          <ChevronDown className="h-4 w-4" />
                        )}
                      </div>
                    </button>

                    {/* Endpoints List */}
                    {!isCollapsed && (
                      <div className="ml-6 space-y-1">
                        {category.endpoints.map((endpoint, idx) => (
                          <button
                            key={idx}
                            onClick={() => {
                              setSelectedEndpoint(endpoint);
                              setSelectedCategory(category.category);
                              setActiveTab("overview");
                              setResponseData(null);
                              setExecutionError(null);
                            }}
                            className={`w-full text-left p-2 rounded text-sm transition-colors ${
                              selectedEndpoint === endpoint
                                ? "bg-blue-50 dark:bg-blue-900/20 text-blue-600 dark:text-blue-400"
                                : "hover:bg-gray-100 dark:hover:bg-gray-800"
                            }`}
                          >
                            <div className="flex items-center gap-2">
                              <Badge
                                className={`${getMethodColor(endpoint.method)} text-white text-xs px-2 py-0`}
                              >
                                {endpoint.method}
                              </Badge>
                              <span className="truncate">
                                {endpoint.summary}
                              </span>
                            </div>
                          </button>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}

              {filteredData.length === 0 && (
                <div className="text-center py-8 text-gray-500">
                  <Search className="h-12 w-12 mx-auto mb-2 opacity-20" />
                  <p className="text-sm">No endpoints match your filters</p>
                </div>
              )}
            </CardContent>
          </Card>
        </div>

        {/* Right Panel: Endpoint Details */}
        <div className="lg:col-span-8 xl:col-span-9">
          {selectedEndpoint ? (
            <Card>
              <CardHeader>
                <div className="space-y-2">
                  <div className="flex items-center gap-2">
                    <Badge
                      className={`${getMethodColor(selectedEndpoint.method)} text-white`}
                    >
                      {selectedEndpoint.method}
                    </Badge>
                    <code className="text-lg font-mono">
                      {selectedEndpoint.path}
                    </code>
                  </div>
                  <CardDescription className="text-base">
                    {selectedEndpoint.description}
                  </CardDescription>
                  <div className="flex flex-wrap gap-2 mt-2">
                    {selectedEndpoint.tags.map((tag, idx) => (
                      <Badge key={idx} variant="outline" className="text-xs">
                        {tag}
                      </Badge>
                    ))}
                    {selectedEndpoint.requiresAuth && (
                      <Badge variant="destructive" className="text-xs">
                        <Lock className="h-3 w-3 mr-1" />
                        Auth Required
                      </Badge>
                    )}
                    {selectedEndpoint.roleRequired && (
                      <Badge variant="default" className="text-xs">
                        Role: {selectedEndpoint.roleRequired}
                      </Badge>
                    )}
                  </div>
                </div>
              </CardHeader>

              <CardContent>
                <Tabs value={activeTab} onValueChange={setActiveTab}>
                  <TabsList className="grid w-full grid-cols-3">
                    <TabsTrigger value="overview">
                      <BookOpen className="h-4 w-4 mr-2" />
                      Overview
                    </TabsTrigger>
                    <TabsTrigger value="try-it">
                      <Play className="h-4 w-4 mr-2" />
                      Try it out
                    </TabsTrigger>
                    <TabsTrigger value="code">
                      <Code className="h-4 w-4 mr-2" />
                      Code
                    </TabsTrigger>
                  </TabsList>

                  {/* Overview Tab */}
                  <TabsContent value="overview" className="space-y-6 mt-6">
                    {/* Authentication Info */}
                    <div>
                      <h3 className="text-sm font-semibold mb-2">
                        Authentication
                      </h3>
                      <div className="flex items-center gap-2 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
                        {selectedEndpoint.requiresAuth ? (
                          <>
                            <Lock className="h-4 w-4 text-red-600" />
                            <span className="text-sm">
                              {selectedEndpoint.auth}
                            </span>
                          </>
                        ) : (
                          <>
                            <Unlock className="h-4 w-4 text-green-600" />
                            <span className="text-sm">
                              No authentication required
                            </span>
                          </>
                        )}
                      </div>
                    </div>

                    {/* Request Schema */}
                    {selectedEndpoint.requestSchema && (
                      <div>
                        <h3 className="text-sm font-semibold mb-2">
                          Request Body
                        </h3>
                        <div className="space-y-2">
                          {Object.entries(
                            selectedEndpoint.requestSchema.properties
                          ).map(([key, prop]) => (
                            <div
                              key={key}
                              className="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                            >
                              <div className="flex items-center justify-between mb-1">
                                <code className="text-sm font-mono">{key}</code>
                                <div className="flex gap-2">
                                  <Badge variant="outline" className="text-xs">
                                    {prop.type}
                                  </Badge>
                                  {prop.required && (
                                    <Badge
                                      variant="destructive"
                                      className="text-xs"
                                    >
                                      Required
                                    </Badge>
                                  )}
                                </div>
                              </div>
                              <p className="text-sm text-gray-600 dark:text-gray-400">
                                {prop.description}
                              </p>
                              {(prop as any).example && (
                                <code className="text-xs text-gray-500 mt-1 block">
                                  Example:{" "}
                                  {JSON.stringify((prop as any).example)}
                                </code>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Response Schema */}
                    {selectedEndpoint.responseSchema && (
                      <div>
                        <h3 className="text-sm font-semibold mb-2">
                          Response Body
                        </h3>
                        <div className="space-y-2">
                          {Object.entries(
                            selectedEndpoint.responseSchema.properties
                          ).map(([key, prop]) => (
                            <div
                              key={key}
                              className="p-3 bg-gray-50 dark:bg-gray-800 rounded-lg"
                            >
                              <div className="flex items-center justify-between mb-1">
                                <code className="text-sm font-mono">{key}</code>
                                <Badge variant="outline" className="text-xs">
                                  {prop.type}
                                </Badge>
                              </div>
                              <p className="text-sm text-gray-600 dark:text-gray-400">
                                {prop.description}
                              </p>
                              {(prop as any).example && (
                                <code className="text-xs text-gray-500 mt-1 block">
                                  Example:{" "}
                                  {JSON.stringify((prop as any).example)}
                                </code>
                              )}
                            </div>
                          ))}
                        </div>
                      </div>
                    )}

                    {/* Example Request */}
                    {selectedEndpoint.example &&
                      selectedEndpoint.example !==
                        "No request body required" && (
                        <div>
                          <div className="flex items-center justify-between mb-2">
                            <h3 className="text-sm font-semibold">
                              Example Request
                            </h3>
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={() =>
                                copyToClipboard(
                                  selectedEndpoint.example,
                                  "example"
                                )
                              }
                            >
                              {copiedCode === "example" ? (
                                <CheckCircle className="h-4 w-4" />
                              ) : (
                                <Copy className="h-4 w-4" />
                              )}
                            </Button>
                          </div>
                          <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-x-auto text-sm">
                            <code>{selectedEndpoint.example}</code>
                          </pre>
                        </div>
                      )}
                  </TabsContent>

                  {/* Try it out Tab */}
                  <TabsContent value="try-it" className="space-y-4 mt-6">
                    {/* Authentication Status */}
                    <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                      {isAuthenticated ? (
                        <div className="flex items-center justify-between">
                          <div className="flex items-center gap-2">
                            <CheckCircle className="h-5 w-5 text-green-600" />
                            <span className="font-medium">Authenticated</span>
                          </div>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() => {
                              setIsAuthenticated(false);
                              setUserToken("");
                              localStorage.removeItem("auth_token");
                            }}
                          >
                            Logout
                          </Button>
                        </div>
                      ) : (
                        <div className="space-y-2">
                          <div className="flex items-center gap-2">
                            <AlertCircle className="h-5 w-5 text-yellow-600" />
                            <span className="font-medium">
                              Not Authenticated
                            </span>
                          </div>
                          {!showTokenInput ? (
                            <Button
                              size="sm"
                              onClick={() => setShowTokenInput(true)}
                            >
                              Add Token
                            </Button>
                          ) : (
                            <div className="flex gap-2">
                              <Input
                                type="password"
                                placeholder="Enter Bearer token"
                                value={manualToken}
                                onChange={(e) => setManualToken(e.target.value)}
                                className="flex-1"
                              />
                              <Button
                                size="sm"
                                onClick={() => {
                                  if (manualToken) {
                                    setUserToken(manualToken);
                                    setIsAuthenticated(true);
                                    setShowTokenInput(false);
                                    toast.success("Token added");
                                  }
                                }}
                              >
                                Save
                              </Button>
                            </div>
                          )}
                        </div>
                      )}
                    </div>

                    {/* Request Body Editor */}
                    {selectedEndpoint.method !== "GET" && (
                      <div>
                        <label className="text-sm font-semibold mb-2 block">
                          Request Body
                        </label>
                        <Textarea
                          value={requestBody}
                          onChange={(e) => setRequestBody(e.target.value)}
                          rows={10}
                          className="font-mono text-sm"
                          placeholder={selectedEndpoint.example}
                        />
                      </div>
                    )}

                    {/* Execute Button */}
                    <Button
                      onClick={executeRequest}
                      disabled={isExecuting}
                      className="w-full"
                      size="lg"
                    >
                      {isExecuting ? (
                        <>Loading...</>
                      ) : (
                        <>
                          <Play className="h-4 w-4 mr-2" />
                          Execute Request
                        </>
                      )}
                    </Button>

                    {/* Response Metadata */}
                    {responseMetadata && (
                      <div className="p-4 bg-gray-50 dark:bg-gray-800 rounded-lg">
                        <div className="flex items-center justify-between">
                          <span className="text-sm font-medium">Response</span>
                          <div className="flex gap-2">
                            <Badge
                              variant={
                                responseMetadata.status < 400
                                  ? "default"
                                  : "destructive"
                              }
                            >
                              {responseMetadata.status}{" "}
                              {responseMetadata.statusText}
                            </Badge>
                            <Badge variant="outline">
                              {responseMetadata.duration}ms
                            </Badge>
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Response Data */}
                    {responseData && (
                      <div>
                        <div className="flex items-center justify-between mb-2">
                          <h3 className="text-sm font-semibold">
                            Response Body
                          </h3>
                          <Button
                            size="sm"
                            variant="outline"
                            onClick={() =>
                              copyToClipboard(
                                JSON.stringify(responseData, null, 2),
                                "response"
                              )
                            }
                          >
                            {copiedCode === "response" ? (
                              <CheckCircle className="h-4 w-4" />
                            ) : (
                              <Copy className="h-4 w-4" />
                            )}
                          </Button>
                        </div>
                        <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-x-auto text-sm max-h-96">
                          <code>{JSON.stringify(responseData, null, 2)}</code>
                        </pre>
                      </div>
                    )}

                    {/* Error Response */}
                    {executionError && (
                      <div className="p-4 bg-red-50 dark:bg-red-900/20 rounded-lg">
                        <div className="flex items-center gap-2 mb-2">
                          <AlertCircle className="h-5 w-5 text-red-600" />
                          <span className="font-semibold text-red-600">
                            Error
                          </span>
                        </div>
                        <pre className="text-sm text-red-800 dark:text-red-200 overflow-x-auto">
                          {executionError}
                        </pre>
                      </div>
                    )}
                  </TabsContent>

                  {/* Code Tab */}
                  <TabsContent value="code" className="space-y-4 mt-6">
                    {/* cURL */}
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <h3 className="text-sm font-semibold">cURL</h3>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() =>
                            copyToClipboard(generateCurl(), "curl")
                          }
                        >
                          {copiedCode === "curl" ? (
                            <CheckCircle className="h-4 w-4" />
                          ) : (
                            <Copy className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                      <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-x-auto text-sm">
                        <code>{generateCurl()}</code>
                      </pre>
                    </div>

                    {/* JavaScript Example */}
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <h3 className="text-sm font-semibold">
                          JavaScript (Fetch)
                        </h3>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() =>
                            copyToClipboard(
                              `fetch('${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${selectedEndpoint.path}', {\n  method: '${selectedEndpoint.method}',\n  headers: {\n    'Content-Type': 'application/json',\n    ${selectedEndpoint.requiresAuth ? `'Authorization': 'Bearer YOUR_TOKEN',\n    ` : ""}\n  },\n  ${selectedEndpoint.method !== "GET" ? `body: JSON.stringify(${requestBody})\n` : ""}})\n.then(res => res.json())\n.then(data => console.log(data));`,
                              "js"
                            )
                          }
                        >
                          {copiedCode === "js" ? (
                            <CheckCircle className="h-4 w-4" />
                          ) : (
                            <Copy className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                      <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-x-auto text-sm">
                        <code>{`fetch('${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${selectedEndpoint.path}', {
  method: '${selectedEndpoint.method}',
  headers: {
    'Content-Type': 'application/json',
    ${selectedEndpoint.requiresAuth ? `'Authorization': 'Bearer YOUR_TOKEN',\n    ` : ""}\n  },
  ${selectedEndpoint.method !== "GET" ? `body: JSON.stringify(${requestBody})\n` : ""}})\n.then(res => res.json())\n.then(data => console.log(data));`}</code>
                      </pre>
                    </div>

                    {/* Python Example */}
                    <div>
                      <div className="flex items-center justify-between mb-2">
                        <h3 className="text-sm font-semibold">
                          Python (requests)
                        </h3>
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() =>
                            copyToClipboard(
                              `import requests\n\nurl = '${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${selectedEndpoint.path}'\nheaders = {\n    'Content-Type': 'application/json',\n    ${selectedEndpoint.requiresAuth ? `'Authorization': 'Bearer YOUR_TOKEN'\n` : ""}\n}\n${selectedEndpoint.method !== "GET" ? `data = ${requestBody}\n\nresponse = requests.${selectedEndpoint.method.toLowerCase()}(url, headers=headers, json=data)` : `response = requests.${selectedEndpoint.method.toLowerCase()}(url, headers=headers)`}\nprint(response.json())`,
                              "python"
                            )
                          }
                        >
                          {copiedCode === "python" ? (
                            <CheckCircle className="h-4 w-4" />
                          ) : (
                            <Copy className="h-4 w-4" />
                          )}
                        </Button>
                      </div>
                      <pre className="p-4 bg-gray-900 text-gray-100 rounded-lg overflow-x-auto text-sm">
                        <code>{`import requests

url = '${process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080"}${selectedEndpoint.path}'
headers = {
    'Content-Type': 'application/json',
    ${selectedEndpoint.requiresAuth ? `'Authorization': 'Bearer YOUR_TOKEN'\n` : ""}\n}
${selectedEndpoint.method !== "GET" ? `data = ${requestBody}\n\nresponse = requests.${selectedEndpoint.method.toLowerCase()}(url, headers=headers, json=data)` : `response = requests.${selectedEndpoint.method.toLowerCase()}(url, headers=headers)`}
print(response.json())`}</code>
                      </pre>
                    </div>
                  </TabsContent>
                </Tabs>
              </CardContent>
            </Card>
          ) : (
            <Card className="h-full flex items-center justify-center min-h-[600px]">
              <div className="text-center text-gray-500">
                <BookOpen className="h-16 w-16 mx-auto mb-4 opacity-20" />
                <p>Select an endpoint to view documentation</p>
              </div>
            </Card>
          )}
        </div>
      </div>
    </div>
  );
}

export default function DevelopersPage() {
  return (
    <Suspense fallback={<DevelopersPageSkeleton />}>
      <DevelopersPageContent />
    </Suspense>
  );
}
