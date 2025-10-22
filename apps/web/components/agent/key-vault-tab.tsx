'use client';

import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Copy, Check, Key, Calendar, RotateCw } from 'lucide-react';
import { PremiumUpsellBanner } from './premium-upsell-banner';
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

  if (loading) {
    return <div className="text-center py-8">Loading key vault...</div>;
  }

  if (!keyVault) {
    return <div className="text-center py-8 text-muted-foreground">Key vault not found</div>;
  }

  const daysUntilExpiration = differenceInDays(new Date(keyVault.key_expires_at), new Date());
  const isExpiringSoon = daysUntilExpiration <= 30;

  return (
    <div className="space-y-6">
      <Card className="p-6">
        <div className="flex items-center gap-2 mb-6">
          <Key className="h-5 w-5" />
          <h3 className="text-lg font-semibold">Cryptographic Key Vault</h3>
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
            <div className="flex items-center gap-2">
              <div className="text-sm">
                {new Date(keyVault.key_expires_at).toLocaleDateString('en-US', {
                  year: 'numeric',
                  month: 'long',
                  day: 'numeric'
                })}
              </div>
              <div className={`text-sm ${isExpiringSoon ? 'text-red-600 font-semibold' : 'text-muted-foreground'}`}>
                ({daysUntilExpiration} days remaining)
              </div>
            </div>
            {isExpiringSoon && (
              <div className="mt-2 text-sm text-red-600">
                ⚠️ Key expires soon! Consider rotating credentials.
              </div>
            )}
          </div>

          {/* Created At */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Created
            </label>
            <div className="text-sm">
              {formatDistanceToNow(new Date(keyVault.key_created_at), { addSuffix: true })}
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

      {/* Premium Upsell */}
      <PremiumUpsellBanner />
    </div>
  );
}
