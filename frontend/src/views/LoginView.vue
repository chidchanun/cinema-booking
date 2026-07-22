<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { LoaderCircle, ShieldCheck, Ticket } from '@lucide/vue'

import { ApiError, loginWithGoogle } from '@/services/api'
import { useSessionStore } from '@/stores/session'

type LoginState = 'checking' | 'ready' | 'submitting' | 'error'

const state = ref<LoginState>('checking')
const message = ref('')
const googleButton = ref<HTMLDivElement | null>(null)
const router = useRouter()
const session = useSessionStore()

const googleClientID = import.meta.env.VITE_GOOGLE_CLIENT_ID?.trim()

function loadGoogleIdentity(): Promise<void> {
  if (window.google?.accounts?.id) return Promise.resolve()

  return new Promise((resolve, reject) => {
    const existingScript = document.querySelector<HTMLScriptElement>(
      'script[data-google-identity]',
    )
    if (existingScript) {
      existingScript.addEventListener('load', () => resolve(), { once: true })
      existingScript.addEventListener('error', () => reject(new Error('Google SDK failed')), {
        once: true,
      })
      return
    }

    const script = document.createElement('script')
    script.src = 'https://accounts.google.com/gsi/client'
    script.async = true
    script.defer = true
    script.dataset.googleIdentity = 'true'
    script.onload = () => resolve()
    script.onerror = () => reject(new Error('Google SDK failed'))
    document.head.appendChild(script)
  })
}

async function handleGoogleCredential(response: GoogleCredentialResponse) {
  if (!response.credential || state.value === 'submitting') return

  state.value = 'submitting'
  message.value = ''

  try {
    const result = await loginWithGoogle(response.credential)
    session.setUser(result.user)
    await router.replace('/')
  } catch (error) {
    state.value = 'error'
    message.value =
      error instanceof ApiError ? error.message : 'ไม่สามารถเข้าสู่ระบบได้ กรุณาลองอีกครั้ง'
    renderGoogleButton()
  }
}

async function renderGoogleButton() {
  await nextTick()
  if (!googleButton.value || !window.google?.accounts?.id || !googleClientID) return

  googleButton.value.replaceChildren()
  const availableWidth = googleButton.value.clientWidth
  window.google.accounts.id.initialize({
    client_id: googleClientID,
    callback: handleGoogleCredential,
    cancel_on_tap_outside: true,
  })
  window.google.accounts.id.renderButton(googleButton.value, {
    type: 'standard',
    theme: 'outline',
    size: 'large',
    text: 'continue_with',
    shape: 'rectangular',
    logo_alignment: 'left',
    width: Math.max(220, Math.min(360, availableWidth || 360)),
  })
}

async function initializeLogin() {
  if (!googleClientID) {
    state.value = 'error'
    message.value = 'ยังไม่ได้กำหนด Google Client ID สำหรับหน้าเว็บ'
    return
  }

  try {
    await loadGoogleIdentity()
    state.value = 'ready'
    await renderGoogleButton()
  } catch {
    state.value = 'error'
    message.value = 'ไม่สามารถโหลด Google Sign-In ได้ กรุณาตรวจสอบการเชื่อมต่อ'
  }
}

onMounted(initializeLogin)
</script>

<template>
  <main class="login-shell">
    <div class="image-shade" aria-hidden="true"></div>

    <header class="brand-bar">
      <a class="brand" href="/" aria-label="Cinema Booking home">
        <span class="brand-mark"><Ticket :size="20" stroke-width="1.8" /></span>
        <span>CINEMA BOOKING</span>
      </a>
      <span class="secure-label"><ShieldCheck :size="16" /> Secure access</span>
    </header>

    <section class="login-layout" aria-labelledby="login-title">
      <div class="intro-copy">
        <p class="eyebrow">YOUR SEAT AWAITS</p>
        <h1 id="login-title">Cinema Booking</h1>
        <p class="intro-text">
          เข้าสู่ระบบเพื่อเลือกที่นั่ง จองรอบฉาย และติดตามรายการจองของคุณ
        </p>
      </div>

      <div class="login-panel">
        <div class="panel-heading">
          <p class="panel-kicker">MEMBER ACCESS</p>
          <h2>เข้าสู่ระบบ</h2>
          <p>ใช้บัญชี Google เพื่อดำเนินการต่ออย่างปลอดภัย</p>
        </div>

        <div v-if="state === 'checking' || state === 'submitting'" class="loading-state">
          <LoaderCircle class="spinner" :size="24" aria-hidden="true" />
          <span>{{ state === 'checking' ? 'กำลังตรวจสอบเซสชัน' : 'กำลังเข้าสู่ระบบ' }}</span>
        </div>

        <div
          v-show="state === 'ready' || state === 'error'"
          ref="googleButton"
          class="google-button"
        ></div>

        <p v-if="message" class="error-message" role="alert">{{ message }}</p>

        <div class="privacy-note">
          <ShieldCheck :size="17" aria-hidden="true" />
          <span>ระบบใช้ Google เพื่อตรวจสอบตัวตนเท่านั้น เราไม่เห็นรหัสผ่านของคุณ</span>
        </div>
      </div>
    </section>

    <footer class="login-footer">
      <span>© 2026 Cinema Booking</span>
      <span>Private screening access</span>
    </footer>
  </main>
</template>

