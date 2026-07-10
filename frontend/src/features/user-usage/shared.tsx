import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import type { UserUsageScope } from './data';

export function ScopeSwitch({
  value,
  onChange,
  canViewProject,
  mineLabel,
  projectLabel,
}: {
  value: UserUsageScope;
  onChange: (scope: UserUsageScope) => void;
  canViewProject: boolean;
  mineLabel: string;
  projectLabel: string;
}) {
  if (!canViewProject) return null;
  return (
    <div className='bg-muted inline-flex h-9 items-center p-1' data-testid='user-usage-scope-switch'>
      <Button
        type='button'
        size='sm'
        variant={value === 'mine' ? 'secondary' : 'ghost'}
        className='h-7 px-3 shadow-none'
        aria-pressed={value === 'mine'}
        onClick={() => onChange('mine')}
      >
        {mineLabel}
      </Button>
      <Button
        type='button'
        size='sm'
        variant={value === 'project' ? 'secondary' : 'ghost'}
        className='h-7 px-3 shadow-none'
        aria-pressed={value === 'project'}
        onClick={() => onChange('project')}
      >
        {projectLabel}
      </Button>
    </div>
  );
}

export function StatusBadge({ status }: { status: string }) {
  const variant = status === 'completed' ? 'default' : status === 'failed' || status === 'canceled' ? 'destructive' : 'secondary';
  return <Badge variant={variant}>{status}</Badge>;
}
