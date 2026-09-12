import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { useAppStore } from '@/stores'
import VersionBadge from '@/components/common/VersionBadge.vue'

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
    Object.assign(useAppStore(), { currentVersion: '0.2.4-local.1', hasUpdate: true, dockerDeployment: true, versionWarning: '' })
  })

  it('shows the local image upgrade commands without an in-place update button', async () => {
    const wrapper = mount(VersionBadge)
    await wrapper.get('button').trigger('click')
    expect(wrapper.text()).toContain('v0.2.4-local.2')
    expect(wrapper.text()).toContain('image: jlliu0204/sub2api-local:0.2.4-local.2')
    expect(wrapper.text()).toContain('docker compose pull sub2api')
    expect(wrapper.text()).not.toContain('version.updateNow')
    expect(wrapper.text()).not.toContain('weishaw/sub2api')
    wrapper.unmount()
  })

  it('uses Docker rollback instructions without offering binary rollback', async () => {
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
    expect(wrapper.text()).not.toContain('version.rollbackConfirm')
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
})
