<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import {
  ArrowLeft,
  ChevronDown,
  History,
  LoaderCircle,
  LogIn,
  LogOut,
  ShieldCheck,
  Ticket,
} from '@lucide/vue'
import { useRouter } from 'vue-router'

import { ApiError } from '@/services/api'
import { useSessionStore } from '@/stores/session'

withDefaults(
  defineProps<{
    section?: string
    backTo?: string
    backLabel?: string
  }>(),
  {
    section: '',
    backTo: '',
    backLabel: '',
  },
)

const router = useRouter()
const session = useSessionStore()
const profileMenu = ref<HTMLElement | null>(null)
const menuOpen = ref(false)
const loggingOut = ref(false)
const logoutError = ref('')
const imageFailed = ref(false)

const profileInitial = computed(() => {
  const source = session.user?.name.trim() || session.user?.email.trim() || 'U'
  return Array.from(source)[0]?.toLocaleUpperCase() ?? 'U'
})

const showProfileImage = computed(
  () => Boolean(session.user?.picture) && !imageFailed.value,
)

function toggleMenu(): void {
  menuOpen.value = !menuOpen.value
  logoutError.value = ''
}

function closeMenu(event: MouseEvent): void {
  if (!profileMenu.value?.contains(event.target as Node)) menuOpen.value = false
}

function closeMenuWithEscape(event: KeyboardEvent): void {
  if (event.key === 'Escape') menuOpen.value = false
}

async function handleLogout(): Promise<void> {
  if (loggingOut.value) return

  loggingOut.value = true
  logoutError.value = ''
  try {
    await session.signOut()
    menuOpen.value = false
    await router.replace('/login')
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      menuOpen.value = false
      await router.replace('/login')
      return
    }

    logoutError.value = error instanceof ApiError ? error.message : 'ไม่สามารถออกจากระบบได้'
  } finally {
    loggingOut.value = false
  }
}

onMounted(() => {
  void session.load()
  document.addEventListener('click', closeMenu)
  document.addEventListener('keydown', closeMenuWithEscape)
})

onBeforeUnmount(() => {
  document.removeEventListener('click', closeMenu)
  document.removeEventListener('keydown', closeMenuWithEscape)
})
</script>

<template>
  <header class="site-header">
    <RouterLink class="site-brand" to="/">
      <span class="site-brand-mark"><Ticket :size="19" /></span>
      <span>CINEMA BOOKING</span>
    </RouterLink>

    <div class="header-actions">
      <RouterLink v-if="backTo" class="back-link" :to="backTo">
        <ArrowLeft :size="17" /> <span>{{ backLabel }}</span>
      </RouterLink>
      <span v-else-if="section" class="header-section">{{ section }}</span>

      <span v-if="session.loading && !session.checked" class="profile-loading" aria-label="กำลังตรวจสอบสถานะเข้าสู่ระบบ">
        <LoaderCircle :size="19" />
      </span>

      <div v-else-if="session.user" ref="profileMenu" class="profile-menu">
        <button
          class="profile-toggle"
          type="button"
          aria-haspopup="menu"
          :aria-expanded="menuOpen"
          aria-label="เปิดเมนูโปรไฟล์"
          @click.stop="toggleMenu"
        >
          <span class="avatar" aria-hidden="true">
            <img
              v-if="showProfileImage"
              :src="session.user.picture"
              :alt="session.user.name"
              referrerpolicy="no-referrer"
              @error="imageFailed = true"
            />
            <span v-else>{{ profileInitial }}</span>
          </span>
          <span class="profile-name">{{ session.user.name }}</span>
          <ChevronDown class="profile-chevron" :class="{ open: menuOpen }" :size="16" />
        </button>

        <div v-if="menuOpen" class="profile-dropdown" role="menu">
          <div class="profile-summary">
            <strong>{{ session.user.name }}</strong>
            <span>{{ session.user.email }}</span>
          </div>
          <RouterLink class="menu-item" to="/bookings" role="menuitem" @click="menuOpen = false">
            <Ticket :size="18" />
            <span>ตั๋วของฉัน</span>
          </RouterLink>
          <RouterLink class="menu-item" to="/bookings/history" role="menuitem" @click="menuOpen = false">
            <History :size="18" />
            <span>ประวัติการสั่งซื้อ</span>
          </RouterLink>
          <RouterLink
            v-if="session.user.role === 'ADMIN'"
            class="menu-item admin-item"
            to="/admin"
            role="menuitem"
            @click="menuOpen = false"
          >
            <ShieldCheck :size="18" />
            <span>จัดการระบบ</span>
          </RouterLink>
          <button class="menu-item logout-item" type="button" role="menuitem" :disabled="loggingOut" @click="handleLogout">
            <LoaderCircle v-if="loggingOut" class="spinner" :size="18" />
            <LogOut v-else :size="18" />
            <span>{{ loggingOut ? 'กำลังออกจากระบบ' : 'ออกจากระบบ' }}</span>
          </button>
          <p v-if="logoutError" class="logout-error" role="alert">{{ logoutError }}</p>
        </div>
      </div>

      <RouterLink v-else class="login-link" to="/login">
        <LogIn :size="17" />
        <span>เข้าสู่ระบบ</span>
      </RouterLink>
    </div>
  </header>
