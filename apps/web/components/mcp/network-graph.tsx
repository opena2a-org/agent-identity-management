"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ReactFlow,
  Node,
  Edge,
  Controls,
  Background,
  BackgroundVariant,
  useNodesState,
  useEdgesState,
  Position,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { api } from "@/lib/api";
import { Loader2, AlertCircle, Server, Bot, RefreshCw } from "lucide-react";
import Link from "next/link";

interface GraphNode {
  id: string;
  type: "mcp" | "agent";
  label: string;
  size: number;
  color: string;
  status: string;
  trustScore: number;
}

interface GraphEdge {
  id: string;
  source: string;
  target: string;
  weight: number;
  type: "connection" | "attestation";
}

interface NetworkGraphProps {
  mcpServerId?: string; // If provided, shows connections for specific MCP server
  height?: string;
  showControls?: boolean;
  showLegend?: boolean;
  onNodeClick?: (nodeId: string, nodeType: "mcp" | "agent") => void;
}

// Node data type for type-safe access
interface FlowNodeData {
  label: string;
  color: string;
  status: string;
  trustScore: number;
  nodeType: "mcp" | "agent";
  originalId: string;
}

// Custom node component for MCP servers
function MCPNode({ data }: { data: any }) {
  return (
    <div
      className="flex flex-col items-center justify-center p-3 rounded-panel border-2 shadow-panel bg-glass backdrop-blur-card min-w-[120px] cursor-pointer hover:shadow-chrome transition-shadow"
      style={{ borderColor: data.color }}
    >
      <Server
        className="h-6 w-6 mb-1"
        style={{ color: data.color }}
      />
      <div className="text-xs font-medium text-ink text-center max-w-[100px] truncate">
        {data.label}
      </div>
      <div className="text-[10px] text-ink-secondary">
        {data.trustScore.toFixed(0)}% trust
      </div>
      <div
        className="mt-1 px-2 py-0.5 rounded-pill text-[9px] font-medium"
        style={{
          backgroundColor: `${data.color}20`,
          color: data.color,
        }}
      >
        {data.status}
      </div>
    </div>
  );
}

// Custom node component for Agents
function AgentNode({ data }: { data: any }) {
  return (
    <div
      className="flex flex-col items-center justify-center p-3 rounded-full border-2 shadow-panel bg-glass backdrop-blur-card min-w-[100px] min-h-[100px] cursor-pointer hover:shadow-chrome transition-shadow"
      style={{ borderColor: data.color }}
    >
      <Bot
        className="h-5 w-5 mb-1"
        style={{ color: data.color }}
      />
      <div className="text-xs font-medium text-ink text-center max-w-[80px] truncate">
        {data.label}
      </div>
      <div className="text-[10px] text-ink-secondary">
        {data.trustScore.toFixed(0)}%
      </div>
    </div>
  );
}

const nodeTypes = {
  mcpNode: MCPNode,
  agentNode: AgentNode,
};

