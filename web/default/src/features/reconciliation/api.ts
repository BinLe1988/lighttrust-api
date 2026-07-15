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
import { api } from '@/lib/api'

import type {
  AccessDiagnostic,
  ApiResponse,
  PageData,
  ReconcileAccountSummary,
  ReconcileConfig,
  ReconcileConfigInput,
  ReconcileDailySummary,
  ReconcileItem,
  ReconcileRun,
} from './types'

export const reconciliationKeys = {
  all: ['reconciliation'] as const,
  configs: () => [...reconciliationKeys.all, 'configs'] as const,
  items: (configId: number, page: number) =>
    [...reconciliationKeys.all, 'items', configId, page] as const,
  daily: (configId: number, page: number) =>
    [...reconciliationKeys.all, 'daily', configId, page] as const,
  accounts: (configId: number, page: number) =>
    [...reconciliationKeys.all, 'accounts', configId, page] as const,
  runs: (configId: number, page: number) =>
    [...reconciliationKeys.all, 'runs', configId, page] as const,
}

export async function listReconcileConfigs() {
  const response = await api.get<ApiResponse<ReconcileConfig[]>>(
    '/api/reconcile/configs'
  )
  return response.data.data
}

export async function saveReconcileConfig(
  value: ReconcileConfigInput,
  id?: number
) {
  const response = id
    ? await api.put<ApiResponse<ReconcileConfig>>(
        `/api/reconcile/configs/${id}`,
        value
      )
    : await api.post<ApiResponse<ReconcileConfig>>(
        '/api/reconcile/configs',
        value
      )
  return response.data.data
}

export async function deleteReconcileConfig(id: number) {
  await api.delete(`/api/reconcile/configs/${id}`)
}

export async function diagnoseReconcileConfig(id: number) {
  const response = await api.post<
    ApiResponse<Record<string, AccessDiagnostic[]>>
  >(`/api/reconcile/configs/${id}/diagnostics`)
  return response.data.data
}

export async function createReconcileRun(configId: number) {
  const response = await api.post('/api/reconcile/runs', {
    config_id: configId,
  })
  return response.data
}

export async function retryReconcileRun(runId: string) {
  const response = await api.post(
    `/api/reconcile/runs/${encodeURIComponent(runId)}/retry`
  )
  return response.data
}

async function listPage<T>(
  path: string,
  configId: number,
  page: number
): Promise<PageData<T>> {
  const response = await api.get<ApiResponse<PageData<T>>>(path, {
    params: { config_id: configId, p: page, page_size: 20 },
  })
  return response.data.data
}

export const listReconcileItems = (configId: number, page: number) =>
  listPage<ReconcileItem>('/api/reconcile/items', configId, page)

export const listReconcileDaily = (configId: number, page: number) =>
  listPage<ReconcileDailySummary>('/api/reconcile/daily', configId, page)

export const listReconcileAccounts = (configId: number, page: number) =>
  listPage<ReconcileAccountSummary>('/api/reconcile/accounts', configId, page)

export const listReconcileRuns = (configId: number, page: number) =>
  listPage<ReconcileRun>('/api/reconcile/runs', configId, page)

export async function exportReconcile(
  configId: number,
  type: 'items' | 'daily' | 'accounts'
) {
  const response = await api.get('/api/reconcile/export', {
    params: { config_id: configId, type },
    responseType: 'blob',
  })
  const url = URL.createObjectURL(response.data)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `reconciliation-${type}-${configId}.csv`
  anchor.click()
  URL.revokeObjectURL(url)
}
