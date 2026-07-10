import { createFileRoute } from '@tanstack/react-router';
import UserHome from '@/features/user-home';

export const Route = createFileRoute('/_authenticated/home/')({
  component: UserHome,
});
