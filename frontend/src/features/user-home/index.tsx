import { Link } from '@tanstack/react-router';
import { IconActivity, IconKey, IconRobot, IconWallet } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useMyCommercialProfile } from '@/features/billing/data/commercial-profile';
import { useMyProjects } from '@/features/projects/data/projects';

function formatMicros(value: number | undefined, currency: string | undefined) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
  }).format((value ?? 0) / 1_000_000);
}

export default function UserHome() {
  const { t } = useTranslation();
  const selectedProjectId = useSelectedProjectId();
  const projects = useMyProjects();
  const profile = useMyCommercialProfile({ projectId: selectedProjectId || undefined, limit: 5 });
  const activeProject = projects.data?.find((project) => project.id === selectedProjectId);
  const account = profile.data?.billingAccount;
  const totals = profile.data?.totals;

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='user-home'>
      <header>
        <h1 className='text-2xl font-semibold'>{t('userHome.title')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>{t('userHome.description')}</p>
      </header>

      {!activeProject && !projects.isLoading && (
        <Alert>
          <AlertTitle>{t('userHome.noProject.title')}</AlertTitle>
          <AlertDescription>{t('userHome.noProject.description')}</AlertDescription>
        </Alert>
      )}

      <section className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        <SummaryCard
          icon={IconWallet}
          title={t('userHome.metrics.available')}
          value={profile.isLoading ? '-' : formatMicros(account?.availableMicros, account?.currency)}
        />
        <SummaryCard
          icon={IconActivity}
          title={t('userHome.metrics.requests')}
          value={profile.isLoading ? '-' : String(totals?.requestCount ?? 0)}
        />
        <SummaryCard
          icon={IconWallet}
          title={t('userHome.metrics.today')}
          value={profile.isLoading ? '-' : formatMicros(totals?.todayConsumptionMicros, account?.currency)}
        />
        <SummaryCard
          icon={IconRobot}
          title={t('userHome.metrics.project')}
          value={activeProject?.name || t('userHome.metrics.notSelected')}
        />
      </section>

      {profile.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('userHome.loadError.title')}</AlertTitle>
          <AlertDescription>{t('userHome.loadError.description')}</AlertDescription>
        </Alert>
      )}

      <section className='flex flex-wrap gap-3'>
        <Button asChild disabled={!activeProject}>
          <Link to='/project/api-keys'>
            <IconKey className='h-4 w-4' />
            {t('userHome.actions.apiKeys')}
          </Link>
        </Button>
        <Button asChild variant='outline' disabled={!activeProject}>
          <Link to='/project/playground'>
            <IconRobot className='h-4 w-4' />
            {t('userHome.actions.playground')}
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

function SummaryCard({ icon: Icon, title, value }: { icon: typeof IconActivity; title: string; value: string }) {
  return (
    <Card>
      <CardHeader className='flex flex-row items-center justify-between gap-3 pb-2'>
        <CardTitle className='text-sm font-medium'>{title}</CardTitle>
        <Icon className='text-muted-foreground h-4 w-4' />
      </CardHeader>
      <CardContent>
        <div className='truncate text-xl font-semibold' title={value}>
          {value}
        </div>
      </CardContent>
    </Card>
  );
}
