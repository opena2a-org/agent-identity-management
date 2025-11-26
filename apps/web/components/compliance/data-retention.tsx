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
import { Database, Calendar, AlertCircle, RefreshCw, CheckCircle, XCircle } from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface Policy {
  id: string;
  dataType: string;
  retentionPeriodDays: number;
  description: string;
  autoDelete: boolean;
  createdAt: string;
}

interface StorageMetrics {
  totalRecords: number;
  oldestRecordDate: string;
  deletionCandidates: number;
}

export function DataRetention() {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [metrics, setMetrics] = useState<StorageMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchDataRetention = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getDataRetention();
      setPolicies(data.policies);
      setMetrics(data.storageMetrics);
    } catch (err: any) {
      console.error('Failed to fetch data retention:', err);
      setError(err.message || 'Failed to load data retention');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchDataRetention();
  }, []);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchDataRetention();
  };

  const getRetentionBadge = (days: number) => {
    if (days <= 30) {
      return <Badge className="bg-red-100 text-red-800 border-red-200">{days} days</Badge>;
    } else if (days <= 180) {
      return <Badge className="bg-yellow-100 text-yellow-800 border-yellow-200">{days} days</Badge>;
    } else if (days <= 365) {
      return <Badge className="bg-blue-100 text-blue-800 border-blue-200">{days} days</Badge>;
    } else {
      return (
        <Badge className="bg-green-100 text-green-800 border-green-200">
          {Math.floor(days / 365)} year{days >= 730 ? 's' : ''}
        </Badge>
      );
    }
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Data Retention</CardTitle>
          <CardDescription>Loading data retention policies...</CardDescription>
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
          <CardTitle>Data Retention</CardTitle>
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
              <Database className="h-5 w-5" />
              Data Retention Policies
            </CardTitle>
            <CardDescription>
              Manage data retention periods and automatic deletion policies
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
      <CardContent className="space-y-6">
        {/* Storage Metrics */}
        {metrics && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <Card className="bg-blue-50 border-blue-200">
              <CardContent className="pt-6">
                <div className="text-sm text-muted-foreground mb-1">Total Records</div>
                <div className="text-2xl font-bold text-blue-600">
                  {metrics.totalRecords.toLocaleString()}
                </div>
              </CardContent>
            </Card>

            <Card className="bg-yellow-50 border-yellow-200">
              <CardContent className="pt-6">
                <div className="text-sm text-muted-foreground mb-1">Oldest Record</div>
                <div className="text-2xl font-bold text-yellow-600">
                  {formatDistanceToNow(new Date(metrics.oldestRecordDate), { addSuffix: true })}
                </div>
              </CardContent>
            </Card>

            <Card className="bg-red-50 border-red-200">
              <CardContent className="pt-6">
                <div className="text-sm text-muted-foreground mb-1">Deletion Candidates</div>
                <div className="text-2xl font-bold text-red-600">
                  {metrics.deletionCandidates.toLocaleString()}
                </div>
              </CardContent>
            </Card>
          </div>
        )}

        {/* Retention Policies Table */}
        {policies.length === 0 ? (
          <div className="text-center py-12">
            <Database className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <p className="text-muted-foreground">No retention policies configured</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Data Type</TableHead>
                <TableHead>Description</TableHead>
                <TableHead>Retention Period</TableHead>
                <TableHead>Auto-Delete</TableHead>
                <TableHead>Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {policies.map((policy) => (
                <TableRow key={policy.id}>
                  <TableCell>
                    <code className="text-sm bg-muted px-2 py-1 rounded font-mono">
                      {policy.dataType}
                    </code>
                  </TableCell>
                  <TableCell>
                    <div className="max-w-md">
                      <p className="text-sm text-muted-foreground">{policy.description}</p>
                    </div>
                  </TableCell>
                  <TableCell>{getRetentionBadge(policy.retentionPeriodDays)}</TableCell>
                  <TableCell>
                    {policy.autoDelete ? (
                      <Badge className="bg-green-100 text-green-800 border-green-200">
                        <CheckCircle className="h-3 w-3 mr-1" />
                        Enabled
                      </Badge>
                    ) : (
                      <Badge className="bg-gray-100 text-gray-800 border-gray-200">
                        <XCircle className="h-3 w-3 mr-1" />
                        Disabled
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                      <Calendar className="h-4 w-4" />
                      <span>{new Date(policy.createdAt).toLocaleDateString()}</span>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}

        {/* Information Box */}
        <div className="p-4 bg-blue-50 border border-blue-200 rounded-lg">
          <h4 className="text-sm font-semibold text-blue-900 mb-2 flex items-center gap-2">
            <AlertCircle className="h-4 w-4" />
            Data Retention Best Practices
          </h4>
          <ul className="text-sm text-blue-800 space-y-1 list-disc list-inside">
            <li>Review and update retention policies quarterly</li>
            <li>Ensure policies comply with GDPR, HIPAA, and SOC 2 requirements</li>
            <li>Enable auto-delete to reduce storage costs and compliance risk</li>
            <li>Regularly audit deletion candidates before automated cleanup</li>
            <li>Maintain audit logs for all data deletions</li>
          </ul>
        </div>
      </CardContent>
    </Card>
  );
}
