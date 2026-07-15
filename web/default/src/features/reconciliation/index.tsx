import { zodResolver } from '@hookform/resolvers/zod'
/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Download,
  Play,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { hasPermission } from '@/lib/admin-permissions'
import { useAuthStore } from '@/stores/auth-store'

import {
  createReconcileRun,
  deleteReconcileConfig,
  diagnoseReconcileConfig,
  exportReconcile,
  listReconcileAccounts,
  listReconcileConfigs,
  listReconcileDaily,
  listReconcileItems,
  listReconcileRuns,
  reconciliationKeys,
  retryReconcileRun,
  saveReconcileConfig,
} from './api'
import type {
  AccessDiagnostic,
  PageData,
  ReconcileAccountSummary,
  ReconcileConfig,
  ReconcileConfigInput,
  ReconcileDailySummary,
  ReconcileItem,
  ReconcileRun,
} from './types'

const EMPTY_CONFIG: ReconcileConfigInput = {
  name: '',
  provider: 'bedrock',
  account_id: '',
  role_arn: '',
  external_id: '',
  regions: ['us-east-1'],
  channel_mappings: { 'us-east-1': [] },
  invocation_source: 'cloudwatch',
  invocation_log_group: '',
  invocation_s3_bucket: '',
  invocation_s3_prefix: '',
  cur_s3_bucket: '',
  cur_s3_prefix: '',
  athena_database: '',
  athena_table: '',
  athena_workgroup: 'primary',
  athena_output_location: '',
  cost_explorer_enabled: true,
  enabled: false,
  schedule: '',
  maturity_delay_seconds: 1800,
  lookback_days: 3,
  tolerance: '0',
}

const configFormSchema = z.object({
  name: z.string().trim().min(1),
  provider: z.literal('bedrock'),
  account_id: z.string().regex(/^\d{12}$/),
  role_arn: z.string().trim().min(1),
  external_id: z.string(),
  invocation_source: z.enum(['cloudwatch', 's3']),
  invocation_log_group: z.string(),
  invocation_s3_bucket: z.string(),
  invocation_s3_prefix: z.string(),
  cur_s3_bucket: z.string(),
  cur_s3_prefix: z.string(),
  athena_database: z.string().trim().min(1),
  athena_table: z.string().trim().min(1),
  athena_workgroup: z.string().trim().min(1),
  athena_output_location: z.string().startsWith('s3://'),
  cost_explorer_enabled: z.boolean(),
  enabled: z.boolean(),
  schedule: z.string(),
  maturity_delay_seconds: z.number().int().nonnegative(),
  lookback_days: z.number().int().min(1).max(31),
  tolerance: z.string().trim().min(1),
  regions_text: z.string().trim().min(1),
  mappings_text: z.string().refine((value) => {
    try {
      const parsed: unknown = JSON.parse(value)
      return Boolean(
        parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      )
    } catch {
      return false
    }
  }),
})

type ConfigFormValues = z.infer<typeof configFormSchema>

function configFormValues(selected?: ReconcileConfig): ConfigFormValues {
  const value = selected
    ? { ...EMPTY_CONFIG, ...selected, external_id: '' }
    : EMPTY_CONFIG
  return {
    ...value,
    provider: 'bedrock',
    regions_text: value.regions.join(', '),
    mappings_text: JSON.stringify(value.channel_mappings, null, 2),
  }
}

function money(value?: string, currency = 'USD') {
  const parsed = Number(value ?? 0)
  if (!Number.isFinite(parsed)) return value ?? '-'
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'USD',
    maximumFractionDigits: 6,
  }).format(parsed)
}

function dateTime(timestamp?: number) {
  if (!timestamp) return '-'
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(new Date(timestamp * 1000))
}

function MaturityBadge({ value }: { value?: string }) {
  return (
    <Badge variant={value === 'final' ? 'secondary' : 'outline'}>
      {value || '-'}
    </Badge>
  )
}

