import { useEffect, useState } from 'react';
import { Link } from '@tanstack/react-router';
import { Archive, Check, Copy, KeyRound, Loader2, Pencil, Plus, RefreshCw, Settings2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  useArchivePersonalAPIKey,
  useCreatePersonalAPIKey,
  usePersonalAPIKeyModels,
  usePersonalAPIKeys,
  useRotatePersonalAPIKey,
  useUpdatePersonalAPIKey,
  type CommercialLimits,
  type PersonalAPIKey,
  type PersonalAPIKeyInput,
  type RequestLimitWindow,
} from './data';

type KeyFormState = {
  name: string;
  status: 'enabled' | 'disabled';
  expiresAt: string;
  ipAllowlist: string;
  allowedModelIds: string[];
  requestLimit: string;
  requestLimitWindow: RequestLimitWindow;
  budgetEnabled: boolean;
  currency: string;
  totalBudget: string;
  dailyBudget: string;
  monthlyBudget: string;
  singleRequestMax: string;
};

const emptyForm: KeyFormState = {
  name: '',
  status: 'enabled',
  expiresAt: '',
  ipAllowlist: '',
  allowedModelIds: [],
  requestLimit: '',
  requestLimitWindow: 'minute',
  budgetEnabled: false,
  currency: 'CNY',
  totalBudget: '',
  dailyBudget: '',
  monthlyBudget: '',
  singleRequestMax: '',
};

