import { Bell, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import {
  useMarkAllBillingNotificationsRead,
  useMarkBillingNotificationRead,
  useSaveMyBillingNotificationPreference,
  type BillingNotificationPreference,
} from '../data/billing';
import { BillingSection } from '../shared';
import { formatDate, microsToAmount, type BillingViewProps } from '../utils';

export function NotificationsView({ data, isLoading, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const savePreference = useSaveMyBillingNotificationPreference();
  const markRead = useMarkBillingNotificationRead();
  const markAll = useMarkAllBillingNotificationsRead();
  const preference = data?.notificationPreference;
  const unread = data?.notifications.filter((item) => item.status === 'unread').length ?? 0;

  const toggle = async (key: keyof Omit<BillingNotificationPreference, 'id'>) => {
    if (!preference) return;
    try {
      await savePreference.mutateAsync({
        enabled: preference.enabled,
        lowBalanceEnabled: preference.lowBalanceEnabled,
        paymentEnabled: preference.paymentEnabled,
        subscriptionEnabled: preference.subscriptionEnabled,
        largeConsumptionEnabled: preference.largeConsumptionEnabled,
        [key]: !preference[key],
      });
      toast.success(t('billing.notifications.preferenceSaved'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  const markAllRead = async () => {
    try {
      const count = await markAll.mutateAsync();
      toast.success(t('billing.notifications.markAllSuccess', { count }));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  return (
    <div className='grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]' data-testid='billing-notifications-view'>
      <BillingSection title={t('billing.notifications.preferencesTitle')} description={t('billing.notifications.description')}>
        <div className='space-y-2'>
          {(
            [
              ['enabled', t('billing.notifications.pref.enabled')],
              ['lowBalanceEnabled', t('billing.notifications.pref.lowBalance')],
              ['paymentEnabled', t('billing.notifications.pref.payment')],
              ['subscriptionEnabled', t('billing.notifications.pref.subscription')],
              ['largeConsumptionEnabled', t('billing.notifications.pref.largeConsumption')],
            ] as Array<[keyof Omit<BillingNotificationPreference, 'id'>, string]>
          ).map(([key, label]) => (
            <label key={key} className='flex items-center justify-between gap-3 border px-3 py-2 text-sm'>
              <span>{label}</span>
              <Switch
                checked={Boolean(preference?.[key])}
                disabled={!preference || savePreference.isPending}
                onCheckedChange={() => toggle(key)}
              />
            </label>
          ))}
        </div>
      </BillingSection>

      <BillingSection title={t('billing.notifications.title')} description={t('billing.notifications.listDescription')}>
        <div className='mb-4 flex justify-end'>
          <Button size='sm' variant='outline' disabled={markAll.isPending || unread === 0} onClick={markAllRead}>
            {markAll.isPending ? <Loader2 className='size-4 animate-spin' /> : <Bell className='size-4' />}
            {t('billing.notifications.markAllRead')}
          </Button>
        </div>
        <div className='divide-y border'>
          {data?.notifications.map((notification) => (
            <article key={notification.id} className='flex flex-wrap items-start justify-between gap-4 p-4 text-sm'>
              <div className='min-w-0 flex-1'>
                <div className='flex flex-wrap items-center gap-2'>
                  <h3 className='font-medium'>{notification.title}</h3>
                  <Badge variant={notification.status === 'unread' ? 'default' : 'secondary'}>{notification.status}</Badge>
                  <Badge variant={notification.severity === 'error' ? 'destructive' : 'outline'}>{notification.category}</Badge>
                </div>
                <p className='text-muted-foreground mt-2'>{notification.message}</p>
                <div className='text-muted-foreground mt-2 flex flex-wrap gap-3 text-xs'>
                  <span>{formatDate(notification.createdAt)}</span>
                  {typeof notification.amountMicros === 'number' && (
                    <span className='font-mono'>{formatCurrency.format(microsToAmount(notification.amountMicros))}</span>
                  )}
                </div>
              </div>
              {notification.status === 'unread' && (
                <Button size='sm' variant='ghost' disabled={markRead.isPending} onClick={() => markRead.mutate(notification.id)}>
                  {t('billing.notifications.markRead')}
                </Button>
              )}
            </article>
          ))}
          {!isLoading && (data?.notifications.length ?? 0) === 0 && (
            <p className='text-muted-foreground p-10 text-center text-sm'>{t('common.noData')}</p>
          )}
        </div>
      </BillingSection>
    </div>
  );
}