export function Reconciliation() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const user = useAuthStore((state) => state.auth.user)
  const canOperate = hasPermission(user, 'reconcile', 'operate')
  const canConfigure = hasPermission(user, 'reconcile', 'sensitive_write')
  const canExport = hasPermission(user, 'reconcile', 'export')
  const configsQuery = useQuery({
    queryKey: reconciliationKeys.configs(),
    queryFn: listReconcileConfigs,
  })
  const [configId, setConfigId] = useState(0)
  const [diagnostics, setDiagnostics] = useState<Record<
    string,
    AccessDiagnostic[]
  > | null>(null)

  useEffect(() => {
    if (!configId && configsQuery.data?.[0]) {
      setConfigId(configsQuery.data[0].id)
    }
  }, [configId, configsQuery.data])

  const selected = configsQuery.data?.find((item) => item.id === configId)
  const invalidate = () =>
    queryClient.invalidateQueries({ queryKey: reconciliationKeys.all })
  const runMutation = useMutation({
    mutationFn: () => createReconcileRun(configId),
    onSuccess: () => {
      toast.success(t('Reconciliation task queued'))
      invalidate()
    },
  })
  const diagnosticMutation = useMutation({
    mutationFn: () => diagnoseReconcileConfig(configId),
    onSuccess: (value) => {
      setDiagnostics(value)
      toast.success(t('Access diagnostics completed'))
    },
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        <span className='inline-flex min-w-0 items-center gap-2'>
          <span className='truncate'>{t('Reconciliation')}</span>
          <Badge variant='outline'>Bedrock</Badge>
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <div className='flex flex-wrap items-center gap-2'>
          <select
            value={configId}
            onChange={(event) => {
              setConfigId(Number(event.target.value))
              setDiagnostics(null)
            }}
            className='border-input bg-background h-8 min-w-48 rounded-lg border px-2.5 text-sm'
            aria-label={t('Reconciliation configuration')}
          >
            <option value={0}>{t('Select configuration')}</option>
            {configsQuery.data?.map((config) => (
              <option key={config.id} value={config.id}>
                {config.name}
              </option>
            ))}
          </select>
          <Button
            variant='outline'
            disabled={!selected || !canOperate || diagnosticMutation.isPending}
            onClick={() => diagnosticMutation.mutate()}
          >
            <ShieldCheck data-icon='inline-start' />
            {t('Diagnose access')}
          </Button>
          <Button
            disabled={!selected || !canOperate || runMutation.isPending}
            onClick={() => runMutation.mutate()}
          >
            <Play data-icon='inline-start' />
            {t('Run reconciliation')}
          </Button>
        </div>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <Tabs defaultValue='overview'>
          <TabsList className='max-w-full overflow-x-auto' variant='line'>
            {[
              'overview',
              'requests',
              'daily',
              'accounts',
              'runs',
              'config',
            ].map((tab) => (
              <TabsTrigger key={tab} value={tab}>
                {t(
                  (
                    {
                      overview: 'Overview',
                      requests: 'Requests',
                      daily: 'Daily cost',
                      accounts: 'Account spend',
                      runs: 'Run history',
                      config: 'Configuration',
                    } as Record<string, string>
                  )[tab]
                )}
              </TabsTrigger>
            ))}
          </TabsList>
          <TabsContent value='overview' className='pt-4'>
            <Overview configId={configId} diagnostics={diagnostics} />
          </TabsContent>
          <TabsContent value='requests' className='pt-4'>
            <ItemsPanel configId={configId} canExport={canExport} />
          </TabsContent>
          <TabsContent value='daily' className='pt-4'>
            <DailyPanel configId={configId} canExport={canExport} />
          </TabsContent>
          <TabsContent value='accounts' className='pt-4'>
            <AccountsPanel configId={configId} canExport={canExport} />
          </TabsContent>
          <TabsContent value='runs' className='pt-4'>
            <RunsPanel configId={configId} canOperate={canOperate} />
          </TabsContent>
          <TabsContent value='config' className='pt-4'>
            {canConfigure ? (
              <ConfigPanel
                selected={selected}
                onSaved={(saved) => {
                  setConfigId(saved.id)
                  invalidate()
                }}
                onDeleted={() => {
                  setConfigId(0)
                  invalidate()
                }}
              />
            ) : (
              <Card>
                <CardContent className='text-muted-foreground py-12 text-center'>
                  {t(
                    'You do not have permission to edit reconciliation configuration.'
                  )}
                </CardContent>
              </Card>
            )}
          </TabsContent>
        </Tabs>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

function Overview({
  configId,
  diagnostics,
}: {
  configId: number
  diagnostics: Record<string, AccessDiagnostic[]> | null
}) {
  const { t } = useTranslation()
  const accounts = useQuery({
    queryKey: reconciliationKeys.accounts(configId, 1),
    queryFn: () => listReconcileAccounts(configId, 1),
    enabled: configId > 0,
  })
  const items = useQuery({
    queryKey: reconciliationKeys.items(configId, 1),
    queryFn: () => listReconcileItems(configId, 1),
    enabled: configId > 0,
  })
  const runs = useQuery({
    queryKey: reconciliationKeys.runs(configId, 1),
    queryFn: () => listReconcileRuns(configId, 1),
    enabled: configId > 0,
  })
  const account = accounts.data?.items[0]
  const latestRun = runs.data?.items[0]
  if (!configId) return <EmptySelection />

  const cards = [
    [t('Request records'), String(items.data?.total ?? 0)],
    [t('AWS net cost'), money(account?.net_cost, account?.currency)],
    [
      t('Unexplained difference'),
      money(account?.unexplained_delta, account?.currency),
    ],
    [t('Latest run'), latestRun?.status ?? t('No data')],
  ]
  return (
    <div className='space-y-4'>
      <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        {cards.map(([label, value]) => (
          <Card key={label} size='sm'>
            <CardHeader>
              <CardDescription>{label}</CardDescription>
              <CardTitle className='font-mono text-2xl tabular-nums'>
                {value}
              </CardTitle>
            </CardHeader>
          </Card>
        ))}
      </div>
      {diagnostics && (
        <Card>
          <CardHeader>
            <CardTitle>{t('Access diagnostics')}</CardTitle>
            <CardDescription>
              {t('Each AWS capability is checked independently.')}
            </CardDescription>
          </CardHeader>
          <CardContent className='grid gap-3 md:grid-cols-2'>
            {Object.entries(diagnostics).flatMap(([region, values]) =>
              values.map((item) => (
                <div
                  key={`${region}-${item.capability}`}
                  className='flex items-start justify-between gap-3 rounded-lg border p-3'
                >
                  <div className='min-w-0'>
                    <div className='font-medium'>{item.capability}</div>
                    <div className='text-muted-foreground truncate text-xs'>
                      {region} · {item.message}
                    </div>
                  </div>
                  <Badge variant={item.available ? 'secondary' : 'destructive'}>
                    {item.available ? t('Available') : t('Unavailable')}
                  </Badge>
                </div>
              ))
            )}
          </CardContent>
        </Card>
      )}
    </div>
  )
}

function ItemsPanel({
  configId,
  canExport,
}: {
  configId: number
  canExport: boolean
}) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: reconciliationKeys.items(configId, page),
    queryFn: () => listReconcileItems(configId, page),
    enabled: configId > 0,
  })
  if (!configId) return <EmptySelection />
  return (
    <ResultCard
      title={t('Request reconciliation')}
      description={t(
        'Model and token differences matched to Bedrock invocations.'
      )}
      exportAction={
        canExport ? () => exportReconcile(configId, 'items') : undefined
      }
    >
      <SimpleTable
        headers={[
          t('Request ID'),
          t('Status'),
          t('Match method'),
          t('Models'),
          t('Input tokens'),
          t('Output tokens'),
          t('Maturity'),
        ]}
        rows={(query.data?.items ?? []).map((item: ReconcileItem) => ({
          key: item.id,
          cells: [
            <code key='id' className='text-xs'>
              {item.internal_request_id || '-'}
            </code>,
            <Badge
              key='status'
              variant={item.status === 'matched' ? 'secondary' : 'outline'}
            >
              {item.status}
            </Badge>,
            item.match_method || '-',
            `${item.internal_model_id || '-'} → ${item.upstream_model_id || '-'}`,
            `${item.internal_input_tokens} / ${item.upstream_input_tokens}`,
            `${item.internal_output_tokens} / ${item.upstream_output_tokens}`,
            <MaturityBadge key='maturity' value={item.maturity} />,
          ],
        }))}
      />
      <Pager page={page} data={query.data} onPage={setPage} />
    </ResultCard>
  )
}

