import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { IconTrash, IconRefresh } from '@tabler/icons-react';
import { useChat } from '@ai-sdk/react';
import { DefaultChatTransport } from 'ai';
import { AlertTriangle, Copy, KeyRound, MessageSquare, RefreshCcw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useAuthStore } from '@/stores/authStore';
import { useSelectedProjectId } from '@/stores/projectStore';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Textarea } from '@/components/ui/textarea';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Actions, Action } from '@/components/ai-elements/actions';
import { Conversation, ConversationContent, ConversationEmptyState, ConversationScrollButton } from '@/components/ai-elements/conversation';
import { Loader } from '@/components/ai-elements/loader';
import { Message, MessageContent } from '@/components/ai-elements/message';
import { PromptInput, PromptInputTextarea, PromptInputSubmit } from '@/components/ai-elements/prompt-input';
import { Reasoning, ReasoningTrigger, ReasoningContent } from '@/components/ai-elements/reasoning';
import { Response as UIResponse } from '@/components/ai-elements/response';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { PlaygroundBlockReason, usePlaygroundState } from './data';

export default function Playground() {
  const { t } = useTranslation();
  const [selectedKey, setSelectedKey] = useState('');
  const [model, setModel] = useState('');
  const [temperature, setTemperature] = useState(0.6);
  const [maxTokens, setMaxTokens] = useState(4096);
  const [systemPrompt, setSystemPrompt] = useState(t('playground.settings.defaultSystemPrompt'));

  // useRef hooks for direct access to current values
  const modelRef = useRef(model);
  const temperatureRef = useRef(temperature);
  const maxTokensRef = useRef(maxTokens);
  const systemPromptRef = useRef(systemPrompt);
  const selectedKeyRef = useRef(selectedKey);

  // Keep refs synchronized with state
  useEffect(() => {
    modelRef.current = model;
  }, [model]);

  useEffect(() => {
    temperatureRef.current = temperature;
  }, [temperature]);

  useEffect(() => {
    maxTokensRef.current = maxTokens;
  }, [maxTokens]);

  useEffect(() => {
    systemPromptRef.current = systemPrompt;
  }, [systemPrompt]);

  useEffect(() => {
    selectedKeyRef.current = selectedKey;
  }, [selectedKey]);

  const { accessToken } = useAuthStore((state) => state.auth);
  const selectedProjectId = useSelectedProjectId();
  const { data: playgroundState, isLoading: playgroundLoading } = usePlaygroundState(selectedProjectId);
  const [runtimeBlockReason, setRuntimeBlockReason] = useState<PlaygroundBlockReason>('none');

  const [input, setInput] = useState('');

  const { messages, sendMessage, status, setMessages, regenerate, stop } = useChat({
    transport: new DefaultChatTransport({
      api: '/admin/account/playground/chat',
      credentials: 'include',
      headers: () => ({
        Authorization: 'Bearer ' + accessToken,
      }),
      body: () => {
        return {
          model: modelRef.current,
          temperature: temperatureRef.current,
          max_tokens: maxTokensRef.current,
          system: systemPromptRef.current,
        };
      },
      fetch: async (url, init) => {
        const requestURL = new URL(typeof url === 'string' ? url : url.toString(), window.location.origin);
        requestURL.searchParams.set('projectId', selectedProjectId || '');
        requestURL.searchParams.set('keyId', selectedKeyRef.current);
        const res = await fetch(`${requestURL.pathname}${requestURL.search}`, init);
        if (!res.ok) {
          let message = res.statusText || 'Request failed';
          let code: number | undefined = res.status;
          let reason: PlaygroundBlockReason | undefined;
          try {
            const body = await res.clone().json();
            const upstream = body?.error?.message || body?.message;
            const upstreamCode = typeof body?.error?.code === 'number' ? body.error.code : undefined;
            reason = body?.reason || body?.error?.reason;
            if (upstream) message = upstream;
            if (typeof upstreamCode === 'number') code = upstreamCode;
          } catch {
            // ignore JSON parse errors
          }
          const err: any = new Error(message);
          err.status = res.status;
          err.code = code;
          err.reason = reason;
          err.response = res;
          throw err;
        }
        return res;
      },
    }),
    onError: (error) => {
      const anyErr = error as any;
      const status = anyErr?.status;
      const codeFromError = typeof anyErr?.code === 'number' ? anyErr.code : undefined;
      const reason = anyErr?.reason as PlaygroundBlockReason | undefined;
      if (reason) setRuntimeBlockReason(reason);

      // If the error includes a fetch Response, try to parse JSON body
      const response: Response | undefined = anyErr?.response instanceof Response ? anyErr.response : undefined;
      if (response) {
        response
          .clone()
          .json()
          .then((data: any) => {
            const upstreamMsg = data?.error?.message || data?.message;
            const upstreamCode = typeof data?.error?.code === 'number' ? data.error.code : undefined;
            const code = upstreamCode ?? codeFromError ?? status;
            const message = upstreamMsg || error.message || 'Unknown error';
            const title = code ? `${code} ${message}` : message;
            toast.error(title);
          })
          .catch(() => {
            const upstreamMsg = anyErr?.error?.message || anyErr?.response?.error?.message;
            const code = codeFromError ?? status;
            const message = upstreamMsg || error.message || 'Unknown error';
            const title = code ? `${code} ${message}` : message;
            toast.error(title);
          });
        return;
      }

      try {
        const upstreamMsg = anyErr?.error?.message || anyErr?.response?.error?.message;
        const code = codeFromError ?? status;
        const message = upstreamMsg || error.message || 'Unknown error';
        const title = code ? `${code} ${message}` : message;
        toast.error(title);
      } catch {
        toast.error(error.message);
      }
    },
  });
  const isLoading = status === 'submitted' || status === 'streaming';
  const hasUserMessage = useMemo(() => messages.some((m) => m.role === 'user'), [messages]);
  const effectiveBlockReason = runtimeBlockReason !== 'none' ? runtimeBlockReason : playgroundState?.blockReason || 'none';
  const canSend = Boolean(playgroundState?.canSend && selectedKey && model);

  const keyOptions = useMemo(
    () => (playgroundState?.apiKeys || []).filter((key) => key.usable).map((key) => ({ value: key.id, label: key.name })),
    [playgroundState?.apiKeys]
  );
  const selectedKeyState = playgroundState?.apiKeys.find((key) => key.id === selectedKey);
  const modelOptions = useMemo(() => {
    const allowed = selectedKeyState?.allowedModelIds || [];
    return (playgroundState?.models || [])
      .filter(
        (item) => item.modality === 'chat' && item.availability === 'available' && (allowed.length === 0 || allowed.includes(item.modelId))
      )
      .map((item) => ({ value: item.modelId, label: item.displayName || item.modelId }));
  }, [playgroundState?.models, selectedKeyState?.allowedModelIds]);
  const selectedModel = playgroundState?.models.find((item) => item.modelId === model);

  useEffect(() => {
    const current = playgroundState?.apiKeys.find((key) => key.id === selectedKey);
    if (!current?.usable) {
      setSelectedKey(playgroundState?.apiKeys.find((key) => key.usable)?.id || '');
    }
  }, [playgroundState?.apiKeys, selectedKey]);

  useEffect(() => {
    if (!modelOptions.some((option) => option.value === model)) {
      setModel(modelOptions[0]?.value || '');
    }
  }, [model, modelOptions]);

  useEffect(() => {
    setRuntimeBlockReason('none');
    setMessages([]);
  }, [selectedProjectId, setMessages]);

  // Handle form submission
  const handleSubmit = useCallback(
    (message: { text?: string }, e: React.FormEvent) => {
      e.preventDefault();
      if (isLoading || !canSend) return;
      if (message.text?.trim()) {
        setRuntimeBlockReason('none');
        sendMessage({ text: message.text });
        setInput('');
      }
    },
    [canSend, isLoading, sendMessage]
  );

  const handleClear = useCallback(() => {
    setMessages([]);
  }, [setMessages]);

  const handleRetry = useCallback(() => {
    if (messages.length === 0) return;
    setRuntimeBlockReason('none');

    // 找到最后一个助手消息的索引
    let lastAssistantIndex = -1;
    for (let i = messages.length - 1; i >= 0; i--) {
      if (messages[i].role === 'assistant') {
        lastAssistantIndex = i;
        break;
      }
    }

    if (lastAssistantIndex !== -1) {
      // 移除最后一个助手消息及其之后的所有消息
      const newMessages = messages.slice(0, lastAssistantIndex);
      setMessages(newMessages);

      // 使用 setTimeout 确保状态更新后再调用 regenerate
      setTimeout(() => {
        regenerate();
      }, 100);
    } else {
      // 如果没有助手消息，直接重新发送
      regenerate();
    }
  }, [messages, regenerate, setMessages]);

  return (
    <TooltipProvider>
      {/* {process.env.NODE_ENV === 'development' && (
        <AIDevtools
          config={{
            enabled: true,
            position: 'bottom',
            theme: 'dark',
            streamCapture: {
              enabled: true,
              endpoint: '/admin/playground/chat',
              autoConnect: true,
            },
          }}
          enabled={true}
        />
      )} */}
      <div className='bg-background flex h-screen w-full flex-col md:flex-row'>
        {/* Settings Sidebar */}

        <div className='bg-card shadow-soft border-border m-4 flex max-h-[44vh] w-auto flex-col rounded-lg border border-r md:max-h-none md:w-[340px] md:max-w-[400px] md:min-w-[280px]'>
          <div className='border-b p-4'>
            <h1 className='text-xl font-bold tracking-tight'>{t('playground.title')}</h1>
            <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>{t('playground.description')}</p>
          </div>

          <ScrollArea className='min-h-0 flex-1 p-4'>
            <div className='space-y-6'>
              {effectiveBlockReason !== 'none' && (
                <div data-testid='playground-block-state' className='border-border bg-muted/40 flex gap-3 rounded-md border p-3'>
                  <AlertTriangle className='text-muted-foreground mt-0.5 size-4 shrink-0' />
                  <div className='min-w-0'>
                    <p className='text-sm font-medium'>{t(`playground.blocks.${effectiveBlockReason}.title`)}</p>
                    <p className='text-muted-foreground mt-1 text-xs leading-5'>
                      {t(`playground.blocks.${effectiveBlockReason}.description`)}
                    </p>
                  </div>
                </div>
              )}

              <div data-testid='playground-key-select' className='space-y-3'>
                <Label htmlFor='api-key' className='flex items-center gap-2 text-xs font-semibold'>
                  <KeyRound className='size-3.5' />
                  {t('playground.settings.apiKey')}
                </Label>
                <AutoCompleteSelect
                  selectedValue={selectedKey}
                  onSelectedValueChange={setSelectedKey}
                  items={keyOptions}
                  isLoading={playgroundLoading}
                  emptyMessage={t('playground.errors.noUsableKeys')}
                  placeholder={playgroundLoading ? t('loading') : t('playground.settings.selectApiKey')}
                />
              </div>

              <div data-testid='playground-model-select' className='space-y-3'>
                <Label htmlFor='model' className='text-xs font-semibold'>
                  {t('playground.settings.model')}
                </Label>
                <AutoCompleteSelect
                  selectedValue={model}
                  onSelectedValueChange={(v) => setModel(v)}
                  items={modelOptions}
                  isLoading={playgroundLoading}
                  emptyMessage={t('playground.errors.noModelsAvailable')}
                  placeholder={playgroundLoading ? t('loading') : t('playground.settings.selectModel')}
                />
                {selectedModel && (
                  <p className='text-muted-foreground text-[10px] leading-4'>
                    {t('playground.modelSummary', {
                      modality: selectedModel.modality,
                      scope: selectedModel.priceRule?.scope || t('playground.price.unconfigured'),
                      pattern: selectedModel.priceRule?.pattern || '-',
                      currency: selectedModel.currency || '-',
                    })}
                  </p>
                )}
              </div>

              <div className='space-y-3'>
                <Label htmlFor='temperature' className='text-xs font-semibold'>
                  {t('playground.settings.temperature')}: {temperature}
                </Label>
                <div className='px-1'>
                  <Input
                    id='temperature'
                    type='range'
                    min='0'
                    max='2'
                    step='0.1'
                    value={temperature}
                    onChange={(e) => setTemperature(parseFloat(e.target.value))}
                    className='bg-muted h-2 w-full cursor-pointer appearance-none rounded-lg'
                  />
                  <div className='text-muted-foreground mt-1 flex justify-between text-[10px]'>
                    <span>0</span>
                    <span>1</span>
                    <span>2</span>
                  </div>
                </div>
              </div>

              <div className='space-y-3'>
                <Label htmlFor='maxTokens' className='text-xs font-semibold'>
                  {t('playground.settings.maxTokens')}
                </Label>
                <Input
                  id='maxTokens'
                  type='number'
                  min='1'
                  max='4000'
                  value={maxTokens}
                  onChange={(e) => setMaxTokens(parseInt(e.target.value))}
                  className='h-9'
                />
              </div>

              <div className='space-y-3'>
                <Label htmlFor='systemPrompt' className='text-xs font-semibold'>
                  {t('playground.settings.systemPrompt')}
                </Label>
                <Textarea
                  id='systemPrompt'
                  placeholder={t('playground.settings.defaultSystemPrompt')}
                  value={systemPrompt}
                  onChange={(e) => setSystemPrompt(e.target.value)}
                  rows={4}
                  className='min-h-[80px] resize-none text-sm'
                />
              </div>
            </div>
          </ScrollArea>

          <div className='space-y-2 border-t p-4'>
            <Button
              onClick={handleRetry}
              variant='outline'
              className='h-9 w-full text-xs'
              disabled={!canSend || isLoading || messages.length === 0 || messages.every((msg) => msg.role !== 'assistant')}
            >
              <IconRefresh className='mr-2 h-3 w-3' />
              {isLoading
                ? t('playground.chat.generating')
                : messages.length === 0
                  ? t('playground.chat.noMessages')
                  : messages.every((msg) => msg.role !== 'assistant')
                    ? t('playground.chat.noMessages')
                    : t('playground.chat.retry')}
            </Button>

            <Button onClick={handleClear} variant='outline' className='h-9 w-full text-xs' disabled={isLoading}>
              <IconTrash className='mr-2 h-3 w-3' />
              {t('playground.chat.clear')}
            </Button>
          </div>
        </div>

        {/* Chat Area */}
        <div className='flex flex-1 flex-col p-4'>
          <div className='shadow-soft border-border bg-card flex h-full flex-col rounded-lg border p-4 md:p-6'>
            <Conversation className='max-h-[50vh] flex-1 md:max-h-none'>
              <ConversationContent>
                {messages.length === 0 ? (
                  <ConversationEmptyState
                    icon={<MessageSquare className='size-12' />}
                    title={t('playground.chat.startConversation')}
                    description={t('playground.chat.typeMessageBelow')}
                  />
                ) : (
                  (() => {
                    const lastAssistantIndex = (() => {
                      for (let i = messages.length - 1; i >= 0; i--) {
                        if (messages[i].role === 'assistant') return i;
                      }
                      return -1;
                    })();
                    return messages.map((message, messageIndex) => {
                      const isLastAssistant = message.role === 'assistant' && messageIndex === lastAssistantIndex;
                      const textContent = message.parts
                        ?.filter((p) => p.type === 'text')
                        .map((p: any) => p.text)
                        .join('\n');
                      return (
                        <Message from={message.role} key={message.id} fullWidth={!hasUserMessage}>
                          <MessageContent>
                            {message.parts?.map((part, index) => {
                              if (part.type === 'text') {
                                return <UIResponse key={index}>{part.text}</UIResponse>;
                              }
                              if (part.type === 'reasoning') {
                                return (
                                  <Reasoning key={index} isStreaming={isLastAssistant && status === 'streaming'}>
                                    <ReasoningTrigger />
                                    <ReasoningContent>{part.text}</ReasoningContent>
                                  </Reasoning>
                                );
                              }
                              return null;
                            })}
                            {isLastAssistant && textContent ? (
                              <Actions className='mt-2'>
                                <Action onClick={() => regenerate()} label={t('playground.chat.retry')}>
                                  <RefreshCcw className='size-3' />
                                </Action>
                                <Action onClick={() => navigator.clipboard.writeText(textContent)} label={t('copy')}>
                                  <Copy className='size-3' />
                                </Action>
                              </Actions>
                            ) : null}
                          </MessageContent>
                        </Message>
                      );
                    });
                  })()
                )}
                {status === 'submitted' && <Loader />}
              </ConversationContent>
              <ConversationScrollButton />
            </Conversation>

            <PromptInput onSubmit={handleSubmit} className='relative mt-4 w-full'>
              <PromptInputTextarea
                value={input}
                placeholder={canSend ? t('playground.chat.typeMessage') : t('playground.chat.blockedPlaceholder')}
                onChange={(e) => setInput(e.currentTarget.value)}
                disabled={!canSend}
                className='pr-16'
              />
              <PromptInputSubmit
                status={status}
                disabled={status === 'ready' ? !canSend || !input.trim() : false}
                // className='absolute right-2 top-1/2 -translate-y-1/2'
                className='absolute right-3 bottom-3'
                onClick={(e) => {
                  // When not ready (submitted/streaming/error), treat click as cancel
                  if (status !== 'ready') {
                    e.preventDefault();
                    stop();
                  }
                }}
              />
            </PromptInput>
          </div>
        </div>
      </div>
    </TooltipProvider>
  );
}
