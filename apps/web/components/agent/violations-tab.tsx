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
  attempted_capability: string;
  severity: 'critical' | 'high' | 'medium' | 'low' | 'warning' | 'info'; // Support legacy values
  trust_score_impact: number;
  is_blocked: boolean;
  source_ip?: string;
  created_at: string;
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
        v.created_at,
        v.attempted_capability,
        v.severity,
        v.trust_score_impact.toString(),
        v.is_blocked ? 'Yes' : 'No',
        v.source_ip || 'N/A'
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
                  {formatDistanceToNow(new Date(violation.created_at), { addSuffix: true })}
                </TableCell>
                <TableCell className="font-mono text-sm">
                  {violation.attempted_capability}
                </TableCell>
                <TableCell>
                  <ViolationSeverityBadge severity={violation.severity} />
                </TableCell>
                <TableCell className="text-sm">
                  <span className={violation.trust_score_impact < 0 ? 'text-red-600' : 'text-green-600'}>
                    {violation.trust_score_impact > 0 ? '+' : ''}{violation.trust_score_impact}
                  </span>
                </TableCell>
                <TableCell>
                  <Badge variant={violation.is_blocked ? 'destructive' : 'secondary'}>
                    {violation.is_blocked ? 'Blocked' : 'Allowed'}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-xs">
                  {violation.source_ip || 'N/A'}
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
