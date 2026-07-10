import { useState } from 'react';
import { Check, FolderKanban, Loader2, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useProjectStore } from '@/stores/projectStore';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { useCreateUserWorkspace, useUserWorkspaces, type UserWorkspace } from './data';

export default function Workspaces() {
  const { t } = useTranslation();
  const { selectedProjectId, setSelectedProjectId } = useProjectStore();
  const query = useUserWorkspaces();
  const createWorkspace = useCreateUserWorkspace();
  const [open, setOpen] = useState(false);
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');

  const workspaces = query.data?.workspaces ?? [];
  const settings = query.data?.settings;
  const canCreate = Boolean(settings?.allowSelfServiceCreation && workspaces.length < settings.maxWorkspacesPerUser);

  const handleCreate = async () => {
    try {
      const created = await createWorkspace.mutateAsync({ name, description });
      setSelectedProjectId(created.id);
      setName('');
      setDescription('');
      setOpen(false);
      toast.success(t('workspaces.create.success'));
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('workspaces.create.error'));
    }
  };

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='workspaces-page'>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('workspaces.title')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>{t('workspaces.description')}</p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button disabled={!canCreate} data-testid='create-workspace'>
              <Plus className='h-4 w-4' />
              {t('workspaces.create.action')}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('workspaces.create.title')}</DialogTitle>
              <DialogDescription>{t('workspaces.create.description')}</DialogDescription>
            </DialogHeader>
            <div className='space-y-4'>
              <div className='space-y-2'>
                <Label htmlFor='workspace-name'>{t('workspaces.fields.name')}</Label>
                <Input id='workspace-name' value={name} maxLength={80} onChange={(event) => setName(event.target.value)} />
              </div>
              <div className='space-y-2'>
                <Label htmlFor='workspace-description'>{t('workspaces.fields.description')}</Label>
                <Textarea id='workspace-description' value={description} onChange={(event) => setDescription(event.target.value)} />
              </div>
            </div>
            <DialogFooter>
              <Button variant='outline' onClick={() => setOpen(false)}>
                {t('workspaces.create.cancel')}
              </Button>
              <Button onClick={handleCreate} disabled={!name.trim() || createWorkspace.isPending}>
                {createWorkspace.isPending && <Loader2 className='h-4 w-4 animate-spin' />}
                {t('workspaces.create.confirm')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </header>

      {settings && !settings.allowSelfServiceCreation && (
        <Alert>
          <AlertTitle>{t('workspaces.policy.managed.title')}</AlertTitle>
          <AlertDescription>{t('workspaces.policy.managed.description')}</AlertDescription>
        </Alert>
      )}
      {settings?.allowSelfServiceCreation && !canCreate && (
        <Alert>
          <AlertTitle>{t('workspaces.policy.limit.title')}</AlertTitle>
          <AlertDescription>{t('workspaces.policy.limit.description', { count: settings.maxWorkspacesPerUser })}</AlertDescription>
        </Alert>
      )}

      {query.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('workspaces.loadError.title')}</AlertTitle>
          <AlertDescription>{t('workspaces.loadError.description')}</AlertDescription>
        </Alert>
      )}

      <section className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {query.isLoading && [0, 1].map((item) => <Skeleton key={item} className='h-44 w-full' />)}
        {workspaces.map((workspace) => (
          <WorkspaceCard
            key={workspace.id}
            workspace={workspace}
            selected={workspace.id === selectedProjectId}
            onSelect={() => setSelectedProjectId(workspace.id)}
          />
        ))}
      </section>

      {!query.isLoading && workspaces.length === 0 && (
        <div className='flex min-h-48 flex-col items-center justify-center gap-3 border border-dashed p-6 text-center'>
          <FolderKanban className='text-muted-foreground h-8 w-8' />
          <div>
            <p className='font-medium'>{t('workspaces.empty.title')}</p>
            <p className='text-muted-foreground mt-1 text-sm'>{t('workspaces.empty.description')}</p>
          </div>
        </div>
      )}
    </div>
  );
}

function WorkspaceCard({ workspace, selected, onSelect }: { workspace: UserWorkspace; selected: boolean; onSelect: () => void }) {
  const { t } = useTranslation();
  return (
    <Card className={selected ? 'border-primary' : undefined} data-testid={`workspace-${workspace.id}`}>
      <CardHeader className='flex flex-row items-start justify-between gap-3'>
        <div className='min-w-0'>
          <CardTitle className='truncate text-base' title={workspace.name}>
            {workspace.name}
          </CardTitle>
          <p className='text-muted-foreground mt-1 line-clamp-2 min-h-10 text-sm'>
            {workspace.description || t('workspaces.noDescription')}
          </p>
        </div>
        <Badge variant={workspace.isOwner ? 'default' : 'secondary'}>
          {workspace.isOwner ? t('workspaces.owner') : t('workspaces.member')}
        </Badge>
      </CardHeader>
      <CardContent className='flex items-center justify-between gap-3'>
        <div className='text-muted-foreground text-sm'>{t('workspaces.capabilities.consumer')}</div>
        <Button variant={selected ? 'secondary' : 'outline'} onClick={onSelect} disabled={selected}>
          {selected && <Check className='h-4 w-4' />}
          {selected ? t('workspaces.selected') : t('workspaces.select')}
        </Button>
      </CardContent>
    </Card>
  );
}
