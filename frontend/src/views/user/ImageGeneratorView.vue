<template>
  <AppLayout>
    <div class="mx-auto max-w-6xl space-y-6">
      <div class="grid gap-6 lg:grid-cols-[minmax(0,420px)_minmax(0,1fr)]">
        <section class="card">
          <form class="space-y-5 p-6" @submit.prevent="handleGenerate">
            <div class="rounded-lg border border-gray-200 bg-gray-50 px-4 py-3 dark:border-dark-700 dark:bg-dark-800/60">
              <p class="text-sm font-medium text-gray-600 dark:text-dark-300">
                {{ t('imageGenerator.currentBalance') }}
              </p>
              <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">
                ${{ balanceText }}
              </p>
            </div>

            <div>
              <label for="prompt" class="input-label">{{ t('imageGenerator.prompt') }}</label>
              <textarea
                id="prompt"
                v-model="form.prompt"
                rows="7"
                maxlength="4000"
                :disabled="loading"
                class="input mt-1 resize-y"
                :placeholder="t('imageGenerator.promptPlaceholder')"
              />
              <div class="mt-1 flex justify-between text-xs text-gray-500 dark:text-dark-400">
                <span>{{ t('imageGenerator.promptHint') }}</span>
                <span>{{ form.prompt.length }}/4000</span>
              </div>
            </div>

            <div class="grid gap-4 sm:grid-cols-2">
              <div>
                <label for="model" class="input-label">{{ t('imageGenerator.model') }}</label>
                <select id="model" v-model="form.model" :disabled="loading || modelLoading || modelOptions.length === 0" class="input mt-1">
                  <option v-for="model in modelOptions" :key="model.value" :value="model.value">
                    {{ model.label }}
                  </option>
                </select>
                <p v-if="modelError" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
                  {{ modelError }}
                </p>
                <p v-if="!modelLoading && modelOptions.length === 0" class="mt-1 text-xs text-red-600 dark:text-red-400">
                  {{ t('imageGenerator.noModels') }}
                </p>
              </div>

              <div>
                <label for="size" class="input-label">{{ t('imageGenerator.size') }}</label>
                <select id="size" v-model="form.size" :disabled="loading" class="input mt-1">
                  <option v-for="size in sizeOptions" :key="size.value" :value="size.value">
                    {{ size.label }}
                  </option>
                </select>
              </div>

              <div>
                <label for="quality" class="input-label">{{ t('imageGenerator.quality') }}</label>
                <select id="quality" v-model="form.quality" :disabled="loading" class="input mt-1">
                  <option v-for="quality in qualityOptions" :key="quality.value" :value="quality.value">
                    {{ quality.label }}
                  </option>
                </select>
              </div>

              <div>
                <label for="count" class="input-label">{{ t('imageGenerator.count') }}</label>
                <input
                  id="count"
                  v-model.number="form.n"
                  type="number"
                  min="1"
                  max="4"
                  :disabled="loading"
                  class="input mt-1"
                />
              </div>
            </div>

            <div class="rounded-lg border border-blue-200 bg-blue-50 px-4 py-3 text-sm text-blue-700 dark:border-blue-900/50 dark:bg-blue-950/30 dark:text-blue-300">
              {{ t('imageGenerator.billingNote') }}
            </div>

            <transition name="fade">
              <div
                v-if="errorMessage"
                class="rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300"
              >
                {{ errorMessage }}
              </div>
            </transition>

            <button type="submit" class="btn btn-primary w-full py-3" :disabled="!canSubmit">
              <svg v-if="loading" class="-ml-1 mr-2 h-5 w-5 animate-spin" fill="none" viewBox="0 0 24 24">
                <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
                <path
                  class="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                />
              </svg>
              <Icon v-else name="sparkles" size="md" class="mr-2" />
              {{ loading ? t('imageGenerator.generating') : t('imageGenerator.generate') }}
            </button>
          </form>
        </section>

        <section class="min-h-[520px]">
          <div
            v-if="images.length === 0"
            class="flex h-full min-h-[520px] items-center justify-center rounded-lg border border-dashed border-gray-300 bg-gray-50 dark:border-dark-700 dark:bg-dark-800/40"
          >
            <div class="text-center">
              <Icon name="sparkles" size="xl" class="mx-auto text-gray-400 dark:text-dark-500" />
              <p class="mt-3 text-sm text-gray-500 dark:text-dark-400">
                {{ loading ? t('imageGenerator.waitingResult') : t('imageGenerator.empty') }}
              </p>
            </div>
          </div>

          <div v-else class="grid gap-4 sm:grid-cols-2">
            <article v-for="(image, index) in images" :key="image.src" class="card overflow-hidden">
              <div class="aspect-square bg-gray-100 dark:bg-dark-800">
                <img :src="image.src" :alt="image.alt" class="h-full w-full object-contain" />
              </div>
              <div class="flex items-center justify-between gap-3 p-4">
                <p class="min-w-0 truncate text-sm font-medium text-gray-700 dark:text-dark-200">
                  {{ t('imageGenerator.imageLabel', { index: index + 1 }) }}
                </p>
                <button type="button" class="btn btn-secondary btn-sm" @click="downloadImage(image.src, index)">
                  <Icon name="download" size="sm" class="mr-1.5" />
                  {{ t('imageGenerator.download') }}
                </button>
              </div>
            </article>
          </div>
        </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { imagesAPI } from '@/api/images'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const authStore = useAuthStore()

