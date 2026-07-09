'use client';

import { useEffect, useMemo, useState } from 'react';
import { Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import {
  useRegistrationSettings,
  useUpdateRegistrationSettings,
  type RegistrationSettings as RegistrationSettingsModel,
} from '../data/system';

const defaultForm: RegistrationSettingsModel = {
  enabled: false,
  requireApproval: false,
  createDefaultProject: false,
  createDefaultApiKey: false,
  signupGrantAmount: '0',
  defaultProjectName: 'My Project',
  defaultApiKeyName: 'Default API Key',
  rateLimitWindowSeconds: 3600,
  rateLimitMaxAttempts: 20,
};

function normalizeForm(settings: RegistrationSettingsModel): RegistrationSettingsModel {
  return {
    ...settings,
    createDefaultProject: settings.createDefaultProject || settings.createDefaultApiKey,
    signupGrantAmount: settings.signupGrantAmount?.trim() || '0',
    defaultProjectName: settings.defaultProjectName?.trim() || defaultForm.defaultProjectName,
    defaultApiKeyName: settings.defaultApiKeyName?.trim() || defaultForm.defaultApiKeyName,
    rateLimitWindowSeconds: Math.max(0, Number(settings.rateLimitWindowSeconds ?? defaultForm.rateLimitWindowSeconds)),
    rateLimitMaxAttempts: Math.max(0, Number(settings.rateLimitMaxAttempts ?? defaultForm.rateLimitMaxAttempts)),
  };
}

export function RegistrationSettings() {
  const { t } = useTranslation();
  const { data: settings, isLoading } = useRegistrationSettings();
  const updateSettings = useUpdateRegistrationSettings();
  const [formData, setFormData] = useState<RegistrationSettingsModel>(defaultForm);

  useEffect(() => {
    if (settings) {
      setFormData(normalizeForm(settings));
    }
  }, [settings]);

  const normalizedFormData = useMemo(() => normalizeForm(formData), [formData]);
  const normalizedSettings = useMemo(() => normalizeForm(settings ?? defaultForm), [settings]);
  const hasChanges = JSON.stringify(normalizedFormData) !== JSON.stringify(normalizedSettings);

  const updateField = <K extends keyof RegistrationSettingsModel>(key: K, value: RegistrationSettingsModel[K]) => {
    setFormData((previous) => {
      const next = { ...previous, [key]: value };
      if (key === 'createDefaultApiKey' && value === true) {
        next.createDefaultProject = true;
      }
      if (key === 'createDefaultProject' && value === false) {
        next.createDefaultApiKey = false;
      }
      return next;
    });
  };

  const handleSave = async () => {
    await updateSettings.mutateAsync(normalizedFormData);
  };

  if (isLoading) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.registration.title')}</CardTitle>
        <CardDescription>{t('system.registration.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div className='space-y-1'>
            <Label htmlFor='registration-enabled'>{t('system.registration.enabled.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.registration.enabled.description')}</div>
          </div>
          <Switch
            id='registration-enabled'
            checked={formData.enabled}
            onCheckedChange={(checked) => updateField('enabled', checked)}
            disabled={updateSettings.isPending}
          />
        </div>

        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div className='space-y-1'>
            <Label htmlFor='registration-require-approval'>{t('system.registration.requireApproval.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.registration.requireApproval.description')}</div>
          </div>
          <Switch
            id='registration-require-approval'
            checked={formData.requireApproval}
            onCheckedChange={(checked) => updateField('requireApproval', checked)}
            disabled={updateSettings.isPending}
          />
        </div>

        <Separator />

        <div className='grid gap-4 md:grid-cols-3'>
          <div className='space-y-2'>
            <Label htmlFor='registration-signup-grant'>{t('system.registration.signupGrantAmount.label')}</Label>
            <Input
              id='registration-signup-grant'
              inputMode='decimal'
              value={formData.signupGrantAmount}
              onChange={(event) => updateField('signupGrantAmount', event.target.value)}
              placeholder='0'
              disabled={updateSettings.isPending}
            />
            <div className='text-muted-foreground text-sm'>{t('system.registration.signupGrantAmount.description')}</div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='registration-rate-window'>{t('system.registration.rateLimitWindowSeconds.label')}</Label>
            <Input
              id='registration-rate-window'
              type='number'
              min={0}
              value={formData.rateLimitWindowSeconds}
              onChange={(event) => updateField('rateLimitWindowSeconds', Number(event.target.value))}
              disabled={updateSettings.isPending}
            />
            <div className='text-muted-foreground text-sm'>{t('system.registration.rateLimitWindowSeconds.description')}</div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='registration-rate-max'>{t('system.registration.rateLimitMaxAttempts.label')}</Label>
            <Input
              id='registration-rate-max'
              type='number'
              min={0}
              value={formData.rateLimitMaxAttempts}
              onChange={(event) => updateField('rateLimitMaxAttempts', Number(event.target.value))}
              disabled={updateSettings.isPending}
            />
            <div className='text-muted-foreground text-sm'>{t('system.registration.rateLimitMaxAttempts.description')}</div>
          </div>
        </div>

        <Separator />

        <div className='space-y-4'>
          <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
            <div className='space-y-1'>
              <Label htmlFor='registration-default-project'>{t('system.registration.createDefaultProject.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.registration.createDefaultProject.description')}</div>
            </div>
            <Switch
              id='registration-default-project'
              checked={formData.createDefaultProject}
              onCheckedChange={(checked) => updateField('createDefaultProject', checked)}
              disabled={updateSettings.isPending}
            />
          </div>

          {formData.createDefaultProject && (
            <div className='grid gap-4 md:grid-cols-2'>
              <div className='space-y-2'>
                <Label htmlFor='registration-project-name'>{t('system.registration.defaultProjectName.label')}</Label>
                <Input
                  id='registration-project-name'
                  value={formData.defaultProjectName}
                  onChange={(event) => updateField('defaultProjectName', event.target.value)}
                  disabled={updateSettings.isPending}
                />
              </div>

              <div className='space-y-2'>
                <Label htmlFor='registration-default-api-key'>{t('system.registration.createDefaultApiKey.label')}</Label>
                <div className='flex min-h-10 items-center justify-between gap-4 rounded-lg border px-3'>
                  <span className='text-muted-foreground text-sm'>{t('system.registration.createDefaultApiKey.description')}</span>
                  <Switch
                    id='registration-default-api-key'
                    checked={formData.createDefaultApiKey}
                    onCheckedChange={(checked) => updateField('createDefaultApiKey', checked)}
                    disabled={updateSettings.isPending}
                  />
                </div>
              </div>

              {formData.createDefaultApiKey && (
                <div className='space-y-2 md:col-span-2'>
                  <Label htmlFor='registration-api-key-name'>{t('system.registration.defaultApiKeyName.label')}</Label>
                  <Input
                    id='registration-api-key-name'
                    value={formData.defaultApiKeyName}
                    onChange={(event) => updateField('defaultApiKeyName', event.target.value)}
                    disabled={updateSettings.isPending}
                  />
                </div>
              )}
            </div>
          )}
        </div>

        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={!hasChanges || updateSettings.isPending} className='min-w-[100px]'>
            {updateSettings.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('system.buttons.saving')}
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                {t('system.buttons.save')}
              </>
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
