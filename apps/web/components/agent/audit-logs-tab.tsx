'use client';

import { useState, useEffect } from 'react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { FileText, Clock, User, AlertCircle, RefreshCw, MapPin } from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface AuditLog {
  id: string;
  action: string;
  performedBy: string;
  performedByEmail: string;
  timestamp: string;
  details: string;
  ipAddress?: string;
}

interface AuditLogsTabProps {
  agentId: string;
  defaultLimit?: number;
}

export function AuditLogsTab({ agentId, defaultLimit = 50 }: AuditLogsTabProps) {
  const [logs, setLogs] = useState<AuditLog[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [limit, setLimit] = useState(defaultLimit);

  const fetchAuditLogs = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getAgentAuditLogs(agentId, limit);
      // Transform backend response to match frontend interface
      const transformedLogs = (data.logs || []).map((log: any) => ({
        id: log.id,
        action: log.action,
        performedBy: log.userId || log.performedBy || 'system',
        performedByEmail: log.userEmail || log.performedByEmail || '',
        timestamp: log.createdAt || log.timestamp,
        details: log.details,
        ipAddress: log.ipAddress,
      }));
      setLogs(transformedLogs);
      setTotal(data.total);
    } catch (err: any) {
      console.error('Failed to fetch audit logs:', err);
      setError(err.message || 'Failed to load audit logs');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchAuditLogs();
  }, [agentId, limit]);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchAuditLogs();
  };

  const getActionBadge = (action: string) => {
    const variants: Record<string, string> = {
      created: 'bg-green-100 text-green-800 border-green-200',
      updated: 'bg-blue-100 text-blue-800 border-blue-200',
      deleted: 'bg-red-100 text-red-800 border-red-200',
      verified: 'bg-purple-100 text-purple-800 border-purple-200',
      suspended: 'bg-orange-100 text-orange-800 border-orange-200',
      reactivated: 'bg-green-100 text-green-800 border-green-200',
      credentials_rotated: 'bg-yellow-100 text-yellow-800 border-yellow-200',
      trust_score_adjusted: 'bg-blue-100 text-blue-800 border-blue-200',
    };

    return (
      <Badge
        variant="outline"
        className={variants[action] || 'bg-gray-100 text-gray-800 border-gray-200'}
      >
        {action.replace(/_/g, ' ')}
      </Badge>
    );
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Audit Logs</CardTitle>
          <CardDescription>Loading audit history...</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-16" />
          ))}
        </CardContent>
      </Card>
    );
  }

  if (error) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Audit Logs</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="text-center py-8 text-muted-foreground">
            <AlertCircle className="h-12 w-12 mx-auto mb-3 text-yellow-600" />
            <p>{error}</p>
            <Button onClick={handleRefresh} className="mt-4" variant="outline">
              <RefreshCw className="h-4 w-4 mr-2" />
              Try Again
            </Button>
          </div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <div className="flex items-center justify-between">
          <div>
            <CardTitle className="flex items-center gap-2">
              <FileText className="h-5 w-5" />
              Audit Logs
            </CardTitle>
            <CardDescription>
              Complete history of all actions performed on this agent
            </CardDescription>
          </div>
          <Button variant="outline" size="sm" onClick={handleRefresh} disabled={refreshing}>
            {refreshing ? (
              <>
                <RefreshCw className="h-4 w-4 mr-2 animate-spin" />
                Refreshing...
              </>
            ) : (
              <>
                <RefreshCw className="h-4 w-4 mr-2" />
                Refresh
              </>
            )}
          </Button>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        {/* Summary */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <Card className="bg-gray-50">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Total Actions</div>
              <div className="text-2xl font-bold">{total}</div>
            </CardContent>
          </Card>

          <Card className="bg-blue-50 border-blue-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Shown</div>
              <div className="text-2xl font-bold text-blue-600">{logs.length}</div>
            </CardContent>
          </Card>

          <Card className="bg-green-50 border-green-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Latest Action</div>
              <div className="text-sm font-semibold text-green-600">
                {logs.length > 0
                  ? formatDistanceToNow(new Date(logs[0].timestamp), { addSuffix: true })
                  : 'N/A'}
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Logs Table */}
        {logs.length === 0 ? (
          <div className="text-center py-12">
            <FileText className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <p className="text-muted-foreground">No audit logs found</p>
            <p className="text-sm text-muted-foreground mt-1">
              Actions performed on this agent will appear here
            </p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Action</TableHead>
                <TableHead>Performed By</TableHead>
                <TableHead>Details</TableHead>
                <TableHead>IP Address</TableHead>
                <TableHead>Timestamp</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell>{getActionBadge(log.action)}</TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <User className="h-4 w-4 text-muted-foreground" />
                      <div>
                        <div className="font-medium text-sm">{log.performedBy}</div>
                        <div className="text-xs text-muted-foreground">{log.performedByEmail}</div>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell>
                    <p className="text-sm text-muted-foreground max-w-md">{log.details}</p>
                  </TableCell>
                  <TableCell>
                    {log.ipAddress ? (
                      <div className="flex items-center gap-1 text-sm">
                        <MapPin className="h-3 w-3 text-muted-foreground" />
                        <code className="text-xs bg-muted px-2 py-1 rounded">{log.ipAddress}</code>
                      </div>
                    ) : (
                      <span className="text-sm text-muted-foreground">N/A</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2 text-sm">
                      <Clock className="h-4 w-4 text-muted-foreground" />
                      <span>{formatDistanceToNow(new Date(log.timestamp), { addSuffix: true })}</span>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* Load More */}
        {logs.length > 0 && total > limit && (
          <div className="text-center">
            <Button variant="outline" onClick={() => setLimit(limit + 50)}>
              Load More (showing {logs.length} of {total})
            </Button>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
