import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { useAppStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'
import { performUpdate, rollback } from '@/api/admin/system'

vi.mock('@/stores', async () => {
  const { reactive } = await import('vue')
  const app = reactive({
    versionLoading: false, currentVersion: '0.2.4-local.1', latestVersion: '0.2.4-local.2',
    hasUpdate: true, buildType: 'release', dockerDeployment: true,
    updateRepository: 'DuskLin/sub2api', updateDockerImage: 'jlliu0204/sub2api-local',
    versionWarning: '', releaseInfo: null,
    fetchVersion: vi.fn(), clearVersionCache: vi.fn()
  })
  return { useAppStore: () => app, useAuthStore: () => ({ isAdmin: true }) }
})
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/system', () => ({
  performUpdate: vi.fn(), restartService: vi.fn(), rollback: vi.fn(),
  getRollbackVersions: vi.fn().mockResolvedValue({ versions: [{ version: '0.2.4-local.1', published_at: '', html_url: '' }] })
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copied: false, copyToClipboard: vi.fn() }) }))

describe('local Docker version badge', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(performUpdate).mockResolvedValue({ message: 'Updated', need_restart: true })
    vi.mocked(rollback).mockResolvedValue({ message: 'Rolled back', need_restart: true })
    Object.assign(useAppStore(), { currentVersion: '0.2.4-local.1', hasUpdate: true, buildType: 'release', dockerDeployment: true, versionWarning: '' })
  })

  it('updates a Docker installation and offers restart after success', async () => {
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('v0.2.4-local.2')
    expect(wrapper.text()).not.toContain('docker compose pull sub2api')
    const updateButton = wrapper.findAll('button').find(b => b.text() === 'version.updateNow')!
    await updateButton.trigger('click')
    await flushPromises()
    expect(performUpdate).toHaveBeenCalledOnce()
    expect(useAppStore().clearVersionCache).toHaveBeenCalledOnce()
    expect(wrapper.text()).toContain('version.updateComplete')
    expect(wrapper.text()).toContain('version.restartNow')
    expect(wrapper.text()).not.toContain('weishaw/sub2api')
    wrapper.unmount()
  })

  it('allows online rollback and keeps the local Docker command as an alternative', async () => {
    useAppStore().hasUpdate = false
    useAppStore().currentVersion = '0.2.4-local.2'
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    const rollbackToggle = wrapper.findAll('button').find(b => b.text().includes('version.rollback'))!
    await rollbackToggle.trigger('click')
    await flushPromises()
    const version = wrapper.findAll('button').find(b => b.text().includes('v0.2.4-local.1'))!
    await version.trigger('click')
    expect(wrapper.text()).toContain('image: jlliu0204/sub2api-local:0.2.4-local.1')
    const confirm = wrapper.findAll('button').find(b => b.text() === 'version.rollbackConfirm')!
    await confirm.trigger('click')
    await flushPromises()
    expect(rollback).toHaveBeenCalledWith('0.2.4-local.1')
    expect(wrapper.text()).toContain('version.rollbackComplete')
    expect(wrapper.text()).toContain('version.restartNow')
    expect(wrapper.text()).not.toContain('install.sh')
    wrapper.unmount()
  })

  it('retains the in-place update action for binary installations', async () => {
    useAppStore().dockerDeployment = false
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('version.updateNow')
    wrapper.unmount()
  })

  it('shows an update error and lets the administrator retry', async () => {
    vi.mocked(performUpdate).mockRejectedValueOnce(new Error('Download failed'))
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    await wrapper.findAll('button').find(b => b.text() === 'version.updateNow')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Download failed')
    expect(useAppStore().clearVersionCache).not.toHaveBeenCalled()
    await wrapper.findAll('button').find(b => b.text() === 'version.retry')!.trigger('click')
    await flushPromises()
    expect(performUpdate).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('version.restartNow')
    wrapper.unmount()
  })

  it('does not offer in-place updates for source builds', async () => {
    useAppStore().buildType = 'source'
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('version.sourceModeHint')
    expect(wrapper.text()).not.toContain('version.updateNow')
    expect(performUpdate).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