function DailyPanel({
  configId,
  canExport,
}: {
  configId: number
  canExport: boolean
}) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: reconciliationKeys.daily(configId, page),
    queryFn: () => listReconcileDaily(configId, page),
    enabled: configId > 0,
  })
  if (!configId) return <EmptySelection />
  return (
    <ResultCard
      title={t('Daily cost reconciliation')}
      description={t('Invocation usage allocated against CUR invoice cost.')}
      exportAction={
        canExport ? () => exportReconcile(configId, 'daily') : undefined
      }
    >
      <SimpleTable
        headers={[
          t('Day'),
          t('Region / channel'),
          t('Model'),
          t('Token category'),
          t('Tokens'),
          t('CUR cost'),
          t('Maturity'),
        ]}
        rows={(query.data?.items ?? []).map((item: ReconcileDailySummary) => ({
          key: item.id,
          cells: [
            dateTime(item.day),
            `${item.region} / ${item.channel_id || t('Unattributed')}`,
            item.model_id,
            item.token_category,
            String(item.upstream_tokens),
            money(item.cur_cost),
            <MaturityBadge key='maturity' value={item.maturity} />,
          ],
        }))}
      />
      <Pager page={page} data={query.data} onPage={setPage} />
    </ResultCard>
  )
}

function AccountsPanel({
  configId,
  canExport,
}: {
  configId: number
  canExport: boolean
}) {
  const { t } = useTranslation()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: reconciliationKeys.accounts(configId, page),
    queryFn: () => listReconcileAccounts(configId, page),
    enabled: configId > 0,
  })
  if (!configId) return <EmptySelection />
  return (
    <ResultCard
      title={t('Account spend reconciliation')}
      description={t(
        'Gross cost, adjustments, attribution, and unexplained difference.'
      )}
      exportAction={
        canExport ? () => exportReconcile(configId, 'accounts') : undefined
      }
    >
      <SimpleTable
        headers={[
          t('Period'),
          t('Gross cost'),
          t('Credits / refunds'),
          t('Net cost'),
          t('Attributed'),
          t('Unattributed'),
          t('Unexplained'),
          t('Maturity'),
        ]}
        rows={(query.data?.items ?? []).map(
          (item: ReconcileAccountSummary) => ({
            key: item.id,
            cells: [
              `${dateTime(item.period_start)} — ${dateTime(item.period_end)}`,
              money(item.gross_cost, item.currency),
              `${money(item.credits, item.currency)} / ${money(item.refunds, item.currency)}`,
              money(item.net_cost, item.currency),
              money(item.attributed_cost, item.currency),
              money(item.unattributed_cost, item.currency),
              money(item.unexplained_delta, item.currency),
              <MaturityBadge key='maturity' value={item.maturity} />,
            ],
          })
        )}
      />
      <Pager page={page} data={query.data} onPage={setPage} />
    </ResultCard>
  )
}