<style>
:root {
  color: #f5f3ef;
  background: #11100f;
  font-family:
    Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
  font-synthesis: none;
}

* {
  box-sizing: border-box;
}

body {
  margin: 0;
  min-width: 320px;
  min-height: 100vh;
}

button,
a {
  font: inherit;
}

.login-shell {
  position: relative;
  display: grid;
  grid-template-rows: auto 1fr auto;
  min-height: 100vh;
  overflow: hidden;
  background-color: #11100f;
  background-image: url("@/assets/cinema-login.png");
  background-position: center;
  background-size: cover;
}

.image-shade {
  position: absolute;
  inset: 0;
  background: rgba(10, 9, 8, 0.62);
}

.brand-bar,
.login-layout,
.login-footer {
  position: relative;
  z-index: 1;
}

.brand-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: min(1180px, calc(100% - 64px));
  margin: 0 auto;
  padding: 28px 0;
  border-bottom: 1px solid rgba(255, 255, 255, 0.2);
}

.brand {
  display: inline-flex;
  align-items: center;
  gap: 12px;
  color: #ffffff;
  font-size: 16px;
  font-weight: 750;
  text-decoration: none;
  letter-spacing: 0;
}

.brand-mark {
  display: grid;
  width: 38px;
  height: 38px;
  place-items: center;
  color: #f3c969;
  border: 1px solid rgba(243, 201, 105, 0.66);
  border-radius: 4px;
}

.secure-label {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  color: rgba(255, 255, 255, 0.78);
  font-size: 15px;
}

.login-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 420px;
  align-items: center;
  gap: 80px;
  width: min(1180px, calc(100% - 64px));
  margin: 0 auto;
  padding: 48px 0;
}

.intro-copy {
  max-width: 620px;
}

.eyebrow,
.panel-kicker {
  margin: 0 0 14px;
  color: #f3c969;
  font-size: 14px;
  font-weight: 750;
  letter-spacing: 0;
}

.intro-copy h1 {
  max-width: 600px;
  margin: 0;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 66px;
  font-weight: 500;
  line-height: 1.02;
  letter-spacing: 0;
  text-wrap: balance;
}

.intro-text {
  max-width: 510px;
  margin: 24px 0 0;
  color: rgba(255, 255, 255, 0.82);
  font-size: 19px;
  line-height: 1.75;
}

.login-panel {
  min-height: 390px;
  padding: 40px 30px;
  color: #22201d;
  background: rgba(250, 249, 246, 0.96);
  border: 1px solid rgba(255, 255, 255, 0.5);
  border-radius: 6px;
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.34);
}

.panel-heading h2 {
  margin: 0;
  color: #181715;
  font-family: Georgia, "Times New Roman", serif;
  font-size: 36px;
  font-weight: 500;
  line-height: 1.15;
  letter-spacing: 0;
}

.panel-heading > p:last-child {
  margin: 14px 0 0;
  color: #68635c;
  font-size: 16px;
  line-height: 1.65;
}

.loading-state {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  min-height: 48px;
  margin-top: 34px;
  color: #5f5a53;
  font-size: 16px;
  border: 1px solid #d9d5cf;
  border-radius: 4px;
}

.spinner {
  margin-right: 10px;
  animation: spin 0.85s linear infinite;
}

.google-button {
  display: flex;
  min-height: 48px;
  margin-top: 34px;
  align-items: center;
  justify-content: center;
}

.error-message {
  margin: 16px 0 0;
  padding: 11px 12px;
  color: #8e211b;
  font-size: 15px;
  line-height: 1.5;
  background: #fff0ee;
  border-left: 3px solid #b7332b;
}

.privacy-note {
  display: flex;
  gap: 10px;
  margin-top: 30px;
  padding-top: 22px;
  color: #777169;
  font-size: 14px;
  line-height: 1.55;
  border-top: 1px solid #ddd9d2;
}

.privacy-note svg {
  flex: 0 0 auto;
  color: #8a7040;
}

.login-footer {
  display: flex;
  justify-content: space-between;
  width: min(1180px, calc(100% - 64px));
  margin: 0 auto;
  padding: 24px 0;
  color: rgba(255, 255, 255, 0.58);
  font-size: 14px;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@media (max-width: 820px) {
  .brand-bar,
  .login-layout,
  .login-footer {
    width: min(100% - 36px, 560px);
  }

  .secure-label {
    display: none;
  }

  .login-shell {
    min-height: 100svh;
    overflow: auto;
    background-position: center top;
  }

  .login-layout {
    grid-template-columns: 1fr;
    align-content: start;
    gap: 30px;
    padding: 38px 0 30px;
  }

  .intro-copy h1 {
    font-size: 48px;
  }

  .intro-text {
    margin-top: 16px;
    font-size: 17px;
  }

  .login-panel {
    min-height: 360px;
    padding: 32px 22px;
  }

  .login-footer {
    gap: 16px;
    padding: 20px 0;
  }

  .login-footer span:last-child {
    display: none;
  }
}

@media (max-width: 390px) {
  .brand-bar,
  .login-layout,
  .login-footer {
    width: calc(100% - 28px);
  }

  .brand-bar {
    padding: 18px 0;
  }

  .login-layout {
    padding-top: 28px;
  }

  .intro-copy h1 {
    font-size: 41px;
  }

  .login-panel {
    padding: 28px 18px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .spinner {
    animation: none;
  }
}
</style>
