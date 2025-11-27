'use client';

import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Download, RefreshCw } from 'lucide-react';
import { ViolationSeverityBadge } from './violation-severity-badge';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface Violation {
  id: string;
  attemptedCapability: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'warning' | 'info'; // Support legacy values
  trustScoreImpact: number;
  isBlocked: boolean;
  sourceIp?: string;
  createdAt: string;
}

interface ViolationsTabProps {
  agentId: string;
}

export function ViolationsTab({ agentId }: ViolationsTabProps) {
  const [violations, setViolations] = useState<Violation[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [page, setPage] = useState(1);
  const limit = 10;

  const fetchViolations = async () => {
    setLoading(true);
    try {
      const response = await api.getAgentViolations(agentId, limit, (page - 1) * limit);
      setViolations(response.violations || []);
      setTotal(response.total || 0);
    } catch (error) {
      console.error('Failed to fetch violations:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchViolations();
    // Auto-refresh every 30 seconds
    const interval = setInterval(fetchViolations, 30000);
    return () => clearInterval(interval);
  }, [agentId, page]);

  const exportToCSV = () => {
    const csv = [
      ['Timestamp', 'Capability', 'Severity', 'Trust Impact', 'Blocked', 'Source IP'],
      ...violations.map(v => [
        v.createdAt,
        v.attemptedCapability,
        v.severity,
        v.trustScoreImpact.toString(),
        v.isBlocked ? 'Yes' : 'No',
        v.sourceIp || 'N/A'
      ])
    ].map(row => row.join(',')).join('\n');

    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `violations-${agentId}-${Date.now()}.csv`;
    a.click();
  };

  return (
    <Card className="p-6">
      <div className="flex justify-between items-center mb-6">
        <div>
          <h3 className="text-lg font-semibold">Capability Violations</h3>
          <p className="text-sm text-muted-foreground">
            Total violations: {total}
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" size="sm" onClick={fetchViolations}>
            <RefreshCw className="h-4 w-4 mr-2" />
            Refresh
          </Button>
          <Button variant="outline" size="sm" onClick={exportToCSV} disabled={violations.length === 0}>
            <Download className="h-4 w-4 mr-2" />
            Export CSV
          </Button>
        </div>
      </div>

      {loading ? (
        <div className="text-center py-8">Loading violations...</div>
      ) : violations.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          No violations found. This agent has a clean record!
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Timestamp</TableHead>
              <TableHead>Attempted Capability</TableHead>
              <TableHead>Severity</TableHead>
              <TableHead>Trust Impact</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Source IP</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {violations.map((violation) => (
              <TableRow key={violation.id}>
                <TableCell className="text-sm">
                  {formatDistanceToNow(new Date(violation.createdAt), { addSuffix: true })}
                </TableCell>
                <TableCell className="font-mono text-sm">
                  {violation.attemptedCapability}
                </TableCell>
                <TableCell>
                  <ViolationSeverityBadge severity={violation.severity} />
                </TableCell>
                <TableCell className="text-sm">
                  <span className={violation.trustScoreImpact < 0 ? 'text-red-600' : 'text-green-600'}>
                    {violation.trustScoreImpact > 0 ? '+' : ''}{violation.trustScoreImpact}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge variant={violation.isBlocked ? 'destructive' : 'secondary'}>
                    {violation.isBlocked ? 'Blocked' : 'Allowed'}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {violation.sourceIp || 'N/A'}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {total > limit && (
        <div className="flex justify-center gap-2 mt-4">
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage(p => Math.max(1, p - 1))}
            disabled={page === 1}
          >
            Previous
          </Button>
          <span className="py-2 px-4 text-sm">
            Page {page} of {Math.ceil(total / limit)}
          </span>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setPage(p => p + 1)}
            disabled={page >= Math.ceil(total / limit)}
          >
            Next
          </Button>
        </div>
      )}
    </Card>
  );
}
