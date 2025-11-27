'use client';

import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Copy, Check, Key, Calendar, RotateCw, Loader2 } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { api } from '@/lib/api';
import { formatDistanceToNow, differenceInDays } from 'date-fns';

interface KeyVault {
  agent_id: string;
  public_key: string;
  key_algorithm: string;
  key_created_at: string;
  key_expires_at: string;
  rotation_count: number;
  has_previous_public_key: boolean;
}

interface KeyVaultTabProps {
  agentId: string;
}

export function KeyVaultTab({ agentId }: KeyVaultTabProps) {
  const [keyVault, setKeyVault] = useState<KeyVault | null>(null);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);
  const [rotating, setRotating] = useState(false);
  const [showRotateConfirm, setShowRotateConfirm] = useState(false);
  const [newApiKey, setNewApiKey] = useState<string | null>(null);
  const [showNewKeyDialog, setShowNewKeyDialog] = useState(false);

  useEffect(() => {
    const fetchKeyVault = async () => {
      setLoading(true);
      try {
        const data = await api.getAgentKeyVault(agentId);
        setKeyVault(data);
      } catch (error) {
        console.error('Failed to fetch key vault:', error);
      } finally {
        setLoading(false);
      }
    };

    fetchKeyVault();
  }, [agentId]);

  const copyPublicKey = () => {
    if (keyVault) {
      navigator.clipboard.writeText(keyVault.public_key);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  const handleRotateCredentials = async () => {
    setRotating(true);
    try {
      const response = await api.rotateAgentCredentials(agentId);
      setNewApiKey(response.apiKey);
      setShowNewKeyDialog(true);
      setShowRotateConfirm(false);

      // Refresh key vault data
      const data = await api.getAgentKeyVault(agentId);
      setKeyVault(data);
    } catch (error: any) {
      alert(error?.message || 'Failed to rotate credentials');
    } finally {
      setRotating(false);
    }
  };

  const copyNewApiKey = () => {
    if (newApiKey) {
      navigator.clipboard.writeText(newApiKey);
      alert('New API key copied to clipboard!');
    }
  };

  if (loading) {
    return <div className="text-center py-8">Loading key vault...</div>;
  }

  if (!keyVault) {
    return <div className="text-center py-8 text-muted-foreground">Key vault not found</div>;
  }

  const expirationDate = keyVault.key_expires_at ? new Date(keyVault.key_expires_at) : null;
  const isValidDate = expirationDate && expirationDate.getTime() > 0;
  const daysUntilExpiration = isValidDate ? differenceInDays(expirationDate, new Date()) : null;
  const isExpiringSoon = daysUntilExpiration !== null && daysUntilExpiration <= 30 && daysUntilExpiration > 0;

  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-2">
            <Key className="h-5 w-5" />
            <h3 className="text-lg font-semibold">Cryptographic Key Vault</h3>
          </div>
          <Button
            variant="outline"
            onClick={() => setShowRotateConfirm(true)}
            disabled={rotating}
            className="border-blue-500 text-blue-600 hover:bg-blue-50"
          >
            {rotating ? (
              <>
                <Loader2 className="h-4 w-4 mr-1 animate-spin" />
                Rotating...
              </>
            ) : (
              <>
                <RotateCw className="h-4 w-4 mr-1" />
                Rotate Credentials
              </>
            )}
          </Button>
        </div>

        <div className="space-y-6">
          {/* Public Key */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Public Key
            </label>
            <div className="flex gap-2">
              <code className="flex-1 p-3 bg-muted rounded-md text-xs font-mono break-all">
                {keyVault.public_key}
              </code>
              <Button
                variant="outline"
                size="sm"
                onClick={copyPublicKey}
                className="shrink-0"
              >
                {copied ? (
                  <Check className="h-4 w-4" />
                ) : (
                  <Copy className="h-4 w-4" />
                )}
              </Button>
            </div>
          </div>

          {/* Algorithm */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Algorithm
            </label>
            <div className="text-sm font-mono">{keyVault.key_algorithm}</div>
          </div>

          {/* Expiration */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              <Calendar className="inline h-4 w-4 mr-1" />
              Expiration
            </label>
            {isValidDate ? (
              <>
                <div className="flex items-center gap-2">
                  <div className="text-sm">
                    {expirationDate!.toLocaleDateString('en-US', {
                      year: 'numeric',
                      month: 'long',
                      day: 'numeric'
                    })}
                  </div>
                  {daysUntilExpiration !== null && (
                    <div className={`text-sm ${isExpiringSoon ? 'text-red-600 font-semibold' : 'text-muted-foreground'}`}>
                      ({daysUntilExpiration} days remaining)
                    </div>
                  )}
                </div>
                {isExpiringSoon && (
                  <div className="mt-2 text-sm text-red-600">
                    ⚠️ Key expires soon! Consider rotating credentials.
                  </div>
                )}
              </>
            ) : (
              <div className="text-sm text-muted-foreground">
                Never expires
              </div>
            )}
          </div>

          {/* Created At */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Created
            </label>
            <div className="text-sm">
              {(() => {
                const createdDate = keyVault.key_created_at ? new Date(keyVault.key_created_at) : null;
                const isValidCreatedDate = createdDate && createdDate.getTime() > 0;
                return isValidCreatedDate
                  ? formatDistanceToNow(createdDate, { addSuffix: true })
                  : 'Unknown';
              })()}
            </div>
          </div>

          {/* Rotation History */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              <RotateCw className="inline h-4 w-4 mr-1" />
              Rotation History
            </label>
            <div className="text-sm">
              Rotated {keyVault.rotation_count} time{keyVault.rotation_count !== 1 ? 's' : ''}
            </div>
            {keyVault.has_previous_public_key && (
              <div className="text-xs text-muted-foreground mt-1">
                Previous key still valid during grace period
              </div>
            )}
          </div>
        </div>
      </Card>

      {/* Rotate Confirmation Dialog */}
      <AlertDialog open={showRotateConfirm} onOpenChange={setShowRotateConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Rotate Credentials</AlertDialogTitle>
            <AlertDialogDescription>
              This will generate a new API key and cryptographic key pair for this agent.
              The previous key will remain valid for a grace period to prevent service disruption.
              Make sure to update your agent's configuration with the new credentials.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={handleRotateCredentials}
              className="bg-blue-600 hover:bg-blue-700"
            >
              {rotating ? "Rotating..." : "Rotate"}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* New API Key Dialog */}
      <AlertDialog open={showNewKeyDialog} onOpenChange={setShowNewKeyDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>New API Key Generated</AlertDialogTitle>
            <AlertDialogDescription>
              Your new API key has been generated successfully. <strong>This is the only time you'll see the full key.</strong> Copy it now and store it securely.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <div className="my-4">
            <code className="block p-3 bg-muted rounded-md text-xs font-mono break-all">
              {newApiKey}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={copyNewApiKey}
              className="mt-2 w-full"
            >
              <Copy className="h-4 w-4 mr-2" />
              Copy API Key
            </Button>
          </div>
          <AlertDialogFooter>
            <AlertDialogAction onClick={() => {
              setShowNewKeyDialog(false);
              setNewApiKey(null);
            }}>
              Done
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}