</template>

<style scoped>
.site-header {
  position: relative;
  z-index: 20;
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 72px;
  padding: 0 max(24px, calc((100% - 1180px) / 2));
  color: #f8f7f4;
  background: #191817;
  border-bottom: 1px solid #3b3733;
}

.site-brand,
.back-link,
.login-link {
  display: inline-flex;
  align-items: center;
  color: inherit;
  text-decoration: none;
}

.site-brand { gap: 11px; font-size: 15px; font-weight: 750; }
.site-brand-mark { display: grid; width: 34px; height: 34px; place-items: center; color: #f0c766; border: 1px solid #8c7544; border-radius: 4px; }
.header-actions { display: flex; align-items: center; gap: 20px; }
.header-section, .back-link { color: #bdb8b1; font-size: 15px; }
.back-link { gap: 7px; }
.profile-menu { position: relative; }
.profile-toggle { display: flex; align-items: center; gap: 9px; min-height: 44px; padding: 3px 7px 3px 3px; color: #f8f7f4; background: transparent; border: 1px solid transparent; border-radius: 4px; cursor: pointer; }
.profile-toggle:hover, .profile-toggle[aria-expanded="true"] { background: #292724; border-color: #4c4843; }
.avatar { display: grid; width: 36px; height: 36px; flex: 0 0 36px; place-items: center; overflow: hidden; color: #1d1a15; font-size: 16px; font-weight: 800; background: #f0c766; border-radius: 50%; }
.avatar img { width: 100%; height: 100%; object-fit: cover; }
.profile-name { max-width: 150px; overflow: hidden; font-size: 15px; font-weight: 650; text-overflow: ellipsis; white-space: nowrap; }
.profile-chevron { transition: transform 160ms ease; }
.profile-chevron.open { transform: rotate(180deg); }
.profile-dropdown { position: absolute; top: calc(100% + 8px); right: 0; width: 260px; padding: 7px; color: #27231f; background: #fff; border: 1px solid #d8d3cc; border-radius: 6px; box-shadow: 0 16px 36px rgba(0, 0, 0, 0.2); }
.profile-summary { display: grid; gap: 3px; padding: 11px 10px 12px; border-bottom: 1px solid #e4e0da; }
.profile-summary strong, .profile-summary span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.profile-summary strong { font-size: 15px; }
.profile-summary span { color: #756f67; font-size: 14px; }
.menu-item { display: flex; width: 100%; min-height: 42px; align-items: center; gap: 10px; padding: 0 10px; color: #37322d; font-size: 15px; text-align: left; text-decoration: none; background: transparent; border: 0; border-radius: 4px; cursor: pointer; }
.menu-item:hover { background: #f2f0ed; }
.logout-item { margin-top: 4px; color: #9a2d26; border-top: 1px solid #e4e0da; border-radius: 0 0 4px 4px; }
.admin-item { color: #765814; }
.logout-item:disabled { cursor: wait; opacity: 0.7; }
.login-link { gap: 7px; min-height: 38px; padding: 0 13px; font-size: 15px; font-weight: 700; border: 1px solid #6e665e; border-radius: 4px; }
.login-link:hover { border-color: #d1c5b8; }
.profile-loading { display: grid; width: 42px; height: 42px; place-items: center; color: #bdb8b1; }
.profile-loading svg, .spinner { animation: spin 0.85s linear infinite; }
.logout-error { margin: 5px 8px 7px; color: #9a2d26; font-size: 14px; line-height: 1.45; }

@keyframes spin { to { transform: rotate(360deg); } }

@media (max-width: 620px) {
  .site-header { padding: 0 14px; }
  .header-actions { gap: 8px; }
  .header-section, .back-link span, .profile-name, .profile-chevron { display: none; }
  .back-link { width: 34px; height: 34px; justify-content: center; }
  .profile-toggle { padding-right: 3px; }
  .profile-dropdown { position: fixed; top: 66px; right: 12px; width: min(280px, calc(100vw - 24px)); }
}

@media (max-width: 390px) {
  .login-link span { display: none; }
  .login-link { width: 38px; padding: 0; justify-content: center; }
}

@media (prefers-reduced-motion: reduce) {
  .profile-chevron, .profile-loading svg, .spinner { animation: none; transition: none; }
}
</style>