export default function PersonalAPIKeys() {
  const { t } = useTranslation();
  const projectId = useSelectedProjectId();
  const { checkRouteAccess } = useRoutePermissions();
  const keys = usePersonalAPIKeys(projectId);
  const models = usePersonalAPIKeyModels(projectId);
  const createKey = useCreatePersonalAPIKey(projectId);
  const updateKey = useUpdatePersonalAPIKey(projectId);
  const rotateKey = useRotatePersonalAPIKey(projectId);
  const archiveKey = useArchivePersonalAPIKey(projectId);
  const [editing, setEditing] = useState<PersonalAPIKey | 'create' | null>(null);
  const [form, setForm] = useState<KeyFormState>(emptyForm);
  const [secret, setSecret] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<{ type: 'rotate' | 'archive'; key: PersonalAPIKey } | null>(null);

  const canManageShared = checkRouteAccess('/project/api-keys/shared').hasAccess;

  useEffect(() => {
    if (editing === 'create') setForm(emptyForm);
    else if (editing) setForm(formFromKey(editing));
  }, [editing]);

  const handleSubmit = async () => {
    try {
      const input = formInput(form);
      if (editing === 'create') {
        const result = await createKey.mutateAsync(input);
        setSecret(result.secret);
      } else if (editing) {
        await updateKey.mutateAsync({
          keyId: editing.id,
          input: {
            ...input,
            status: form.status,
            clearExpiresAt: !form.expiresAt,
            clearRequestLimit: !form.requestLimit,
            clearCommercialLimits: !form.budgetEnabled,
          },
        });
        toast.success(t('personalApiKeys.messages.updated'));
      }
      setEditing(null);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('personalApiKeys.messages.failed'));
    }
  };

  const confirmAction = async () => {
    if (!pendingAction) return;
    try {
      if (pendingAction.type === 'rotate') {
        const result = await rotateKey.mutateAsync(pendingAction.key.id);
        setSecret(result.secret);
      } else {
        await archiveKey.mutateAsync(pendingAction.key.id);
        toast.success(t('personalApiKeys.messages.archived'));
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('personalApiKeys.messages.failed'));
    } finally {
      setPendingAction(null);
    }
  };

  return (
    <div className='flex flex-1 flex-col gap-6 p-4 md:p-6' data-testid='personal-api-keys-page'>
      <header className='flex flex-wrap items-start justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('personalApiKeys.title')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>{t('personalApiKeys.description')}</p>
        </div>
        <div className='flex flex-wrap gap-2'>
          {canManageShared && (
            <Button asChild variant='outline'>
              <Link to='/project/api-keys/shared'>
                <Settings2 className='h-4 w-4' />
                {t('personalApiKeys.shared')}
              </Link>
            </Button>
          )}
          <Button onClick={() => setEditing('create')} disabled={!projectId} data-testid='create-personal-api-key'>
            <Plus className='h-4 w-4' />
            {t('personalApiKeys.create')}
          </Button>
        </div>
      </header>

      {!projectId && (
        <Alert>
          <AlertTitle>{t('personalApiKeys.noWorkspace.title')}</AlertTitle>
          <AlertDescription>{t('personalApiKeys.noWorkspace.description')}</AlertDescription>
        </Alert>
      )}
      {keys.isError && (
        <Alert variant='destructive'>
          <AlertTitle>{t('personalApiKeys.loadError.title')}</AlertTitle>
          <AlertDescription>{t('personalApiKeys.loadError.description')}</AlertDescription>
        </Alert>
      )}

      <section className='grid gap-4 xl:grid-cols-2'>
        {keys.isLoading && [0, 1].map((item) => <Skeleton key={item} className='h-56 w-full' />)}
        {keys.data?.apiKeys.map((key) => (
          <KeyItem key={key.id} apiKey={key} onEdit={() => setEditing(key)} onAction={(type) => setPendingAction({ type, key })} />
        ))}
      </section>

      {!keys.isLoading && projectId && (keys.data?.apiKeys.length ?? 0) === 0 && (
        <div className='flex min-h-52 flex-col items-center justify-center gap-3 border border-dashed p-6 text-center'>
          <KeyRound className='text-muted-foreground h-8 w-8' />
          <div>
            <p className='font-medium'>{t('personalApiKeys.empty.title')}</p>
            <p className='text-muted-foreground mt-1 text-sm'>{t('personalApiKeys.empty.description')}</p>
          </div>
        </div>
      )}

      <KeyFormDialog
        open={editing !== null}
        editing={editing !== 'create' && editing ? editing : null}
        form={form}
        setForm={setForm}
        models={models.data?.models ?? []}
        pending={createKey.isPending || updateKey.isPending}
        onClose={() => setEditing(null)}
        onSubmit={handleSubmit}
      />
      <SecretDialog secret={secret} onClose={() => setSecret(null)} />
      <AlertDialog open={pendingAction !== null} onOpenChange={(open) => !open && setPendingAction(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t(`personalApiKeys.actions.${pendingAction?.type}.title`)}</AlertDialogTitle>
            <AlertDialogDescription>{t(`personalApiKeys.actions.${pendingAction?.type}.description`)}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('personalApiKeys.cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={confirmAction}>{t('personalApiKeys.confirm')}</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

function KeyItem({
  apiKey,
  onEdit,
  onAction,
}: {
  apiKey: PersonalAPIKey;
  onEdit: () => void;
  onAction: (type: 'rotate' | 'archive') => void;
}) {
  const { t } = useTranslation();
  const expired = apiKey.expiresAt ? new Date(apiKey.expiresAt) <= new Date() : false;
  const allowedModelIds = apiKey.allowedModelIds ?? [];
  const ipAllowlist = apiKey.ipAllowlist ?? [];
  return (
    <article className='space-y-4 border p-4' data-testid={`personal-api-key-${apiKey.id}`}>
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <h2 className='truncate font-medium' title={apiKey.name}>
            {apiKey.name}
          </h2>
          <code className='text-muted-foreground mt-1 block truncate text-xs'>{apiKey.maskedKey}</code>
        </div>
        <Badge variant={apiKey.status === 'enabled' && !expired ? 'default' : 'secondary'}>
          {expired ? t('personalApiKeys.status.expired') : t(`personalApiKeys.status.${apiKey.status}`)}
        </Badge>
      </div>
      <dl className='grid gap-3 text-sm sm:grid-cols-2'>
        <KeyDetail
          label={t('personalApiKeys.fields.models')}
          value={allowedModelIds.length ? allowedModelIds.join(', ') : t('personalApiKeys.unlimited')}
        />
        <KeyDetail
          label={t('personalApiKeys.fields.ipAllowlist')}
          value={ipAllowlist.length ? ipAllowlist.join(', ') : t('personalApiKeys.unlimited')}
        />
        <KeyDetail
          label={t('personalApiKeys.fields.requestLimit')}
          value={
            apiKey.requestLimit
              ? `${apiKey.requestLimit} / ${t(`personalApiKeys.windows.${apiKey.requestLimitWindow}`)}`
              : t('personalApiKeys.unlimited')
          }
        />
        <KeyDetail
          label={t('personalApiKeys.fields.expiresAt')}
          value={apiKey.expiresAt ? new Date(apiKey.expiresAt).toLocaleString() : t('personalApiKeys.never')}
        />
      </dl>
      <div className='flex flex-wrap justify-end gap-2'>
        <Button size='sm' variant='outline' onClick={onEdit}>
          <Pencil className='h-4 w-4' />
          {t('personalApiKeys.edit')}
        </Button>
        <Button size='sm' variant='outline' onClick={() => onAction('rotate')}>
          <RefreshCw className='h-4 w-4' />
          {t('personalApiKeys.rotate')}
        </Button>
        <Button size='sm' variant='outline' onClick={() => onAction('archive')}>
          <Archive className='h-4 w-4' />
          {t('personalApiKeys.archive')}
        </Button>
      </div>
    </article>
  );
}

function KeyDetail({ label, value }: { label: string; value: string }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-xs'>{label}</dt>
      <dd className='mt-1 break-words'>{value}</dd>
    </div>
  );
}

function KeyFormDialog({
  open,
  editing,
  form,
  setForm,
  models,
  pending,
  onClose,
  onSubmit,
}: {
  open: boolean;
  editing: PersonalAPIKey | null;
  form: KeyFormState;
  setForm: React.Dispatch<React.SetStateAction<KeyFormState>>;
  models: Array<{ modelId: string; currency?: string; price?: { items: unknown[] } }>;
  pending: boolean;
  onClose: () => void;
  onSubmit: () => void;
}) {
  const { t } = useTranslation();
  const update = <K extends keyof KeyFormState>(key: K, value: KeyFormState[K]) => setForm((previous) => ({ ...previous, [key]: value }));
  return (
    <Dialog open={open} onOpenChange={(next) => !next && onClose()}>
      <DialogContent className='max-h-[90vh] overflow-y-auto sm:max-w-2xl'>
        <DialogHeader>
          <DialogTitle>{editing ? t('personalApiKeys.edit') : t('personalApiKeys.create')}</DialogTitle>
          <DialogDescription>{t('personalApiKeys.form.description')}</DialogDescription>
        </DialogHeader>
        <div className='grid gap-5 md:grid-cols-2'>
          <div className='space-y-2'>
            <Label htmlFor='personal-key-name'>{t('personalApiKeys.fields.name')}</Label>
            <Input id='personal-key-name' value={form.name} maxLength={80} onChange={(event) => update('name', event.target.value)} />
          </div>
          {editing && (
            <div className='space-y-2'>
              <Label htmlFor='personal-key-status'>{t('personalApiKeys.fields.status')}</Label>
              <Select value={form.status} onValueChange={(value) => update('status', value as KeyFormState['status'])}>
                <SelectTrigger id='personal-key-status'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='enabled'>{t('personalApiKeys.status.enabled')}</SelectItem>
                  <SelectItem value='disabled'>{t('personalApiKeys.status.disabled')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
          )}
          <div className='space-y-2'>
            <Label htmlFor='personal-key-expiry'>{t('personalApiKeys.fields.expiresAt')}</Label>
            <Input
              id='personal-key-expiry'
              type='datetime-local'
              value={form.expiresAt}
              onChange={(event) => update('expiresAt', event.target.value)}
            />
          </div>
          <div className='space-y-2 md:col-span-2'>
            <Label htmlFor='personal-key-ip'>{t('personalApiKeys.fields.ipAllowlist')}</Label>
            <Textarea
              id='personal-key-ip'
              value={form.ipAllowlist}
              onChange={(event) => update('ipAllowlist', event.target.value)}
              placeholder='203.0.113.10&#10;10.0.0.0/8'
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='personal-key-request-limit'>{t('personalApiKeys.fields.requestLimit')}</Label>
            <Input
              id='personal-key-request-limit'
              type='number'
              min={1}
              value={form.requestLimit}
              onChange={(event) => update('requestLimit', event.target.value)}
            />
          </div>
          <div className='space-y-2'>
            <Label htmlFor='personal-key-window'>{t('personalApiKeys.fields.requestLimitWindow')}</Label>
            <Select value={form.requestLimitWindow} onValueChange={(value) => update('requestLimitWindow', value as RequestLimitWindow)}>
              <SelectTrigger id='personal-key-window'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {(['minute', 'hour', 'day', 'all_time'] as const).map((window) => (
                  <SelectItem key={window} value={window}>
                    {t(`personalApiKeys.windows.${window}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-3 md:col-span-2'>
            <Label>{t('personalApiKeys.fields.models')}</Label>
            <div className='grid max-h-48 gap-2 overflow-y-auto border p-3 sm:grid-cols-2'>
              {models.length ? (
                models.map((model) => {
                  const checked = form.allowedModelIds.includes(model.modelId);
                  return (
                    <label key={model.modelId} className='flex min-w-0 items-start gap-2 text-sm'>
                      <Checkbox
                        checked={checked}
                        onCheckedChange={(next) =>
                          update(
                            'allowedModelIds',
                            next ? [...form.allowedModelIds, model.modelId] : form.allowedModelIds.filter((id) => id !== model.modelId)
                          )
                        }
                      />
                      <span className='min-w-0'>
                        <span className='block truncate'>{model.modelId}</span>
                        {model.price && <span className='text-muted-foreground text-xs'>{formatModelPrice(model)}</span>}
                      </span>
                    </label>
                  );
                })
              ) : (
                <span className='text-muted-foreground text-sm'>{t('personalApiKeys.modelsEmpty')}</span>
              )}
            </div>
          </div>
          <div className='flex items-center justify-between gap-4 border p-3 md:col-span-2'>
            <div>
              <Label htmlFor='personal-key-budget'>{t('personalApiKeys.fields.budget')}</Label>
              <p className='text-muted-foreground text-sm'>{t('personalApiKeys.fields.budgetDescription')}</p>
            </div>
            <Switch id='personal-key-budget' checked={form.budgetEnabled} onCheckedChange={(value) => update('budgetEnabled', value)} />
          </div>
          {form.budgetEnabled && (
            <>
              <div className='space-y-2'>
                <Label htmlFor='personal-key-currency'>{t('personalApiKeys.fields.currency')}</Label>
                <Input
                  id='personal-key-currency'
                  value={form.currency}
                  maxLength={8}
                  onChange={(event) => update('currency', event.target.value.toUpperCase())}
                />
              </div>
              {(['totalBudget', 'dailyBudget', 'monthlyBudget', 'singleRequestMax'] as const).map((field) => (
                <div key={field} className='space-y-2'>
                  <Label htmlFor={`personal-key-${field}`}>{t(`personalApiKeys.fields.${field}`)}</Label>
                  <Input
                    id={`personal-key-${field}`}
                    inputMode='decimal'
                    value={form[field]}
                    onChange={(event) => update(field, event.target.value)}
                  />
                </div>
              ))}
            </>
          )}
        </div>
        <DialogFooter>
          <Button variant='outline' onClick={onClose}>
            {t('personalApiKeys.cancel')}
          </Button>
          <Button onClick={onSubmit} disabled={!form.name.trim() || pending}>
            {pending && <Loader2 className='h-4 w-4 animate-spin' />}
            {t('personalApiKeys.save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function SecretDialog({ secret, onClose }: { secret: string | null; onClose: () => void }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);
  useEffect(() => setCopied(false), [secret]);
  const copy = async () => {
    if (!secret) return;
    await navigator.clipboard.writeText(secret);
    setCopied(true);
  };
  return (
    <Dialog open={Boolean(secret)} onOpenChange={(open) => !open && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('personalApiKeys.secret.title')}</DialogTitle>
          <DialogDescription>{t('personalApiKeys.secret.description')}</DialogDescription>
        </DialogHeader>
        <div className='flex items-center gap-2'>
          <Input readOnly value={secret ?? ''} data-testid='personal-api-key-secret' />
          <Button size='icon' variant='outline' onClick={copy} title={t('personalApiKeys.secret.copy')}>
            {copied ? <Check className='h-4 w-4' /> : <Copy className='h-4 w-4' />}
          </Button>
        </div>
        <DialogFooter>
          <Button onClick={onClose}>{t('personalApiKeys.secret.done')}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function formFromKey(key: PersonalAPIKey): KeyFormState {
  const limits = key.commercialLimits;
  return {
    name: key.name,
    status: key.status === 'disabled' ? 'disabled' : 'enabled',
    expiresAt: key.expiresAt ? toLocalDateTime(key.expiresAt) : '',
    ipAllowlist: (key.ipAllowlist ?? []).join('\n'),
    allowedModelIds: key.allowedModelIds ?? [],
    requestLimit: key.requestLimit ? String(key.requestLimit) : '',
    requestLimitWindow: key.requestLimitWindow || 'minute',
    budgetEnabled: Boolean(limits?.enabled),
    currency: limits?.currency || 'CNY',
    totalBudget: microsToAmount(limits?.totalBudgetMicros),
    dailyBudget: microsToAmount(limits?.dailyBudgetMicros),
    monthlyBudget: microsToAmount(limits?.monthlyBudgetMicros),
    singleRequestMax: microsToAmount(limits?.singleRequestMaxMicros),
  };
}

function formInput(form: KeyFormState): PersonalAPIKeyInput {
  const commercialLimits: CommercialLimits | null = form.budgetEnabled
    ? {
        enabled: true,
        currency: form.currency || 'CNY',
        totalBudgetMicros: amountToMicros(form.totalBudget),
        dailyBudgetMicros: amountToMicros(form.dailyBudget),
        monthlyBudgetMicros: amountToMicros(form.monthlyBudget),
        singleRequestMaxMicros: amountToMicros(form.singleRequestMax),
      }
    : null;
  return {
    name: form.name.trim(),
    expiresAt: form.expiresAt ? new Date(form.expiresAt).toISOString() : null,
    ipAllowlist: form.ipAllowlist
      .split(/[\n,]/)
      .map((value) => value.trim())
      .filter(Boolean),
    allowedModelIds: form.allowedModelIds,
    requestLimit: form.requestLimit ? Number(form.requestLimit) : null,
    requestLimitWindow: form.requestLimitWindow,
    commercialLimits,
  };
}

function amountToMicros(value: string) {
  if (!value.trim()) return null;
  const amount = Number(value);
  return Number.isFinite(amount) ? Math.round(amount * 1_000_000) : null;
}
function microsToAmount(value?: number | null) {
  return value == null ? '' : String(value / 1_000_000);
}
function toLocalDateTime(value: string) {
  const date = new Date(value);
  const offset = date.getTimezoneOffset() * 60_000;
  return new Date(date.getTime() - offset).toISOString().slice(0, 16);
}

function formatModelPrice(model: { currency?: string; price?: { items: unknown[] } }) {
  const items = (model.price?.items ?? []) as Array<{
    itemCode?: string;
    pricing?: { mode?: string; flatFee?: string | number; usagePerUnit?: string | number };
  }>;
  const labels = items.map((item) => {
    const value = item.pricing?.mode === 'flat_fee' ? item.pricing.flatFee : item.pricing?.usagePerUnit;
    return value == null ? item.itemCode : `${item.itemCode}: ${value}`;
  });
  return [model.currency, ...labels].filter(Boolean).join(' | ');
}
