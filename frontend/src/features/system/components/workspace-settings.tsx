import { useEffect, useState } from 'react';
import { Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useUpdateWorkspaceSettings, useWorkspaceSettings } from '@/features/workspaces/data';

export function WorkspaceSettings() {
  const { t } = useTranslation();
  const query = useWorkspaceSettings();
  const update = useUpdateWorkspaceSettings();
  const [allowSelfServiceCreation, setAllowSelfServiceCreation] = useState(false);
  const [maxWorkspacesPerUser, setMaxWorkspacesPerUser] = useState(1);

  useEffect(() => {
    if (query.data) {
      setAllowSelfServiceCreation(query.data.allowSelfServiceCreation);
      setMaxWorkspacesPerUser(query.data.maxWorkspacesPerUser);
    }
  }, [query.data]);

  const hasChanges = Boolean(
    query.data &&
    (query.data.allowSelfServiceCreation !== allowSelfServiceCreation || query.data.maxWorkspacesPerUser !== maxWorkspacesPerUser)
  );

  const handleSave = async () => {
    try {
      await update.mutateAsync({ allowSelfServiceCreation, maxWorkspacesPerUser });
      toast.success(t('system.workspacePolicy.saved'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('system.workspacePolicy.saveError'));
    }
  };

  return (
    <Card data-testid='workspace-policy-settings'>
      <CardHeader>
        <CardTitle>{t('system.workspacePolicy.title')}</CardTitle>
        <CardDescription>{t('system.workspacePolicy.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div className='space-y-1'>
            <Label htmlFor='workspace-self-service'>{t('system.workspacePolicy.selfService.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.workspacePolicy.selfService.description')}</div>
          </div>
          <Switch
            id='workspace-self-service'
            checked={allowSelfServiceCreation}
            onCheckedChange={setAllowSelfServiceCreation}
            disabled={query.isLoading || update.isPending}
          />
        </div>
        <div className='max-w-sm space-y-2'>
          <Label htmlFor='workspace-user-limit'>{t('system.workspacePolicy.limit.label')}</Label>
          <Input
            id='workspace-user-limit'
            type='number'
            min={1}
            value={maxWorkspacesPerUser}
            onChange={(event) => setMaxWorkspacesPerUser(Math.max(1, Number(event.target.value) || 1))}
            disabled={query.isLoading || update.isPending}
          />
          <div className='text-muted-foreground text-sm'>{t('system.workspacePolicy.limit.description')}</div>
        </div>
        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={!hasChanges || update.isPending}>
            {update.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : <Save className='h-4 w-4' />}
            {t('system.buttons.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
