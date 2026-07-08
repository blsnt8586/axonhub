import { Copy, Eye } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { useApiKeysContext } from '../context/apikeys-context';
import { useApiKeyCommercialLimitUsage } from '../data/apikeys';
import type { ApiKey, ApiKeyCommercialLimitWindowUsage } from '../data/schema';

export function ApiKeyCell({ apiKey, fullApiKey }: { apiKey: string; fullApiKey: ApiKey }) {
  const { t } = useTranslation();
  const { openDialog } = useApiKeysContext();

  const maskedKey = apiKey.replace(/./g, '*').slice(0, -4) + apiKey.slice(-4);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey);
    toast.success(t('apikeys.messages.copied'));
  };

  const handleViewKey = () => {
    openDialog('view', fullApiKey);
  };

  return (
    <div className='flex max-w-48 items-center space-x-2'>
      <code className='bg-muted truncate rounded px-2 py-1 font-mono text-sm'>{maskedKey}</code>
      <Button variant='ghost' size='sm' onClick={handleViewKey} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.view')}>
        <Eye className='h-3 w-3' />
      </Button>
      <Button variant='ghost' size='sm' onClick={copyToClipboard} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.copy')}>
        <Copy className='h-3 w-3' />
      </Button>
    </div>
  );
}

function formatMicros(value: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    currencyDisplay: 'narrowSymbol',
    maximumFractionDigits: 2,
  }).format(value / 1_000_000);
}

function BudgetLine({
  label,
  usage,
  currency,
}: {
  label: string;
  usage: ApiKeyCommercialLimitWindowUsage;
  currency: string;
}) {
  if (usage.budgetMicros == null) return null;

  return (
    <div className={cn('flex items-center justify-between gap-3', usage.exceeded && 'text-red-600')}>
      <span className='text-muted-foreground'>{label}</span>
      <span className='font-mono'>
        {formatMicros(usage.spentMicros, currency)} / {formatMicros(usage.remainingMicros ?? 0, currency)}
      </span>
    </div>
  );
}

export function ApiKeyBudgetCell({ apiKey }: { apiKey: ApiKey }) {
  const { t } = useTranslation();
  const enabled = Boolean(apiKey.commercialLimits?.enabled);
  const currency = apiKey.commercialLimits?.currency || 'CNY';
  const usage = useApiKeyCommercialLimitUsage(apiKey.id, { enabled });

  if (!enabled) {
    return <span className='text-muted-foreground text-xs'>{t('apikeys.commercialLimits.unlimited')}</span>;
  }

  if (usage.isLoading) {
    return <span className='text-muted-foreground text-xs'>{t('common.loading')}</span>;
  }

  if (!usage.data) {
    return <span className='text-muted-foreground text-xs'>-</span>;
  }

  const hasBudget = usage.data.total.budgetMicros != null || usage.data.daily.budgetMicros != null || usage.data.monthly.budgetMicros != null;

  return (
    <div className='min-w-44 space-y-1 text-xs'>
      <BudgetLine label={t('apikeys.commercialLimits.dailyShort')} usage={usage.data.daily} currency={usage.data.currency || currency} />
      <BudgetLine label={t('apikeys.commercialLimits.monthlyShort')} usage={usage.data.monthly} currency={usage.data.currency || currency} />
      <BudgetLine label={t('apikeys.commercialLimits.totalShort')} usage={usage.data.total} currency={usage.data.currency || currency} />
      {!hasBudget && <span className='text-muted-foreground'>{t('apikeys.commercialLimits.unlimited')}</span>}
    </div>
  );
}
