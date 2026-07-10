import { Boxes, CircleDollarSign } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Skeleton } from '@/components/ui/skeleton';
import { usePersonalAPIKeyModels } from '@/features/personal-api-keys/data';

export default function ConsumerModelCatalog() {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();
  const models = usePersonalAPIKeyModels(projectId);

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='consumer-model-catalog'>
      <header>
        <h1 className='text-2xl font-semibold'>{t('modelCatalog.title')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>{t('modelCatalog.description')}</p>
      </header>

      {models.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('modelCatalog.loadFailed')}</AlertTitle>
          <AlertDescription>{models.error instanceof Error ? models.error.message : t('modelCatalog.loadFailed')}</AlertDescription>
        </Alert>
      )}
      {models.isLoading ? (
        <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
          {[0, 1, 2].map((item) => (
            <Skeleton key={item} className='h-48' />
          ))}
        </div>
      ) : (
        <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
          {models.data?.models.map((model) => (
            <article key={model.modelId} className='min-w-0 border p-4' data-testid={`consumer-model-${model.modelId}`}>
              <div className='flex items-start justify-between gap-3'>
                <div className='min-w-0'>
                  <h2 className='truncate font-semibold' title={model.modelId}>
                    {model.modelId}
                  </h2>
                  <p className='text-muted-foreground mt-1 text-xs'>{t('modelCatalog.available')}</p>
                </div>
                <Badge variant='secondary'>{model.currency || 'CNY'}</Badge>
              </div>
              <div className='mt-5 space-y-2'>
                {(model.price?.items || []).map((item, index) => (
                  <div key={`${item.itemCode}-${index}`} className='flex items-start justify-between gap-4 border-t pt-2 text-sm'>
                    <span className='text-muted-foreground min-w-0 truncate'>{priceItemLabel(item.itemCode, t)}</span>
                    <span className='shrink-0 text-right font-mono text-xs'>{formatPricing(item.pricing, model.currency || 'CNY', t)}</span>
                  </div>
                ))}
                {(model.price?.items.length ?? 0) === 0 && (
                  <div className='text-muted-foreground flex items-center gap-2 border-t pt-3 text-sm'>
                    <CircleDollarSign className='size-4' />
                    {t('modelCatalog.noPrice')}
                  </div>
                )}
              </div>
            </article>
          ))}
          {(models.data?.models.length ?? 0) === 0 && (
            <div className='text-muted-foreground flex min-h-56 flex-col items-center justify-center gap-3 border border-dashed p-6 text-center md:col-span-2 xl:col-span-3'>
              <Boxes className='size-8' />
              <p className='font-medium'>{t('modelCatalog.empty')}</p>
            </div>
          )}
        </section>
      )}
    </div>
  );
}

function priceItemLabel(code: string, t: (key: string) => string) {
  const key = `modelCatalog.items.${code}`;
  const translated = t(key);
  return translated === key ? code : translated;
}

function formatPricing(pricing: Record<string, unknown>, currency: string, t: (key: string, options?: Record<string, unknown>) => string) {
  const mode = typeof pricing.mode === 'string' ? pricing.mode : '';
  const perUnit = pricing.usagePerUnit;
  const flatFee = pricing.flatFee;
  const formatter = new Intl.NumberFormat(undefined, { style: 'currency', currency, maximumFractionDigits: 6 });
  if (mode === 'usage_per_unit' && (typeof perUnit === 'string' || typeof perUnit === 'number')) {
    return t('modelCatalog.perMillion', { amount: formatter.format(Number(perUnit)) });
  }
  if (mode === 'flat_fee' && (typeof flatFee === 'string' || typeof flatFee === 'number')) return formatter.format(Number(flatFee));
  return mode || t('modelCatalog.customPrice');
}
