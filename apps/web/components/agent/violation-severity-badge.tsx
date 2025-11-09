import { Badge } from '@/components/ui/badge';

interface ViolationSeverityBadgeProps {
  severity: 'critical' | 'high' | 'medium' | 'low' | 'warning' | 'info';
}

export function ViolationSeverityBadge({ severity }: ViolationSeverityBadgeProps) {
  // Map legacy severity values to new ones
  const normalizedSeverity = severity === 'warning' ? 'medium' : severity === 'info' ? 'low' : severity;

  const variants: Record<string, string> = {
    critical: 'destructive',
    high: 'destructive',
    medium: 'default',
    low: 'secondary'
  };

  const colors: Record<string, string> = {
    critical: 'bg-red-600 text-white',
    high: 'bg-orange-500 text-white',
    medium: 'bg-yellow-500 text-black',
    low: 'bg-blue-500 text-white'
  };

  return (
    <Badge variant={variants[normalizedSeverity] as any} className={colors[normalizedSeverity]}>
      {normalizedSeverity.toUpperCase()}
    </Badge>
  );
}
