'use client';

import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Copy, Check, Key, Shield } from 'lucide-react';
import { api } from '@/lib/api';
import { formatDistanceToNow } from 'date-fns';

interface KeyVault {
  agentId: string;
  publicKey: string;
  keyAlgorithm: string;
  keyCreatedAt: string;
  hasPreviousPublicKey: boolean;
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
    if (keyVault?.publicKey) {
      navigator.clipboard.writeText(keyVault.publicKey);
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

  return (
    <div className="space-y-6">
      {/* Future Use Cases */}
      <Card className="p-4 bg-purple-50 dark:bg-purple-950 border-purple-200 dark:border-purple-800">
        <div className="flex items-start gap-3">
          <Shield className="h-5 w-5 text-purple-600 dark:text-purple-400 mt-0.5" />
          <div className="text-sm text-purple-800 dark:text-purple-200">
            <p className="font-medium mb-2">Future Use Cases</p>
            <ul className="list-disc list-inside space-y-1 text-purple-700 dark:text-purple-300 text-xs">
              <li><strong>Request Signing:</strong> Cryptographically sign sensitive API calls for tamper-proof verification</li>
              <li><strong>Audit Compliance:</strong> Non-repudiation proof for SOC 2, HIPAA, and regulatory requirements</li>
              <li><strong>Zero-Trust Security:</strong> Verify agent identity even in compromised network environments</li>
            </ul>
          </div>
        </div>
      </Card>

      <Card className="p-6">
        <div className="flex items-center gap-2 mb-6">
          <Key className="h-5 w-5" />
          <h3 className="text-lg font-semibold">Agent Public Key</h3>
          <span className="ml-auto px-2 py-1 text-xs bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300 rounded-full flex items-center gap-1">
            <Shield className="h-3 w-3" />
            Server-side (read-only)
          </span>
        </div>

        <div className="space-y-6">
          {/* Public Key */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Public Key (Base64)
            </label>
            <div className="flex gap-2">
              <code className="flex-1 p-3 bg-muted rounded-md text-xs font-mono break-all">
                {keyVault.publicKey || 'Not set'}
              </code>
              {keyVault.publicKey && (
                <Button
                  type="button"
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
              )}
            </div>
          </div>

          {/* Algorithm */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Algorithm
            </label>
            <div className="text-sm font-mono">{keyVault.keyAlgorithm || 'Ed25519'}</div>
          </div>

          {/* Key Created At */}
          <div>
            <label className="text-sm font-medium text-muted-foreground block mb-2">
              Key Created
            </label>
            <div className="text-sm">
              {(() => {
                const createdDate = keyVault.keyCreatedAt ? new Date(keyVault.keyCreatedAt) : null;
                const isValidCreatedDate = createdDate && createdDate.getTime() > 0;
                return isValidCreatedDate
                  ? formatDistanceToNow(createdDate, { addSuffix: true })
                  : 'Unknown';
              })()}
            </div>
          </div>

          {/* Previous Key Info */}
          {keyVault.hasPreviousPublicKey && (
            <div className="p-3 bg-amber-50 dark:bg-amber-950 rounded-md border border-amber-200 dark:border-amber-800">
              <div className="text-sm text-amber-800 dark:text-amber-200">
                <strong>Grace Period Active:</strong> A previous public key is still valid temporarily
                to prevent service disruption during key updates.
              </div>
            </div>
          )}
        </div>
      </Card>

      {/* Security Note */}
      <Card className="p-4 bg-gray-50 dark:bg-gray-900">
        <div className="text-sm text-muted-foreground">
          <p className="font-medium mb-2">Key Management Best Practices</p>
          <ul className="list-disc list-inside space-y-1 text-xs">
            <li><strong>Private key isolation:</strong> The private key exists only on your agent — never transmitted or stored elsewhere</li>
            <li><strong>Key compromise:</strong> If you suspect compromise, re-register your agent to generate a new keypair</li>
            <li><strong>Daily authentication:</strong> Use <strong>API Keys</strong> for routine auth — they're designed for regular rotation</li>
            <li><strong>Cryptographic identity:</strong> This keypair is your agent's long-term identity — rotate only when necessary</li>
          </ul>
        </div>
      </Card>
    </div>
  );
}