function RunsPanel({
  configId,
  canOperate,
}: {
  configId: number
  canOperate: boolean
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [page, setPage] = useState(1)
  const query = useQuery({
    queryKey: reconciliationKeys.runs(configId, page),
    queryFn: () => listReconcileRuns(configId, page),
    enabled: configId > 0,
    refetchInterval: (value) =>
      value.state.data?.items.some((item) => item.status === 'running')
        ? 8000
        : false,
  })
  const retry = useMutation({
    mutationFn: retryReconcileRun,
    onSuccess: () => {
      toast.success(t('Retry queued'))
      queryClient.invalidateQueries({ queryKey: reconciliationKeys.all })
    },
  })
  if (!configId) return <EmptySelection />
  return (
    <ResultCard
      title={t('Run history')}
      description={t('Persistent execution state and safe retry history.')}
    >
      <SimpleTable
        headers={[
          t('Run ID'),
          t('Period'),
          t('Status'),
          t('Maturity'),
          t('Updated'),
          t('Action'),
        ]}
        rows={(query.data?.items ?? []).map((item: ReconcileRun) => ({
          key: item.run_id,
          cells: [
            <code key='id' className='text-xs'>
              {item.run_id}
            </code>,
            `${dateTime(item.period_start)} — ${dateTime(item.period_end)}`,
            <Badge
              key='status'
              variant={item.status === 'failed' ? 'destructive' : 'secondary'}
            >
              {item.status}
            </Badge>,
            <MaturityBadge key='maturity' value={item.maturity} />,
            dateTime(item.updated_at),
            <Button
              key='retry'
              size='sm'
              variant='outline'
              disabled={
                !canOperate || retry.isPending || item.status === 'running'
              }
              onClick={() => retry.mutate(item.run_id)}
            >
              <RefreshCw data-icon='inline-start' />
              {t('Retry')}
            </Button>,
          ],
        }))}
      />
      <Pager page={page} data={query.data} onPage={setPage} />
    </ResultCard>
  )
}

function ConfigPanel({
  selected,
  onSaved,
  onDeleted,
}: {
  selected?: ReconcileConfig
  onSaved: (saved: ReconcileConfig) => void
  onDeleted: () => void
}) {
  const { t } = useTranslation()
  const [creating, setCreating] = useState(!selected)
  const initial = useMemo(() => configFormValues(selected), [selected])
  const form = useForm<ConfigFormValues>({
    resolver: zodResolver(configFormSchema),
    defaultValues: initial,
  })
  useEffect(() => {
    if (creating) return
    form.reset(initial)
  }, [creating, form, initial])
  const save = useMutation({
    mutationFn: (value: ConfigFormValues) => {
      const { mappings_text, regions_text, ...config } = value
      return saveReconcileConfig(
        {
          ...config,
          regions: regions_text
            .split(',')
            .map((region) => region.trim())
            .filter(Boolean),
          channel_mappings: JSON.parse(mappings_text) as Record<
            string,
            number[]
          >,
        },
        creating ? undefined : selected?.id
      )
    },
    onSuccess: (saved) => {
      toast.success(t('Configuration saved'))
      setCreating(false)
      onSaved(saved)
    },
  })
  const remove = useMutation({
    mutationFn: () => {
      if (!selected) {
        throw new Error('No reconciliation configuration selected')
      }
      return deleteReconcileConfig(selected.id)
    },
    onSuccess: () => {
      toast.success(t('Configuration deleted'))
      onDeleted()
    },
  })
  const startCreating = () => {
    setCreating(true)
    form.reset(configFormValues())
  }
  const field = (
    key: keyof ConfigFormValues,
    label: string,
    type: 'text' | 'password' | 'number' = 'text'
  ) => (
    <div className='space-y-1.5'>
      <Label htmlFor={`reconcile-${key}`}>{t(label)}</Label>
      <Input
        id={`reconcile-${key}`}
        type={type}
        {...form.register(key, { valueAsNumber: type === 'number' })}
      />
      {form.formState.errors[key] && (
        <p className='text-destructive text-xs'>{t('Invalid value')}</p>
      )}
    </div>
  )
  return (
    <form
      onSubmit={form.handleSubmit((value) => save.mutate(value))}
      noValidate
    >
      <Card>
        <CardHeader>
          <CardTitle>
            {!creating && selected
              ? t('Edit configuration')
              : t('Create configuration')}
          </CardTitle>
          <CardDescription>
            {t(
              'External ID is write-only. Leave it blank during updates to keep the current value.'
            )}
          </CardDescription>
          <CardAction>
            <div className='flex flex-wrap gap-2'>
              {!creating && (
                <Button type='button' variant='outline' onClick={startCreating}>
                  <Plus data-icon='inline-start' />
                  {t('New configuration')}
                </Button>
              )}
              {!creating && selected && (
                <Button
                  type='button'
                  variant='destructive'
                  disabled={remove.isPending}
                  onClick={() => {
                    if (
                      window.confirm(
                        t('Delete this reconciliation configuration?')
                      )
                    ) {
                      remove.mutate()
                    }
                  }}
                >
                  <Trash2 data-icon='inline-start' />
                  {t('Delete')}
                </Button>
              )}
              <Button type='submit' disabled={save.isPending}>
                <Save data-icon='inline-start' />
                {t('Save')}
              </Button>
            </div>
          </CardAction>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
            {field('name', 'Name')}
            {field('account_id', 'AWS account ID')}
            {field('role_arn', 'IAM role ARN')}
            {field('external_id', 'External ID', 'password')}
            <div className='space-y-1.5'>
              <Label htmlFor='reconcile-invocation-source'>
                {t('Invocation source')}
              </Label>
              <select
                id='reconcile-invocation-source'
                className='border-input bg-background h-8 w-full rounded-lg border px-2.5 text-sm'
                {...form.register('invocation_source')}
              >
                <option value='cloudwatch'>CloudWatch Logs</option>
                <option value='s3'>S3</option>
              </select>
            </div>
            {field('invocation_log_group', 'CloudWatch log group')}
            {field('invocation_s3_bucket', 'Invocation S3 bucket')}
            {field('invocation_s3_prefix', 'Invocation S3 prefix')}
            {field('cur_s3_bucket', 'CUR S3 bucket')}
            {field('cur_s3_prefix', 'CUR S3 prefix')}
            {field('athena_database', 'Athena database')}
            {field('athena_table', 'Athena table')}
            {field('athena_workgroup', 'Athena workgroup')}
            {field('athena_output_location', 'Athena output location')}
            {field('schedule', 'Schedule')}
            {field('lookback_days', 'Lookback days', 'number')}
            {field(
              'maturity_delay_seconds',
              'Maturity delay seconds',
              'number'
            )}
            {field('tolerance', 'Tolerance')}
          </div>
          <div className='grid gap-4 lg:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label htmlFor='reconcile-regions'>
                {t('Regions, comma separated')}
              </Label>
              <Input
                id='reconcile-regions'
                {...form.register('regions_text')}
              />
              {form.formState.errors.regions_text && (
                <p className='text-destructive text-xs'>{t('Invalid value')}</p>
              )}
            </div>
            <div className='space-y-1.5'>
              <Label htmlFor='reconcile-channel-mappings'>
                {t('Region to AWS channel IDs (JSON)')}
              </Label>
              <textarea
                id='reconcile-channel-mappings'
                className='border-input bg-background min-h-24 w-full rounded-lg border p-2.5 font-mono text-sm'
                {...form.register('mappings_text')}
              />
              {form.formState.errors.mappings_text && (
                <p className='text-destructive text-xs'>{t('Invalid value')}</p>
              )}
            </div>
          </div>
          <div className='flex flex-wrap gap-6'>
            <label className='flex items-center gap-2 text-sm'>
              <input
                type='checkbox'
                {...form.register('cost_explorer_enabled')}
              />
              {t('Enable Cost Explorer')}
            </label>
            <label className='flex items-center gap-2 text-sm'>
              <input type='checkbox' {...form.register('enabled')} />
              {t('Enable scheduled reconciliation')}
            </label>
          </div>
        </CardContent>
      </Card>
    </form>
  )
}

function ResultCard({
  title,
  description,
  exportAction,
  children,
}: {
  title: string
  description: string
  exportAction?: () => void
  children: React.ReactNode
}) {
  const { t } = useTranslation()
  return (
    <Card>
      <CardHeader>
        <CardTitle>{title}</CardTitle>
        <CardDescription>{description}</CardDescription>
        {exportAction && (
          <CardAction>
            <Button variant='outline' onClick={exportAction}>
              <Download data-icon='inline-start' />
              {t('Export CSV')}
            </Button>
          </CardAction>
        )}
      </CardHeader>
      <CardContent className='space-y-3'>{children}</CardContent>
    </Card>
  )
}

function SimpleTable({
  headers,
  rows,
}: {
  headers: string[]
  rows: Array<{ key: string | number; cells: React.ReactNode[] }>
}) {
  const { t } = useTranslation()
  return (
    <div className='overflow-x-auto rounded-lg border'>
      <Table>
        <TableHeader>
          <TableRow className='bg-muted/40'>
            {headers.map((header) => (
              <TableHead key={header} className='text-xs whitespace-nowrap'>
                {header}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.length ? (
            rows.map((row) => (
              <TableRow key={row.key}>
                {row.cells.map((cell, cellIndex) => (
                  <TableCell
                    key={headers[cellIndex]}
                    className='max-w-80 text-sm whitespace-nowrap'
                  >
                    {cell}
                  </TableCell>
                ))}
              </TableRow>
            ))
          ) : (
            <TableRow>
              <TableCell
                colSpan={headers.length}
                className='text-muted-foreground h-24 text-center'
              >
                {t('No data')}
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

function Pager<T>({
  page,
  data,
  onPage,
}: {
  page: number
  data?: PageData<T>
  onPage: (page: number) => void
}) {
  const { t } = useTranslation()
  const pages = Math.max(
    1,
    Math.ceil((data?.total ?? 0) / (data?.page_size || 20))
  )
  return (
    <div className='flex items-center justify-between gap-3'>
      <span className='text-muted-foreground text-xs'>
        {t('Total')}: {data?.total ?? 0}
      </span>
      <div className='flex items-center gap-2'>
        <Button
          size='sm'
          variant='outline'
          disabled={page <= 1}
          onClick={() => onPage(page - 1)}
        >
          {t('Previous')}
        </Button>
        <span className='font-mono text-xs'>
          {page} / {pages}
        </span>
        <Button
          size='sm'
          variant='outline'
          disabled={page >= pages}
          onClick={() => onPage(page + 1)}
        >
          {t('Next')}
        </Button>
      </div>
    </div>
  )
}

function EmptySelection() {
  const { t } = useTranslation()
  return (
    <Card>
      <CardContent className='text-muted-foreground py-12 text-center'>
        {t('Select or create a reconciliation configuration first.')}
      </CardContent>
    </Card>
  )
}
