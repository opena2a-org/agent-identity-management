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
import { Users, Clock, Calendar, AlertCircle, RefreshCw } from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface User {
  id: string;
  email: string;
  name: string;
  role: string;
  lastLogin: string;
  createdAt: string;
  status: string;
}

export function AccessReview() {
  const [users, setUsers] = useState<User[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);

  const fetchAccessReview = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.getAccessReview();
      setUsers(data.users);
      setTotal(data.total);
    } catch (err: any) {
      console.error('Failed to fetch access review:', err);
      setError(err.message || 'Failed to load access review');
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchAccessReview();
  }, []);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchAccessReview();
  };

  const getRoleBadge = (role: string) => {
    const variants = {
      admin: 'bg-purple-100 text-purple-800 border-purple-200',
      manager: 'bg-blue-100 text-blue-800 border-blue-200',
      member: 'bg-green-100 text-green-800 border-green-200',
      viewer: 'bg-gray-100 text-gray-800 border-gray-200',
    };

    return (
      <Badge
        variant="outline"
        className={variants[role as keyof typeof variants] || 'bg-gray-100 text-gray-800'}
      >
        {role}
      </Badge>
    );
  };

  const getStatusBadge = (status: string) => {
    const variants = {
      active: 'bg-green-100 text-green-800 border-green-200',
      inactive: 'bg-gray-100 text-gray-800 border-gray-200',
      pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
      suspended: 'bg-red-100 text-red-800 border-red-200',
    };

    return (
      <Badge
        variant="outline"
        className={variants[status as keyof typeof variants] || 'bg-gray-100 text-gray-800'}
      >
        {status}
      </Badge>
    );
  };

  const getActivityStatus = (lastLogin: string) => {
    const daysSinceLogin = Math.floor(
      (Date.now() - new Date(lastLogin).getTime()) / (1000 * 60 * 60 * 24)
    );

    if (daysSinceLogin > 90) {
      return { color: 'text-red-600', label: 'Inactive (90+ days)' };
    } else if (daysSinceLogin > 30) {
      return { color: 'text-yellow-600', label: 'Low activity (30+ days)' };
    } else if (daysSinceLogin > 7) {
      return { color: 'text-blue-600', label: 'Active' };
    } else {
      return { color: 'text-green-600', label: 'Recently active' };
    }
  };

  if (loading) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>Access Review</CardTitle>
          <CardDescription>Loading user access data...</CardDescription>
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
          <CardTitle>Access Review</CardTitle>
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
              <Users className="h-5 w-5" />
              Access Review
            </CardTitle>
            <CardDescription>
              Review user access patterns and identify inactive or risky accounts
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
        {/* Summary Stats */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          <Card className="bg-gray-50">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Total Users</div>
              <div className="text-2xl font-bold">{total}</div>
            </CardContent>
          </Card>

          <Card className="bg-green-50 border-green-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Active</div>
              <div className="text-2xl font-bold text-green-600">
                {users.filter((u) => u.status === 'active').length}
              </div>
            </CardContent>
          </Card>

          <Card className="bg-yellow-50 border-yellow-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Inactive (30+ days)</div>
              <div className="text-2xl font-bold text-yellow-600">
                {
                  users.filter(
                    (u) =>
                      Math.floor(
                        (Date.now() - new Date(u.lastLogin).getTime()) / (1000 * 60 * 60 * 24)
                      ) > 30
                  ).length
                }
              </div>
            </CardContent>
          </Card>

          <Card className="bg-red-50 border-red-200">
            <CardContent className="pt-6">
              <div className="text-sm text-muted-foreground mb-1">Inactive (90+ days)</div>
              <div className="text-2xl font-bold text-red-600">
                {
                  users.filter(
                    (u) =>
                      Math.floor(
                        (Date.now() - new Date(u.lastLogin).getTime()) / (1000 * 60 * 60 * 24)
                      ) > 90
                  ).length
                }
              </div>
            </CardContent>
          </Card>
        </div>

        {/* Users Table */}
        {users.length === 0 ? (
          <div className="text-center py-12">
            <Users className="h-16 w-16 mx-auto mb-4 text-muted-foreground" />
            <p className="text-muted-foreground">No users found</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>User</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Last Login</TableHead>
                <TableHead>Activity Status</TableHead>
                <TableHead>Account Created</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {users.map((user) => {
                const activityStatus = getActivityStatus(user.lastLogin);
                return (
                  <TableRow key={user.id}>
                    <TableCell>
                      <div>
                        <div className="font-medium">{user.name}</div>
                        <div className="text-sm text-muted-foreground">{user.email}</div>
                      </div>
                    </TableCell>
                    <TableCell>{getRoleBadge(user.role)}</TableCell>
                    <TableCell>{getStatusBadge(user.status)}</TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2 text-sm">
                        <Clock className="h-4 w-4 text-muted-foreground" />
                        <span>{formatDistanceToNow(new Date(user.lastLogin), { addSuffix: true })}</span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <span className={`text-sm font-medium ${activityStatus.color}`}>
                        {activityStatus.label}
                      </span>
                    </TableCell>
                    <TableCell>
                      <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        <Calendar className="h-4 w-4" />
                        <span>{new Date(user.createdAt).toLocaleDateString()}</span>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}
