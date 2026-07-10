import { z } from 'zod';
import { createFileRoute, redirect } from '@tanstack/react-router';
import { getTokenFromStorage } from '@/stores/authStore';
import SignIn from '@/features/auth/sign-in';

export const Route = createFileRoute('/(auth)/sign-in')({
  validateSearch: z.object({
    redirect: z.string().optional().catch(undefined),
  }),
  beforeLoad: () => {
    if (getTokenFromStorage()) {
      throw redirect({ to: '/home' });
    }
  },
  component: SignIn,
});
