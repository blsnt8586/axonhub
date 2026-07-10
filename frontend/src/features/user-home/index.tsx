import { Link } from '@tanstack/react-router';
import { IconActivity, IconCreditCard, IconKey, IconRobot, IconStack2, IconWallet } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { type WorkspaceBlockReason, useUserWorkspaceSummary } from './data';

function formatMicros(value: number | undefined, currency: string | undefined) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
  }).format((value ?? 0) / 1_000_000);
}

export default function UserHome() {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const summary = useUserWorkspaceSummary(selectedProjectId || undefined);
  const data = summary.data;
  const canUseAI = data?.onboarding?.canUseAI === true;

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='user-home'>
      <header>
        <h1 className='text-2xl font-semibold'>{t('userHome.title')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>{t('userHome.description')}</p>
      </header>

      {summary.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('userHome.loadError.title')}</AlertTitle>
          <AlertDescription>{t('userHome.loadError.description')}</AlertDescription>
        </Alert>
      )}

      {data?.onboarding && <OnboardingAlert reason={data.onboarding.blockReason} />}

      <section className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4'>
        <SummaryCard
          icon={IconWallet}
          title={t('userHome.metrics.available')}
          value={summary.isLoading ? undefined : formatMicros(data?.billing.availableMicros, data?.billing.currency)}
          testId='workspace-available-balance'
        />
        <SummaryCard
          icon={IconActivity}
          title={t('userHome.metrics.requests')}
          value={summary.isLoading ? undefined : String(data?.usage.requestCount ?? 0)}
          testId='workspace-request-count'
        />
        <SummaryCard
          icon={IconWallet}
          title={t('userHome.metrics.today')}
          value={summary.isLoading ? undefined : formatMicros(data?.usage.todayConsumptionMicros, data?.billing.currency)}
          testId='workspace-today-consumption'
        />
        <SummaryCard
          icon={IconStack2}
          title={t('userHome.metrics.project')}
          value={summary.isLoading ? undefined : data?.project?.name || t('userHome.metrics.notSelected')}
          testId='workspace-project'
        />
        <SummaryCard
          icon={IconKey}
          title={t('userHome.metrics.apiKeys')}
          value={
            summary.isLoading
              ? undefined
              : t('userHome.metrics.apiKeysValue', { enabled: data?.apiKeys.enabled ?? 0, total: data?.apiKeys.total ?? 0 })
          }
          testId='workspace-api-key-count'
        />
        <SummaryCard
          icon={IconRobot}
          title={t('userHome.metrics.models')}
          value={summary.isLoading ? undefined : String(data?.models.availableCount ?? 0)}
          testId='workspace-model-count'
        />
        <SummaryCard
          icon={IconCreditCard}
          title={t('userHome.metrics.subscriptions')}
          value={summary.isLoading ? undefined : String(data?.subscriptions.activeCount ?? 0)}
          testId='workspace-subscription-count'
        />
      </section>

      <section className='flex flex-wrap gap-3'>
        <Button asChild disabled={!data?.project}>
          <Link to='/project/api-keys'>
            <IconKey className='h-4 w-4' />
            {t('userHome.actions.apiKeys')}
          </Link>
        </Button>
        {canUseAI ? (
          <Button asChild variant='outline'>
            <Link to='/project/playground'>
              <IconRobot className='h-4 w-4' />
              {t('userHome.actions.playground')}
            </Link>
          </Button>
        ) : (
          <Button variant='outline' disabled>
            <IconRobot className='h-4 w-4' />
            {t('userHome.actions.playground')}
          </Button>
        )}
        <Button asChild variant='outline'>
          <Link to='/project/requests'>
            <IconActivity className='h-4 w-4' />
            {t('userHome.actions.usage')}
          </Link>
        </Button>
        <Button asChild variant='outline'>
          <Link to='/billing'>
            <IconWallet className='h-4 w-4' />
            {t('userHome.actions.billing')}
          </Link>
        </Button>
      </section>
    </div>
  );
}

function OnboardingAlert({ reason }: { reason: WorkspaceBlockReason }) {
  const { t } = useTranslation();
  if (reason === 'none') {
    return (
      <Alert data-testid='workspace-ready'>
        <AlertTitle>{t('userHome.states.ready.title')}</AlertTitle>
        <AlertDescription>{t('userHome.states.ready.description')}</AlertDescription>
      </Alert>
    );
  }

  return (
    <Alert
      variant={reason === 'account_unavailable' || reason === 'user_inactive' ? 'destructive' : 'default'}
      data-testid={`workspace-state-${reason}`}
    >
      <AlertTitle>{t(`userHome.states.${reason}.title`)}</AlertTitle>
      <AlertDescription>{t(`userHome.states.${reason}.description`)}</AlertDescription>
    </Alert>
  );
}

function SummaryCard({ icon: Icon, title, value, testId }: { icon: typeof IconActivity; title: string; value?: string; testId: string }) {
  return (
    <Card data-testid={testId}>
      <CardHeader className='flex flex-row items-center justify-between gap-3 pb-2'>
        <CardTitle className='text-sm font-medium'>{title}</CardTitle>
        <Icon className='text-muted-foreground h-4 w-4' />
      </CardHeader>
      <CardContent>
        {value === undefined ? (
          <Skeleton className='h-7 w-24' />
        ) : (
          <div className='truncate text-xl font-semibold' title={value}>
            {value}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