export function MCPNetworkGraph({
  mcpServerId,
  height = "400px",
  showControls = true,
  showLegend = true,
  onNodeClick,
}: NetworkGraphProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchGraphData = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);

      const data = mcpServerId
        ? await api.getMCPServerConnections(mcpServerId)
        : await api.getMCPConnectionGraph();

      if (!data.nodes || data.nodes.length === 0) {
        setNodes([]);
        setEdges([]);
        setLoading(false);
        return;
      }

      // Convert API data to React Flow format
      const mcpNodes = data.nodes.filter((n) => n.type === "mcp");
      const agentNodes = data.nodes.filter((n) => n.type === "agent");

      // Layout: MCP servers on left, Agents on right
      const flowNodes: Node[] = [];

      // Position MCP servers
      mcpNodes.forEach((node, index) => {
        flowNodes.push({
          id: node.id,
          type: "mcpNode",
          position: {
            x: 50,
            y: index * 150 + 50,
          },
          data: {
            label: node.label,
            color: node.color,
            status: node.status,
            trustScore: node.trustScore,
            nodeType: "mcp",
            originalId: node.id.replace("mcp-", ""),
          },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
        });
      });

      // Position Agents
      agentNodes.forEach((node, index) => {
        flowNodes.push({
          id: node.id,
          type: "agentNode",
          position: {
            x: 400,
            y: index * 130 + 50,
          },
          data: {
            label: node.label,
            color: node.color,
            status: node.status,
            trustScore: node.trustScore,
            nodeType: "agent",
            originalId: node.id.replace("agent-", ""),
          },
          sourcePosition: Position.Left,
          targetPosition: Position.Right,
        });
      });

      // Convert edges
      const flowEdges: Edge[] = data.edges.map((edge) => ({
        id: edge.id,
        source: edge.source,
        target: edge.target,
        type: "smoothstep",
        animated: edge.type === "attestation",
        style: {
          stroke: edge.type === "attestation" ? "var(--green)" : "var(--text-tertiary)",
          strokeWidth: edge.weight,
        },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: edge.type === "attestation" ? "var(--green)" : "var(--text-tertiary)",
        },
        label: edge.type === "attestation" ? "attested" : undefined,
        labelStyle: { fontSize: 10, fill: "var(--text-secondary)" },
      }));

      setNodes(flowNodes);
      setEdges(flowEdges);
    } catch (err) {
      console.error("Failed to fetch graph data:", err);
      setError("Failed to load network graph");
    } finally {
      setLoading(false);
    }
  }, [mcpServerId, setNodes, setEdges]);

  useEffect(() => {
    fetchGraphData();
  }, [fetchGraphData]);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      const data = node.data as unknown as FlowNodeData;
      if (onNodeClick && data.nodeType && data.originalId) {
        onNodeClick(data.originalId, data.nodeType);
      }
    },
    [onNodeClick]
  );

  if (loading) {
    return (
      <div
        className="flex items-center justify-center glass rounded-panel"
        style={{ height }}
      >
        <div className="flex flex-col items-center gap-2">
          <Loader2 className="h-8 w-8 animate-spin text-brand-text" />
          <span className="text-sm text-ink-secondary">
            Loading network graph...
          </span>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div
        className="flex items-center justify-center glass-alert rounded-panel"
        style={{ height }}
      >
        <div className="flex flex-col items-center gap-2">
          <AlertCircle className="h-8 w-8 text-danger-text" />
          <span className="text-sm text-ink-secondary">
            {error}
          </span>
          <button
            onClick={fetchGraphData}
            className="mt-2 px-3 py-1 text-sm rounded-pill bg-brand text-white shadow-glow hover:bg-brand-hover flex items-center gap-1"
          >
            <RefreshCw className="h-3 w-3" />
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (nodes.length === 0) {
    return (
      <div
        className="flex items-center justify-center glass rounded-panel"
        style={{ height }}
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <Server className="h-12 w-12 text-ink-tertiary" />
          <p className="text-sm font-medium text-ink-secondary">
            No connections yet
          </p>
          <p className="text-xs text-ink-tertiary max-w-xs">
            Register MCP servers and configure agent connections to see the
            network graph
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="relative" style={{ height }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onNodeClick={handleNodeClick}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        minZoom={0.5}
        maxZoom={2}
        className="bg-glass rounded-panel border border-divider"
      >
        {showControls && <Controls className="!bg-glass !border-glass-border" />}
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} color="var(--stroke)" />
      </ReactFlow>

      {/* Legend */}
      {showLegend && (
        <div className="absolute bottom-4 left-4 glass rounded-panel p-3">
          <div className="text-xs font-medium text-ink mb-2">
            Legend
          </div>
          <div className="space-y-1.5">
            <div className="flex items-center gap-2">
              <div className="w-4 h-4 rounded border-2 border-ink-tertiary flex items-center justify-center">
                <Server className="h-2 w-2 text-ink-tertiary" />
              </div>
              <span className="text-[10px] text-ink-secondary">
                MCP server
              </span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-4 h-4 rounded-full border-2 border-ink-tertiary flex items-center justify-center">
                <Bot className="h-2 w-2 text-ink-tertiary" />
              </div>
              <span className="text-[10px] text-ink-secondary">
                Agent
              </span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-6 h-0.5 bg-ink-tertiary"></div>
              <span className="text-[10px] text-ink-secondary">
                Connection
              </span>
            </div>
            <div className="flex items-center gap-2">
              <div className="w-6 h-0.5 bg-success animate-pulse"></div>
              <span className="text-[10px] text-ink-secondary">
                Attestation
              </span>
            </div>
          </div>
          <div className="mt-2 pt-2 border-t border-divider">
            <div className="text-[10px] text-ink-secondary">
              Trust score colors
            </div>
            {/* These swatches echo the exact scale the API sends as node.color
                (backend trustScoreColor). They are data, not theme chrome:
                tokenising them would stop the legend matching the drawn nodes. */}
            <div className="flex gap-1 mt-1">
              <div className="w-3 h-3 rounded" style={{ backgroundColor: "#22c55e" }} title="≥80%"></div>
              <div className="w-3 h-3 rounded" style={{ backgroundColor: "#84cc16" }} title="60-79%"></div>
              <div className="w-3 h-3 rounded" style={{ backgroundColor: "#eab308" }} title="40-59%"></div>
              <div className="w-3 h-3 rounded" style={{ backgroundColor: "#f97316" }} title="20-39%"></div>
              <div className="w-3 h-3 rounded" style={{ backgroundColor: "#ef4444" }} title="<20%"></div>
            </div>
          </div>
        </div>
      )}

      {/* Refresh button */}
      <button
        onClick={fetchGraphData}
        className="absolute top-4 right-4 p-2 glass rounded-inset hover:bg-glass-inset-gray transition-colors"
        title="Refresh graph"
      >
        <RefreshCw className="h-4 w-4 text-ink-secondary" />
      </button>
    </div>
  );
}
