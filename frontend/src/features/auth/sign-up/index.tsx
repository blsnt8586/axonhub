import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import AnimatedLineBackground from '../sign-in/components/animated-line-background';
import { SignUpForm } from './components/sign-up-form';

export default function SignUp() {
  const { t } = useTranslation();

  return (
    <AuthLayout>
      <AnimatedLineBackground key='optimized-layout' />
      <TwoColumnAuth
        title={t('auth.signUp.title')}
        description={t('auth.signUp.description')}
        rightMaxWidthClassName='max-w-lg'
        rightFooter={
          <p className='text-xs leading-relaxed text-slate-500 sm:text-sm'>
            {t('auth.signUp.termsPrefix')}{' '}
            <a href='/terms' className='font-medium text-slate-700 underline underline-offset-4 hover:text-slate-900'>
              {t('auth.signUp.termsOfService')}
            </a>{' '}
            {t('auth.signUp.and')}{' '}
            <a href='/privacy' className='font-medium text-slate-700 underline underline-offset-4 hover:text-slate-900'>
              {t('auth.signUp.privacyPolicy')}
            </a>
            {t('auth.signUp.termsSuffix')}
          </p>
        }
      >
        <div className='space-y-6'>
          <SignUpForm />
          <p className='text-center text-sm text-slate-600'>
            {t('auth.signUp.haveAccount')}{' '}
            <Link to='/sign-in' className='font-medium text-slate-800 underline underline-offset-4 hover:text-slate-600'>
              {t('auth.signUp.signIn')}
            </Link>
          </p>
        </div>
      </TwoColumnAuth>
    </AuthLayout>
  );
}
