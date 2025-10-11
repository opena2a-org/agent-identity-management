'use client'

import { useEffect, useState } from 'react'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Search, Check, X, Clock, Shield, AlertCircle, CheckCircle2, XCircle } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/date-utils'

interface CapabilityRequest {
  id: string
  agent_id: string
  agent_name: string
  agent_display_name: string
  capability_type: string
  reason: string
  status: 'pending' | 'approved' | 'rejected'
  requested_by: string
  requested_by_email: string
  reviewed_by?: string
  reviewed_by_email?: string
  requested_at: string
  reviewed_at?: string
}

const statusColors = {
  pending: 'bg-yellow-100 text-yellow-800 border-yellow-200',
  approved: 'bg-green-100 text-green-800 border-green-200',
  rejected: 'bg-red-100 text-red-800 border-red-200'
}

const statusIcons = {
  pending: Clock,
  approved: CheckCircle2,
  rejected: XCircle
}

export default function CapabilityRequestsPage() {
  const [requests, setRequests] = useState<CapabilityRequest[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [filterStatus, setFilterStatus] = useState<string>('all')

  useEffect(() => {
    fetchRequests()
  }, [])

  const fetchRequests = async () => {
    try {
      // TODO: Replace with actual API endpoint when backend is ready
      // const data = await api.getCapabilityRequests()

      // Mock data for now (will be replaced with real API)
      const mockData: CapabilityRequest[] = []

      setRequests(mockData)
    } catch (error) {
      console.error('Failed to fetch capability requests:', error)
    } finally {
      setLoading(false)
    }
  }

  const approveRequest = async (requestId: string) => {
    try {
      // TODO: Replace with actual API endpoint
      // await api.approveCapabilityRequest(requestId)

      // Update local state
      setRequests(requests.map(r =>
        r.id === requestId ? { ...r, status: 'approved' as const } : r
      ))

      alert('Capability request approved successfully!')
    } catch (error) {
      console.error('Failed to approve request:', error)
      alert('Failed to approve capability request')
    }
  }

  const rejectRequest = async (requestId: string) => {
    if (!confirm('Are you sure you want to reject this capability request?')) {
      return
    }

    try {
      // TODO: Replace with actual API endpoint
      // await api.rejectCapabilityRequest(requestId)

      // Update local state
      setRequests(requests.map(r =>
        r.id === requestId ? { ...r, status: 'rejected' as const } : r
      ))

      alert('Capability request rejected')
    } catch (error) {
      console.error('Failed to reject request:', error)
      alert('Failed to reject capability request')
    }
  }

  const filteredRequests = requests.filter(request => {
    const matchesSearch =
      request.agent_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      request.agent_display_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
      request.capability_type.toLowerCase().includes(searchQuery.toLowerCase()) ||
      request.requested_by_email.toLowerCase().includes(searchQuery.toLowerCase())

    const matchesStatus = filterStatus === 'all' || request.status === filterStatus

    return matchesSearch && matchesStatus
  })

  const pendingCount = requests.filter(r => r.status === 'pending').length
  const approvedCount = requests.filter(r => r.status === 'approved').length
  const rejectedCount = requests.filter(r => r.status === 'rejected').length

  if (loading) {
    return <div className="flex items-center justify-center h-96">Loading capability requests...</div>
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-3xl font-bold">Capability Requests</h1>
        <p className="text-muted-foreground mt-1">
          Review and approve agent capability expansion requests
        </p>
        <div className="mt-3 p-3 bg-blue-50 dark:bg-blue-950/20 border border-blue-200 dark:border-blue-800 rounded-md">
          <p className="text-sm text-blue-900 dark:text-blue-200">
            <strong>Auto-Grant Architecture:</strong> Initial capabilities are automatically granted during agent registration.
            This page handles requests for <strong>additional capabilities</strong> after registration.
          </p>
        </div>
      </div>

      {/* Stats */}
      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Total Requests</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{requests.length}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Pending Review</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-yellow-600">{pendingCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Approved</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-green-600">{approvedCount}</div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium">Rejected</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-red-600">{rejectedCount}</div>
          </CardContent>
        </Card>
      </div>

      {/* Filters */}
      <Card>
        <CardHeader>
          <CardTitle>Search and Filter</CardTitle>
        </CardHeader>
        <CardContent className="flex gap-4">
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              placeholder="Search by agent name or capability..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="pl-10"
            />
          </div>
          <Select value={filterStatus} onValueChange={setFilterStatus}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Filter by status" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All Statuses</SelectItem>
              <SelectItem value="pending">Pending</SelectItem>
              <SelectItem value="approved">Approved</SelectItem>
              <SelectItem value="rejected">Rejected</SelectItem>
            </SelectContent>
          </Select>
        </CardContent>
      </Card>

      {/* Requests List */}
      <Card>
        <CardHeader>
          <CardTitle>Capability Requests ({filteredRequests.length})</CardTitle>
          <CardDescription>
            Review agent capability expansion requests and approve or reject them
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-4">
            {filteredRequests.map((request) => {
              const StatusIcon = statusIcons[request.status]
              const isPending = request.status === 'pending'

              return (
                <div
                  key={request.id}
                  className="flex items-center justify-between p-4 border rounded-lg hover:bg-accent/50 transition-colors"
                >
                  <div className="flex items-center gap-4 flex-1">
                    <div className="h-10 w-10 rounded-full bg-gradient-to-br from-purple-500 to-pink-600 flex items-center justify-center text-white">
                      <Shield className="h-5 w-5" />
                    </div>

                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <p className="font-medium">{request.agent_display_name}</p>
                        <Badge variant="outline" className="text-xs">
                          {request.agent_name}
                        </Badge>
                        <Badge className={`text-xs ${statusColors[request.status]}`}>
                          <StatusIcon className="h-3 w-3 mr-1" />
                          {request.status.charAt(0).toUpperCase() + request.status.slice(1)}
                        </Badge>
                      </div>
                      <div className="flex items-center gap-2 mt-1">
                        <span className="text-sm font-mono bg-muted px-2 py-0.5 rounded">
                          {request.capability_type}
                        </span>
                      </div>
                      <p className="text-sm text-muted-foreground mt-1">
                        <strong>Reason:</strong> {request.reason}
                      </p>
                      <p className="text-xs text-muted-foreground mt-1">
                        Requested by {request.requested_by_email} • {formatDate(request.requested_at)}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    {request.reviewed_at && !isPending && (
                      <div className="text-right text-xs text-muted-foreground">
                        <p>Reviewed by</p>
                        <p className="font-medium">{request.reviewed_by_email}</p>
                        <p>{formatDate(request.reviewed_at)}</p>
                      </div>
                    )}

                    {isPending && (
                      <div className="flex gap-2">
                        <Button
                          size="sm"
                          variant="default"
                          onClick={() => approveRequest(request.id)}
                          className="bg-green-600 hover:bg-green-700"
                        >
                          <Check className="h-4 w-4 mr-1" />
                          Approve
                        </Button>
                        <Button
                          size="sm"
                          variant="destructive"
                          onClick={() => rejectRequest(request.id)}
                        >
                          <X className="h-4 w-4 mr-1" />
                          Reject
                        </Button>
                      </div>
                    )}
                  </div>
                </div>
              )
            })}

            {filteredRequests.length === 0 && (
              <div className="text-center py-12 text-muted-foreground">
                <AlertCircle className="h-12 w-12 mx-auto mb-4 opacity-50" />
                <p className="text-lg font-medium">No capability requests found</p>
                <p className="text-sm mt-1">
                  {searchQuery || filterStatus !== 'all'
                    ? 'Try adjusting your search or filter criteria'
                    : 'Capability requests will appear here when agents request additional permissions'}
                </p>
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
