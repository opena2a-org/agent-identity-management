import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Crown, ArrowRight } from 'lucide-react';

export function PremiumUpsellBanner() {
  return (
    <Card className="p-6 bg-gradient-to-r from-purple-50 to-blue-50 border-purple-200">
      <div className="flex items-start gap-4">
        <div className="bg-purple-600 p-3 rounded-lg">
          <Crown className="h-6 w-6 text-white" />
        </div>
        <div className="flex-1">
          <h4 className="font-semibold text-lg mb-2">Upgrade to Premium Secrets Management</h4>
          <p className="text-sm text-muted-foreground mb-4">
            Automatically store, rotate, and inject third-party secrets (Stripe, OpenAI, AWS keys)
            with our enterprise secrets vault. Save 20+ hours/month on secret management.
          </p>
          <ul className="text-sm space-y-1 mb-4">
            <li>✓ Third-party API key storage (Stripe, OpenAI, AWS)</li>
            <li>✓ Automatic secret rotation (30/60/90 days)</li>
            <li>✓ Runtime secret injection (zero hardcoding)</li>
            <li>✓ Secret leak detection & auto-revocation</li>
            <li>✓ Compliance reporting (SOC 2, HIPAA, GDPR)</li>
          </ul>
          <div className="flex gap-2">
            <Button size="sm">
              Learn More
              <ArrowRight className="ml-2 h-4 w-4" />
            </Button>
            <Button size="sm" variant="outline">
              Upgrade to Pro ($199/mo)
            </Button>
          </div>
        </div>
      </div>
    </Card>
  );
}