const form = reactive({
  prompt: '',
  model: 'gpt-image-1.5',
  size: '1024x1024',
  quality: 'auto',
  n: 1
})

const loading = ref(false)
const modelLoading = ref(false)
const errorMessage = ref('')
const modelError = ref('')
const images = ref<Array<{ src: string; alt: string }>>([])

const fallbackModelOptions = [
  { value: 'gpt-image-1.5', label: 'gpt-image-1.5' },
  { value: 'gpt-image-1-mini', label: 'gpt-image-1-mini' }
]
const modelOptions = ref([...fallbackModelOptions])

const sizeOptions = [
  { value: '1024x1024', label: '1024 x 1024' },
  { value: '1536x1024', label: '1536 x 1024' },
  { value: '1024x1536', label: '1024 x 1536' },
  { value: '2048x1152', label: '2048 x 1152' },
  { value: '1152x2048', label: '1152 x 2048' }
]

const qualityOptions = computed(() => [
  { value: 'auto', label: t('imageGenerator.qualityAuto') },
  { value: 'low', label: t('imageGenerator.qualityLow') },
  { value: 'medium', label: t('imageGenerator.qualityMedium') },
  { value: 'high', label: t('imageGenerator.qualityHigh') }
])

const balanceText = computed(() => (authStore.user?.balance ?? 0).toFixed(2))

const canSubmit = computed(() => {
  return !loading.value && !modelLoading.value && modelOptions.value.length > 0 && form.prompt.trim().length > 0 && form.n >= 1 && form.n <= 4
})

onMounted(() => {
  void loadModelOptions()
})

async function loadModelOptions(): Promise<void> {
  modelLoading.value = true
  modelError.value = ''
  try {
    const models = await imagesAPI.listModels()
    modelOptions.value = Array.isArray(models) ? models.filter((item) => item.value) : []
    if (modelOptions.value.length > 0 && !modelOptions.value.some((item) => item.value === form.model)) {
      form.model = modelOptions.value[0].value
    }
  } catch (error: any) {
    modelOptions.value = [...fallbackModelOptions]
    form.model = modelOptions.value[0].value
    modelError.value = error?.message || t('imageGenerator.modelLoadFailed')
  } finally {
    modelLoading.value = false
  }
}

async function handleGenerate(): Promise<void> {
  if (!canSubmit.value) return
  loading.value = true
  errorMessage.value = ''
  images.value = []
  try {
    const result = await imagesAPI.generate({
      model: form.model,
      prompt: form.prompt.trim(),
      size: form.size,
      quality: form.quality,
      n: form.n
    })
    images.value = (result.data || [])
      .map((item, index) => ({
        src: item.b64_json ? `data:image/png;base64,${item.b64_json}` : item.url || '',
        alt: item.revised_prompt || `${form.prompt.slice(0, 60)} ${index + 1}`
      }))
      .filter((item) => item.src)
    await authStore.refreshUser()
  } catch (error: any) {
    errorMessage.value = error?.message || t('imageGenerator.generateFailed')
  } finally {
    loading.value = false
  }
}

function downloadImage(src: string, index: number): void {
  const link = document.createElement('a')
  link.href = src
  link.download = `image-${Date.now()}-${index + 1}.png`
  document.body.appendChild(link)
  link.click()
  link.remove()
}
</script>
