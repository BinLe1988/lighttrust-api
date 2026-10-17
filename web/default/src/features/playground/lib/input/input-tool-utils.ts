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
import {
  CameraIcon,
  FileIcon,
  ImageIcon,
  ScreenShareIcon,
  type LucideIcon,
} from 'lucide-react'

type AttachmentAction = {
  action: string
  icon: LucideIcon
  label: string
}

type InputToolNotice = {
  description?: string
  title: string
}

export const ATTACHMENT_ACTIONS = [
  { action: 'upload-file', icon: FileIcon, label: 'Upload file' },
  { action: 'upload-photo', icon: ImageIcon, label: 'Upload photo' },
  {
    action: 'take-screenshot',
    icon: ScreenShareIcon,
    label: 'Take screenshot',
  },
  { action: 'take-photo', icon: CameraIcon, label: 'Take photo' },
] satisfies AttachmentAction[]

export function getAttachmentActionNotice(action: string): InputToolNotice {
  switch (action) {
    case 'upload-photo':
      return {
        title: 'Upload photo',
        description: 'Select an image to upload',
      }
    case 'upload-file':
      return {
        title: 'Feature in development',
        description: action,
      }
    default:
      return {
        title: 'Feature in development',
        description: action,
      }
  }
}

export function getSearchActionNotice(): InputToolNotice {
  return {
    title: 'Search feature in development',
  }
}

/**
 * Upload an image file to the server
 * @returns The uploaded image URL, or null if failed
 */
export async function uploadImage(file: File): Promise<string | null> {
  const formData = new FormData()
  formData.append('file', file)

  try {
    const response = await fetch('/pg/upload', {
      method: 'POST',
      body: formData,
      credentials: 'include',
    })

    if (!response.ok) {
      const errorData = await response.json().catch(() => null)
      throw new Error(errorData?.message || `Upload failed with status ${response.status}`)
    }

    const data = await response.json()
    if (!data.success || !data.data?.url) {
      throw new Error(data.message || 'Upload failed')
    }

    return data.data.url
  } catch (error) {
    console.error('Image upload failed:', error)
    throw error
  }
}

/**
 * Open file picker for images and upload the selected file
 * @returns The uploaded image URL, or null if cancelled or failed
 */
export function pickAndUploadImage(): Promise<string | null> {
  return new Promise((resolve, reject) => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = 'image/jpeg,image/png,image/gif,image/webp,image/bmp,image/svg+xml'
    input.multiple = false

    input.onchange = async () => {
      const file = input.files?.[0]
      if (!file) {
        resolve(null)
        return
      }

      try {
        const url = await uploadImage(file)
        resolve(url)
      } catch (error) {
        reject(error)
      }
    }

    input.oncancel = () => {
      resolve(null)
    }

    input.click()
  })
}
