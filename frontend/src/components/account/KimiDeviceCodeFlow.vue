<template>
  <div class="space-y-4">
    <div class="rounded-lg border border-pink-200 bg-pink-50 p-4 dark:border-pink-800 dark:bg-pink-900/20">
      <h4 class="mb-2 font-semibold text-pink-900 dark:text-pink-200">
        {{ t('admin.accounts.oauth.kimi.title') }}
      </h4>
      <p class="text-sm text-pink-800 dark:text-pink-300">
        {{ t('admin.accounts.oauth.kimi.followSteps') }}
      </p>
    </div>

    <div class="flex flex-wrap gap-4">
      <label class="flex cursor-pointer items-center gap-2">
        <input v-model="inputMethod" type="radio" value="device" class="text-pink-600 focus:ring-pink-500" />
        <span class="text-sm text-gray-800 dark:text-gray-200">{{ t('admin.accounts.oauth.kimi.deviceAuth') }}</span>
      </label>
      <label class="flex cursor-pointer items-center gap-2">
        <input v-model="inputMethod" type="radio" value="refresh_token" class="text-pink-600 focus:ring-pink-500" />
        <span class="text-sm text-gray-800 dark:text-gray-200">{{ t('admin.accounts.oauth.kimi.refreshTokenAuth') }}</span>
      </label>
    </div>

    <div v-if="inputMethod === 'device'" class="space-y-3">
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('admin.accounts.oauth.kimi.deviceDesc') }}</p>
      <button
        type="button"
        class="btn btn-primary"
        :disabled="loading || polling"
        @click="handleStart"
      >
        {{ polling ? t('admin.accounts.oauth.kimi.waiting') : loading ? t('admin.accounts.oauth.generating') : t('admin.accounts.oauth.kimi.startDeviceAuth') }}
      </button>
      <div v-if="userCode" class="rounded-lg border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
        <p class="text-xs font-medium uppercase tracking-wide text-gray-500">{{ t('admin.accounts.oauth.kimi.userCode') }}</p>
        <p class="mt-1 font-mono text-2xl tracking-widest text-gray-900 dark:text-white">{{ userCode }}</p>
        <a
          v-if="verificationUriComplete"
          :href="verificationUriComplete"
          target="_blank"
          rel="noopener noreferrer"
          class="mt-3 inline-flex text-sm text-pink-600 hover:underline dark:text-pink-400"
        >
          {{ t('admin.accounts.oauth.kimi.openVerification') }}
        </a>
        <p v-else-if="verificationUri" class="mt-2 text-sm text-gray-600 dark:text-gray-400">
          {{ verificationUri }}
        </p>
      </div>
    </div>

    <div v-else class="space-y-3">
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ t('admin.accounts.oauth.kimi.refreshTokenDesc') }}</p>
      <textarea
        v-model="refreshTokenInput"
        rows="4"
        class="input font-mono text-sm"
        :placeholder="t('admin.accounts.oauth.kimi.refreshTokenPlaceholder')"
      />
      <button type="button" class="btn btn-primary" :disabled="loading" @click="handleRefreshToken">
        {{ loading ? t('admin.accounts.oauth.kimi.validating') : t('admin.accounts.oauth.kimi.validateAndCreate') }}
      </button>
    </div>

    <p v-if="error" class="text-sm text-red-600 dark:text-red-400">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { KimiOAuthRegion, KimiTokenInfo } from '@/api/admin/kimi'

const props = defineProps<{
  region: KimiOAuthRegion
  proxyId?: number | null
  loading: boolean
  polling: boolean
  error: string
  userCode: string
  verificationUri: string
  verificationUriComplete: string
}>()

const emit = defineEmits<{
  start: []
  'refresh-token': [token: string]
  authorized: [token: KimiTokenInfo]
}>()

const { t } = useI18n()
const inputMethod = ref<'device' | 'refresh_token'>('device')
const refreshTokenInput = ref('')
const loading = computed(() => props.loading)
const polling = computed(() => props.polling)
const error = computed(() => props.error)
const userCode = computed(() => props.userCode)
const verificationUri = computed(() => props.verificationUri)
const verificationUriComplete = computed(() => props.verificationUriComplete)

const handleStart = () => {
  emit('start')
}

const handleRefreshToken = () => {
  emit('refresh-token', refreshTokenInput.value)
}
</script>
