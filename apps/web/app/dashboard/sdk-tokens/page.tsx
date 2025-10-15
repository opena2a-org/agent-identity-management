"use client";

import { useEffect, useState } from "react";
import { api, SDKToken } from "@/lib/api";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import {
  AlertCircle,
  Download,
  Key,
  Monitor,
  Trash2,
  Shield,
  Clock,
  MapPin,
} from "lucide-react";
import { formatDistanceToNow } from "date-fns";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { SDKTokensPageSkeleton } from "@/components/ui/content-loaders";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";

export default function SDKTokensPage() {
  const [tokens, setTokens] = useState<SDKToken[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [includeRevoked, setIncludeRevoked] = useState(false);
  const [selectedToken, setSelectedToken] = useState<SDKToken | null>(null);
  const [showRevokeDialog, setShowRevokeDialog] = useState(false);
  const [showRevokeAllDialog, setShowRevokeAllDialog] = useState(false);
  const [revokeReason, setRevokeReason] = useState("");
  const [revoking, setRevoking] = useState(false);

  useEffect(() => {
    loadTokens();
  }, [includeRevoked]);

  const loadTokens = async () => {
    try {
      setLoading(true);
      // Always fetch ALL tokens (including revoked) for accurate stats
      // We'll filter which ones to display based on includeRevoked state
      const response = await api.listSDKTokens(true);
      setTokens(response.tokens || []);
      setError(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to load SDK tokens"
      );
    } finally {
      setLoading(false);
    }
  };

  const handleRevokeToken = async () => {
    if (!selectedToken || !revokeReason.trim()) return;

    try {
      setRevoking(true);
      await api.revokeSDKToken(selectedToken.id, revokeReason);
      setShowRevokeDialog(false);
      setSelectedToken(null);
      setRevokeReason("");
      await loadTokens();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to revoke token");
    } finally {
      setRevoking(false);
    }
  };

  const handleRevokeAll = async () => {
    if (!revokeReason.trim()) return;

    try {
      setRevoking(true);
      await api.revokeAllSDKTokens(revokeReason);
      setShowRevokeAllDialog(false);
      setRevokeReason("");
      await loadTokens();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "Failed to revoke all tokens"
      );
    } finally {
      setRevoking(false);
    }
  };

  const activeTokens = tokens.filter((t) => !t.revokedAt);
  const revokedTokens = tokens.filter((t) => t.revokedAt);

  const isTokenExpired = (token: SDKToken) => {
    return new Date(token.expiresAt) < new Date();
  };

  const getTokenStatus = (token: SDKToken) => {
    if (token.revokedAt)
      return { label: "Revoked", color: "destructive" as const };
    if (isTokenExpired(token))
      return { label: "Expired", color: "secondary" as const };
    return { label: "Active", color: "default" as const };
  };

  if (loading) {
    return <SDKTokensPageSkeleton />;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-3xl font-bold tracking-tight">SDK Tokens</h1>
          <p className="text-muted-foreground mt-2">
            Manage your SDK authentication tokens and monitor their usage
          </p>
        </div>
        <div className="flex items-center gap-3">
          <Button
            variant="outline"
            onClick={() => setIncludeRevoked(!includeRevoked)}
          >
            {includeRevoked ? "Hide Revoked" : "Show Revoked"}
          </Button>
          {activeTokens.length > 0 && (
            <Button
              variant="destructive"
              onClick={() => setShowRevokeAllDialog(true)}
            >
              <Trash2 className="w-4 h-4 mr-2" />
              Revoke All
            </Button>
          )}
        </div>
      </div>

      {/* Error Alert */}
      {error && (
        <Alert variant="destructive">
          <AlertCircle className="h-4 w-4" />
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {/* Stats Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Active Tokens</CardTitle>
            <Shield className="h-4 w-4 text-green-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{activeTokens.length}</div>
            <p className="text-xs text-muted-foreground">Currently valid</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">Total Usage</CardTitle>
            <Key className="h-4 w-4 text-blue-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {tokens.reduce((sum, t) => sum + t.usageCount, 0)}
            </div>
            <p className="text-xs text-muted-foreground">API requests</p>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              Revoked Tokens
            </CardTitle>
            <Trash2 className="h-4 w-4 text-red-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{revokedTokens.length}</div>
            <p className="text-xs text-muted-foreground">No longer valid</p>
          </CardContent>
        </Card>
      </div>

      {/* Tokens List */}
      {tokens.length === 0 ? (
        <Card>
          <CardContent className="flex flex-col items-center justify-center py-12">
            <Key className="h-12 w-12 text-muted-foreground mb-4" />
            <p className="text-lg font-medium mb-2">No SDK tokens found</p>
            <p className="text-sm text-muted-foreground text-center max-w-md mb-4">
              Download the SDK to automatically generate an authentication token
            </p>
            <Button onClick={() => (window.location.href = "/dashboard/sdk")}>
              <Download className="w-4 h-4 mr-2" />
              Download SDK
            </Button>
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {(includeRevoked ? tokens : activeTokens).map((token) => {
            const status = getTokenStatus(token);
            return (
              <Card
                key={token.id}
                className={token.revokedAt ? "opacity-60" : ""}
              >
                <CardHeader>
                  <div className="flex items-start justify-between">
                    <div className="space-y-1">
                      <div className="flex items-center gap-2">
                        <CardTitle className="text-lg">
                          {token.deviceName || "Unknown Device"}
                        </CardTitle>
                        <Badge variant={status.color}>{status.label}</Badge>
                      </div>
                      <CardDescription className="font-mono text-xs">
                        Token ID: {token.tokenId}
                      </CardDescription>
                    </div>
                    {!token.revokedAt && !isTokenExpired(token) && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => {
                          setSelectedToken(token);
                          setShowRevokeDialog(true);
                        }}
                      >
                        <Trash2 className="w-4 h-4 mr-2" />
                        Revoke
                      </Button>
                    )}
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
                    {/* IP Address */}
                    <div className="flex items-start gap-2">
                      <MapPin className="w-4 h-4 mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="text-sm font-medium">IP Address</p>
                        <p className="text-sm text-muted-foreground">
                          {token.lastIpAddress || token.ipAddress || "Unknown"}
                        </p>
                      </div>
                    </div>

                    {/* Device */}
                    <div className="flex items-start gap-2">
                      <Monitor className="w-4 h-4 mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="text-sm font-medium">User Agent</p>
                        <p
                          className="text-sm text-muted-foreground truncate max-w-[200px]"
                          title={token.userAgent}
                        >
                          {token.userAgent
                            ? token.userAgent.split(" ")[0]
                            : "Unknown"}
                        </p>
                      </div>
                    </div>

                    {/* Last Used */}
                    <div className="flex items-start gap-2">
                      <Clock className="w-4 h-4 mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="text-sm font-medium">Last Used</p>
                        <p className="text-sm text-muted-foreground">
                          {token.lastUsedAt
                            ? formatDistanceToNow(new Date(token.lastUsedAt), {
                                addSuffix: true,
                              })
                            : "Never"}
                        </p>
                      </div>
                    </div>

                    {/* Usage Count */}
                    <div className="flex items-start gap-2">
                      <Key className="w-4 h-4 mt-0.5 text-muted-foreground" />
                      <div>
                        <p className="text-sm font-medium">Usage Count</p>
                        <p className="text-sm text-muted-foreground">
                          {token.usageCount.toLocaleString()} requests
                        </p>
                      </div>
                    </div>
                  </div>

                  {/* Additional Info Row */}
                  <div className="mt-4 pt-4 border-t flex items-center justify-between text-sm">
                    <div className="flex items-center gap-6 text-muted-foreground">
                      <span>
                        Created{" "}
                        {formatDistanceToNow(new Date(token.createdAt), {
                          addSuffix: true,
                        })}
                      </span>
                      <span>
                        Expires{" "}
                        {formatDistanceToNow(new Date(token.expiresAt), {
                          addSuffix: true,
                        })}
                      </span>
                    </div>
                    {token.revokedAt && token.revokeReason && (
                      <div className="text-red-600 text-sm">
                        Revoked: {token.revokeReason}
                      </div>
                    )}
                  </div>
                </CardContent>
              </Card>
            );
          })}
        </div>
      )}

      {/* Revoke Single Token Dialog */}
      <Dialog open={showRevokeDialog} onOpenChange={setShowRevokeDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke SDK Token</DialogTitle>
            <DialogDescription>
              This will immediately invalidate the token. Any applications using
              this token will lose access.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="reason">Reason for revocation</Label>
              <Textarea
                id="reason"
                placeholder="e.g., Device lost, security breach, no longer needed..."
                value={revokeReason}
                onChange={(e) => setRevokeReason(e.target.value)}
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShowRevokeDialog(false)}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRevokeToken}
              disabled={!revokeReason.trim() || revoking}
            >
              {revoking ? "Revoking..." : "Revoke Token"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke All Tokens Dialog */}
      <Dialog open={showRevokeAllDialog} onOpenChange={setShowRevokeAllDialog}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Revoke All SDK Tokens</DialogTitle>
            <DialogDescription>
              This will immediately invalidate all {activeTokens.length} active
              tokens. This action cannot be undone.
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <Alert variant="destructive">
              <AlertCircle className="h-4 w-4" />
              <AlertDescription>
                All applications using SDK tokens will immediately lose access.
                You will need to download the SDK again.
              </AlertDescription>
            </Alert>
            <div className="space-y-2">
              <Label htmlFor="reason-all">Reason for revoking all tokens</Label>
              <Textarea
                id="reason-all"
                placeholder="e.g., Security incident, credential rotation, system compromise..."
                value={revokeReason}
                onChange={(e) => setRevokeReason(e.target.value)}
                rows={3}
              />
            </div>
          </div>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setShowRevokeAllDialog(false)}
            >
              Cancel
            </Button>
            <Button
              variant="destructive"
              onClick={handleRevokeAll}
              disabled={!revokeReason.trim() || revoking}
            >
              {revoking
                ? "Revoking..."
                : `Revoke All ${activeTokens.length} Tokens`}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
