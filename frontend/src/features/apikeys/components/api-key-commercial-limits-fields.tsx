import { FieldValues, Path, UseFormReturn } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { amountToMicros, DEFAULT_COMMERCIAL_LIMIT_CURRENCY, microsToAmountInput } from './api-key-commercial-limits-utils';
import type { CommercialLimitsFormValues } from './api-key-commercial-limits-utils';

type ApiKeyCommercialLimitsFieldsProps<T extends CommercialLimitsFormValues> = {
  form: UseFormReturn<T>;
};

function fieldPath<T extends FieldValues>(name: string) {
  return name as Path<T>;
}

export function ApiKeyCommercialLimitsFields<T extends CommercialLimitsFormValues>({ form }: ApiKeyCommercialLimitsFieldsProps<T>) {
  const { t } = useTranslation();
  const enabled = form.watch(fieldPath<T>('commercialLimits.enabled'));

  const amountFields = [
    { name: 'commercialLimits.totalBudgetMicros', label: t('apikeys.commercialLimits.totalBudget') },
    { name: 'commercialLimits.dailyBudgetMicros', label: t('apikeys.commercialLimits.dailyBudget') },
    { name: 'commercialLimits.monthlyBudgetMicros', label: t('apikeys.commercialLimits.monthlyBudget') },
    { name: 'commercialLimits.singleRequestMaxMicros', label: t('apikeys.commercialLimits.singleRequestMax') },
  ] as const;

  return (
    <div className='rounded-md border p-4'>
      <div className='mb-4 flex items-center justify-between gap-4'>
        <div className='space-y-1'>
          <FormLabel>{t('apikeys.commercialLimits.title')}</FormLabel>
          <FormDescription>{t('apikeys.commercialLimits.description')}</FormDescription>
        </div>
        <FormField
          control={form.control}
          name={fieldPath<T>('commercialLimits.enabled')}
          render={({ field }) => (
            <FormItem>
              <FormControl>
                <Switch checked={Boolean(field.value)} onCheckedChange={field.onChange} aria-label={t('apikeys.commercialLimits.enabled')} />
              </FormControl>
            </FormItem>
          )}
        />
      </div>

      <div className='grid gap-4 sm:grid-cols-2'>
        <FormField
          control={form.control}
          name={fieldPath<T>('commercialLimits.currency')}
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('apikeys.commercialLimits.currency')}</FormLabel>
              <FormControl>
                <Input
                  {...field}
                  value={(field.value as string | null | undefined) || DEFAULT_COMMERCIAL_LIMIT_CURRENCY}
                  onChange={(event) => field.onChange(event.target.value.toUpperCase())}
                  maxLength={8}
                  disabled={!enabled}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        {amountFields.map((amountField) => (
          <FormField
            key={amountField.name}
            control={form.control}
            name={fieldPath<T>(amountField.name)}
            render={({ field }) => (
              <FormItem>
                <FormLabel>{amountField.label}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    step='0.000001'
                    value={microsToAmountInput(field.value as number | null | undefined)}
                    onChange={(event) => field.onChange(amountToMicros(event.target.value))}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        ))}
      </div>

      <FormField
        control={form.control}
        name={fieldPath<T>('commercialLimits.notes')}
        render={({ field }) => (
          <FormItem className='mt-4'>
            <FormLabel>{t('apikeys.commercialLimits.notes')}</FormLabel>
            <FormControl>
              <Textarea
                value={(field.value as string | null | undefined) || ''}
                onChange={field.onChange}
                rows={2}
                disabled={!enabled}
                className='resize-none'
              />
            </FormControl>
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  );
}
