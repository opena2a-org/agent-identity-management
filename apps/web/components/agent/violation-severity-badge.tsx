import { Badge } from '@/components/ui/badge';

interface ViolationSeverityBadgeProps {
  severity: 'critical' | 'high' | 'medium' | 'low';
}

export function ViolationSeverityBadge({ severity }: ViolationSeverityBadgeProps) {
  const variants = {
    critical: 'destructive',
    high: 'destructive',
    medium: 'default',
    low: 'secondary'
  };

  const colors = {
    critical: 'bg-red-600 text-white',
    high: 'bg-orange-500 text-white',
    medium: 'bg-yellow-500 text-black',
    low: 'bg-blue-500 text-white'
  };

  return (
    <Badge variant={variants[severity] as any} className={colors[severity]}>
      {severity.toUpperCase()}
    </Badge>
  );
}
