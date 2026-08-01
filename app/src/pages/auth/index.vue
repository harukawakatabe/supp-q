<script setup lang="ts">
import { ref } from "vue";
import { onLoad } from "@dcloudio/uni-app";
import {
  APIError,
  confirmPasswordReset,
  passwordLogin,
  requestEmailCode,
  requestPasswordReset,
  verifyEmailCode,
} from "@/services/api";

type Mode = "code" | "password" | "reset";
const mode = ref<Mode>("code");
const email = ref("");
const invitation = ref("");
const code = ref("");
const password = ref("");
const codeSent = ref(false);
const busy = ref(false);
const message = ref("");
const errorMessage = ref("");

onLoad((query) => {
  if (typeof query?.invitation === "string") invitation.value = query.invitation;
  if (typeof query?.email === "string") email.value = query.email;
});

function setMode(value: Mode) {
  mode.value = value;
  codeSent.value = false;
  code.value = "";
  message.value = "";
  errorMessage.value = "";
}
function goBack() { uni.navigateBack(); }

function showError(error: unknown) {
  errorMessage.value = error instanceof APIError ? error.message : "请求失败，请稍后再试。";
}

async function sendCode() {
  busy.value = true; errorMessage.value = ""; message.value = "";
  try {
    if (mode.value === "reset") await requestPasswordReset(email.value);
    else await requestEmailCode(email.value, invitation.value);
    codeSent.value = true;
    message.value = "验证码已发送。开发环境可在 Mailpit 中查看邮件。";
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}

async function submitCode() {
  busy.value = true; errorMessage.value = "";
  try {
    await verifyEmailCode(email.value, code.value, invitation.value, password.value);
    uni.reLaunch({ url: "/pages/today/index" });
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}

async function submitPassword() {
  busy.value = true; errorMessage.value = "";
  try {
    await passwordLogin(email.value, password.value);
    uni.reLaunch({ url: "/pages/today/index" });
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}

async function resetPassword() {
  busy.value = true; errorMessage.value = "";
  try {
    await confirmPasswordReset(email.value, code.value, password.value);
    setMode("password");
    message.value = "密码已重置，请使用新密码登录。";
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}
</script>

<template>
  <view class="auth-page">
    <view class="auth-header">
      <button class="back" aria-label="返回" @click="goBack">‹</button>
      <view class="brand-mark">Q</view>
      <text class="brand">小补Q</text>
    </view>

    <main class="auth-panel">
      <text class="eyebrow">SUPP Q ACCOUNT</text>
      <text class="title">进入你的补充空间</text>
      <text class="subtitle">新账户必须使用邀请；已有账户可以直接登录。进入真实账户后，演示数据不会迁移。</text>

      <view class="tabs" role="tablist">
        <button :class="['tab', { active: mode === 'code' }]" @click="setMode('code')">邮箱验证码</button>
        <button :class="['tab', { active: mode === 'password' }]" @click="setMode('password')">密码</button>
      </view>

      <view class="form">
        <label class="field">
          <text>邮箱</text>
          <input v-model.trim="email" type="text" inputmode="email" autocomplete="email" placeholder="you@example.com" />
        </label>

        <template v-if="mode === 'code'">
          <label class="field">
            <text>邀请码或邀请链接中的口令</text>
            <input v-model.trim="invitation" type="text" autocomplete="one-time-code" placeholder="已有账户可留空" />
            <text class="hint">通用邀请码和邮箱定向邀请共用同一套核销规则。</text>
          </label>
          <button class="secondary" :disabled="busy || !email" @click="sendCode">{{ codeSent ? "重新发送验证码" : "发送验证码" }}</button>
          <label v-if="codeSent" class="field">
            <text>6 位验证码</text>
            <input v-model.trim="code" type="number" maxlength="6" inputmode="numeric" placeholder="000000" />
          </label>
          <label v-if="codeSent" class="field">
            <text>同时设置密码（可选）</text>
            <input v-model="password" type="password" autocomplete="new-password" placeholder="至少 10 个字符" />
            <text class="hint">填写后，同一账户可同时使用验证码和密码登录。</text>
          </label>
          <button v-if="codeSent" class="primary" :disabled="busy || code.length !== 6" @click="submitCode">验证并进入</button>
        </template>

        <template v-else-if="mode === 'password'">
          <label class="field">
            <text>密码</text>
            <input v-model="password" type="password" autocomplete="current-password" placeholder="你的账户密码" />
          </label>
          <button class="primary" :disabled="busy || !email || !password" @click="submitPassword">登录</button>
          <button class="link" @click="setMode('reset')">忘记密码</button>
        </template>

        <template v-else>
          <text class="form-title">重置密码</text>
          <button class="secondary" :disabled="busy || !email" @click="sendCode">{{ codeSent ? "重新发送验证码" : "发送重置验证码" }}</button>
          <label v-if="codeSent" class="field"><text>验证码</text><input v-model.trim="code" type="number" maxlength="6" inputmode="numeric" /></label>
          <label v-if="codeSent" class="field"><text>新密码</text><input v-model="password" type="password" autocomplete="new-password" placeholder="至少 10 个字符" /></label>
          <button v-if="codeSent" class="primary" :disabled="busy || code.length !== 6 || password.length < 10" @click="resetPassword">重置密码</button>
          <button class="link" @click="setMode('password')">返回密码登录</button>
        </template>

        <text v-if="message" class="message">{{ message }}</text>
        <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
      </view>

      <view class="demo-note">
        <text>不登录时会继续使用当前独立演示空间。</text>
        <text>演示身份 24 小时无活动后删除；登录后立即进入清理队列。</text>
      </view>
    </main>
  </view>
</template>

<style lang="scss" scoped>
.auth-page { min-height: 100vh; padding-bottom: 80rpx; background: #f7f5f2; }
.auth-header { display: flex; align-items: center; gap: 16rpx; height: 116rpx; padding: calc(20rpx + env(safe-area-inset-top)) 34rpx 12rpx; }
.back { width: 52rpx; height: 52rpx; margin: 0 8rpx 0 0; padding: 0; background: transparent; color: #77736b; font-size: 48rpx; line-height: 46rpx; }
.brand-mark { display: grid; place-items: center; width: 48rpx; height: 48rpx; border-radius: 14rpx; background: #3f6a52; color: white; font-size: 24rpx; font-weight: 700; }
.brand { font-size: 25rpx; font-weight: 650; }
.auth-panel { display: flex; width: min(100%, 760rpx); margin: 0 auto; padding: 58rpx 34rpx; flex-direction: column; }
.eyebrow { color: #8c877e; font-size: 18rpx; letter-spacing: 3rpx; }
.title { margin-top: 18rpx; font-family: Georgia, "Songti SC", serif; font-size: 54rpx; line-height: 1.18; }
.subtitle { margin-top: 20rpx; color: #77736b; font-size: 22rpx; line-height: 1.65; }
.tabs { display: grid; grid-template-columns: 1fr 1fr; margin-top: 48rpx; padding: 6rpx; border-radius: 16rpx; background: #ebe8e2; }
.tab { margin: 0; padding: 17rpx; border-radius: 12rpx; background: transparent; color: #77736b; font-size: 22rpx; line-height: 1.2; }
.tab.active { background: #fffefa; color: #37352f; box-shadow: 0 3rpx 12rpx rgba(55,53,47,.06); font-weight: 650; }
.form { display: flex; margin-top: 34rpx; flex-direction: column; gap: 24rpx; }
.field { display: flex; flex-direction: column; gap: 10rpx; color: #5f5a52; font-size: 21rpx; }
.field input { height: 86rpx; padding: 0 24rpx; border: 1rpx solid #ddd8d0; border-radius: 15rpx; background: #fffefa; color: #292824; font-size: 24rpx; }
.hint { color: #918c83; font-size: 18rpx; line-height: 1.45; }
.primary,.secondary { width: 100%; margin: 0; padding: 23rpx; border-radius: 15rpx; font-size: 23rpx; line-height: 1.2; }
.primary { background: #37352f; color: #fff; }
.secondary { border: 1rpx solid #cbc6bd; background: #fffefa; color: #4e4a43; }
.primary[disabled],.secondary[disabled] { opacity: .45; }
.link { margin: 0 auto; padding: 8rpx 18rpx; background: transparent; color: #5d725f; font-size: 20rpx; line-height: 1.3; }
.form-title { font-size: 28rpx; font-weight: 650; }
.message,.error { padding: 20rpx 22rpx; border-radius: 13rpx; font-size: 20rpx; line-height: 1.5; }
.message { background: #edf3ed; color: #42634d; }
.error { background: #faeeeb; color: #974f43; }
.demo-note { display: flex; margin-top: 50rpx; padding-top: 26rpx; border-top: 1rpx solid #e1ddd6; flex-direction: column; gap: 8rpx; color: #8b867d; font-size: 18rpx; line-height: 1.5; }
@media (min-width: 780px) { .auth-panel { padding-top: 84px; } }
</style>
