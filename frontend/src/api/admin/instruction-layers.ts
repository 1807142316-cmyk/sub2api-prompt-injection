import { apiClient } from '../client'

/** One complete instruction-layer profile; the selected scope replaces its parent. */
export interface InstructionLayerProfile {
  enabled: boolean
  prefix: string
  suffix: string
  model_suffixes: Record<string, string>
}

/** Numeric database IDs are object keys, never secret API-Key values. */
export interface InstructionLayersConfig {
  enabled: boolean
  default: InstructionLayerProfile
  users: Record<string, InstructionLayerProfile>
  api_keys: Record<string, InstructionLayerProfile>
  groups?: Record<string, InstructionLayerProfile>
}

export async function getInstructionLayers(): Promise<InstructionLayersConfig> {
  const { data } = await apiClient.get<InstructionLayersConfig>(
    '/admin/settings/instruction-layers',
  )
  return data
}

export async function updateInstructionLayers(
  config: InstructionLayersConfig,
): Promise<InstructionLayersConfig> {
  const { data } = await apiClient.put<InstructionLayersConfig>(
    '/admin/settings/instruction-layers',
    config,
  )
  return data
}
