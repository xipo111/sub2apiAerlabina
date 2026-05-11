import { apiClient } from './client'

export interface ImageGenerateRequest {
  model: string
  prompt: string
  size: string
  quality: string
  n: number
}

export interface ImageGenerateItem {
  url?: string
  b64_json?: string
  revised_prompt?: string
}

export interface ImageGenerateResponse {
  created?: number
  data: ImageGenerateItem[]
}

export async function generateImage(payload: ImageGenerateRequest): Promise<ImageGenerateResponse> {
  const { data } = await apiClient.post<ImageGenerateResponse>('/images/generate', payload, {
    timeout: 180000
  })
  return data
}

export const imagesAPI = {
  generate: generateImage
}

export default imagesAPI
