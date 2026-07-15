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
export type ReconcileMaturity = 'pending' | 'provisional' | 'final'

export type ReconcileConfig = {
  id: number
  name: string
  provider: string
  account_id: string
  role_arn: string
  external_id_configured: boolean
  regions: string[]
  channel_mappings: Record<string, number[]>
  invocation_source: 'cloudwatch' | 's3'
  invocation_log_group: string
  invocation_s3_bucket: string
  invocation_s3_prefix: string
  cur_s3_bucket: string
  cur_s3_prefix: string
  athena_database: string
  athena_table: string
  athena_workgroup: string
  athena_output_location: string
  cost_explorer_enabled: boolean
  enabled: boolean
  schedule: string
  maturity_delay_seconds: number
  lookback_days: number
  tolerance: string
  created_at: number
  updated_at: number
}

export type ReconcileConfigInput = Omit<
  ReconcileConfig,
  'id' | 'external_id_configured' | 'created_at' | 'updated_at'
> & { external_id: string }

export type ReconcileItem = {
  id: number
  internal_request_id: string
  match_method: string
  confidence: string
  status: string
  internal_model_id: string
  upstream_model_id: string
  internal_input_tokens: number
  upstream_input_tokens: number
  internal_output_tokens: number
  upstream_output_tokens: number
  upstream_cache_read_tokens: number
  upstream_cache_write_tokens: number
  maturity: ReconcileMaturity
  last_observed_at: number
}

export type ReconcileDailySummary = {
  id: number
  day: number
  account_id: string
  region: string
  channel_id: number
  model_id: string
  operation: string
  service_tier: string
  routing_type: string
  token_category: string
  upstream_requests: number
  upstream_tokens: number
  cur_cost: string
  absolute_delta: string
  percentage_delta: string
  maturity: ReconcileMaturity
}

export type ReconcileAccountSummary = {
  id: number
  period_start: number
  period_end: number
  account_id: string
  gross_cost: string
  credits: string
  refunds: string
  tax_and_adjustments: string
  net_cost: string
  attributed_cost: string
  unattributed_cost: string
  unexplained_delta: string
  currency: string
  maturity: ReconcileMaturity
}

export type ReconcileRun = {
  id: number
  run_id: string
  config_id: number
  source: string
  status: string
  maturity: ReconcileMaturity
  period_start: number
  period_end: number
  counters: string
  error: string
  created_at: number
  updated_at: number
}

export type AccessDiagnostic = {
  capability: string
  available: boolean
  message: string
}

export type PageData<T> = {
  page: number
  page_size: number
  total: number
  items: T[]
}

export type ApiResponse<T> = {
  success: boolean
  message?: string
  data: T
}
