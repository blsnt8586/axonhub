import { Badge } from '@/components/ui/badge';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { TableCell, TableRow } from '@/components/ui/table';

export function BillingSection({
  title,
  description,
  children,
  testId,
}: {
  title: string;
  description?: string;
  children: React.ReactNode;
  testId?: string;
}) {
  return (
    <Card className='rounded-lg' data-testid={testId}>
      <CardHeader>
        <CardTitle className='text-base'>{title}</CardTitle>
        {description && <CardDescription>{description}</CardDescription>}
      </CardHeader>
      <CardContent>{children}</CardContent>
    </Card>
  );
}

export function EmptyTableRow({
  colSpan,
  loading,
  emptyLabel,
  loadingLabel,
}: {
  colSpan: number;
  loading: boolean;
  emptyLabel: string;
  loadingLabel: string;
}) {
  return (
    <TableRow>
      <TableCell colSpan={colSpan} className='text-muted-foreground h-28 text-center'>
        {loading ? loadingLabel : emptyLabel}
      </TableCell>
    </TableRow>
  );
}

export function StateBadge({ status, success }: { status: string; success?: boolean }) {
  return <Badge variant={success ? 'default' : 'secondary'}>{status}</Badge>;
}
