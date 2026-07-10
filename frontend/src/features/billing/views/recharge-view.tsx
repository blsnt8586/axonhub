import { type FormEvent, useState } from 'react';
import { ExternalLink, Loader2, Ticket } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { useCreateMyEPayRechargeCheckout, useQuoteRechargePromo, type PromoQuote } from '../data/billing';
import { BillingSection } from '../shared';
import { microsToAmount, normalizeAmount, type BillingViewProps } from '../utils';

export function RechargeView({ currency, formatCurrency }: BillingViewProps) {
  const { t } = useTranslation();
  const [amount, setAmount] = useState('20.00');
  const [promoCode, setPromoCode] = useState('');
  const [quote, setQuote] = useState<PromoQuote | null>(null);
  const createCheckout = useCreateMyEPayRechargeCheckout();
  const quotePromo = useQuoteRechargePromo();

  const applyPromo = async () => {
    const normalized = normalizeAmount(amount);
    if (!normalized) return toast.error(t('billing.recharge.invalidAmount'));
    try {
      setQuote(await quotePromo.mutateAsync({ amount: normalized, currency, promoCode: promoCode.trim() || undefined }));
    } catch (error) {
      setQuote(null);
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const normalized = normalizeAmount(amount);
    if (!normalized) return toast.error(t('billing.recharge.invalidAmount'));
    try {
      const checkout = await createCheckout.mutateAsync({
        amount: normalized,
        currency,
        subject: t('billing.recharge.subject'),
        promoCode: promoCode.trim() || undefined,
      });
      if (!checkout.url) return toast.error(t('billing.recharge.missingCheckoutUrl'));
      window.location.assign(checkout.url);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('common.errors.unknownError'));
    }
  };

  const fallbackMicros = normalizeAmount(amount) ? Math.round(Number(normalizeAmount(amount)) * 1_000_000) : 0;
  const original = quote?.originalAmountMicros ?? fallbackMicros;
  const discount = quote?.discountAmountMicros ?? 0;
  const payable = quote?.payableAmountMicros ?? original;

  return (
    <div className='max-w-2xl' data-testid='billing-recharge-view'>
      <BillingSection title={t('billing.recharge.title')} description={t('billing.recharge.description')}>
        <form className='space-y-5' onSubmit={submit}>
          <div className='space-y-2'>
            <label className='text-sm font-medium' htmlFor='billing-recharge-amount'>
              {t('billing.recharge.amount')}
            </label>
            <Input
              id='billing-recharge-amount'
              inputMode='decimal'
              value={amount}
              onChange={(event) => {
                setAmount(event.target.value);
                setQuote(null);
              }}
              aria-invalid={amount.trim() !== '' && !normalizeAmount(amount)}
            />
            <p className='text-muted-foreground text-xs'>{t('billing.recharge.amountHint', { currency })}</p>
          </div>
          <div className='space-y-2'>
            <label className='text-sm font-medium' htmlFor='billing-recharge-promo'>
              {t('billing.promo.code')}
            </label>
            <div className='flex flex-col gap-2 sm:flex-row'>
              <Input
                id='billing-recharge-promo'
                value={promoCode}
                onChange={(event) => {
                  setPromoCode(event.target.value.toUpperCase());
                  setQuote(null);
                }}
                placeholder={t('billing.promo.placeholder')}
              />
              <Button type='button' variant='outline' onClick={applyPromo} disabled={quotePromo.isPending}>
                {quotePromo.isPending ? <Loader2 className='size-4 animate-spin' /> : <Ticket className='size-4' />}
                {t('billing.promo.apply')}
              </Button>
            </div>
          </div>
          <div className='space-y-2 border p-4 text-sm'>
            <Summary label={t('billing.promo.original')} value={formatCurrency.format(microsToAmount(original))} />
            <Summary label={t('billing.promo.discount')} value={`-${formatCurrency.format(microsToAmount(discount))}`} />
            <div className='border-t pt-2'>
              <Summary label={t('billing.promo.payable')} value={formatCurrency.format(microsToAmount(payable))} strong />
            </div>
          </div>
          <Button type='submit' disabled={createCheckout.isPending}>
            {createCheckout.isPending ? <Loader2 className='size-4 animate-spin' /> : <ExternalLink className='size-4' />}
            {t('billing.recharge.submit')}
          </Button>
        </form>
      </BillingSection>
    </div>
  );
}

function Summary({ label, value, strong = false }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className={`flex justify-between gap-4 ${strong ? 'font-semibold' : ''}`}>
      <span className='text-muted-foreground'>{label}</span>
      <span className='font-mono'>{value}</span>
    </div>
  );
}
