<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { APIError, createInvitation, listInvitations, revokeInvitation, type Invitation } from "@/services/api";

const kind = ref<"generic_code" | "email_bound">("generic_code");
const email = ref("");
const maxUses = ref(10);
const days = ref(7);
const items = ref<Invitation[]>([]);
const createdSecret = ref("");
const busy = ref(false);
const errorMessage = ref("");
const kindLabel = computed(() => kind.value === "generic_code" ? "通用邀请码" : "邮箱定向邀请");

function showError(error: unknown) { errorMessage.value = error instanceof APIError ? error.message : "请求失败，请稍后再试。"; }
function goBack() { uni.navigateBack(); }
async function refresh() { try { items.value = await listInvitations(); } catch (error) { showError(error); } }
onMounted(refresh);

async function submit() {
  busy.value = true; errorMessage.value = ""; createdSecret.value = "";
  try {
    const expiresAt = new Date(Date.now() + Math.max(1, days.value) * 86400000).toISOString();
    const created = await createInvitation({ kind: kind.value, email: email.value || undefined, maxUses: kind.value === "email_bound" ? 1 : maxUses.value, expiresAt });
    createdSecret.value = created.secret;
    email.value = "";
    await refresh();
  } catch (error) { showError(error); }
  finally { busy.value = false; }
}

async function revoke(id: string) { try { await revokeInvitation(id); await refresh(); } catch (error) { showError(error); } }
function state(item: Invitation) { if (item.revokedAt) return "已撤销"; if (new Date(item.expiresAt) <= new Date()) return "已过期"; if (item.useCount >= item.maxUses) return "已用完"; return "可使用"; }
</script>

<template>
  <view class="admin-page">
    <view class="topbar"><button class="back" @click="goBack">‹</button><text>邀请管理</text></view>
    <main class="content">
      <text class="title">邀请</text>
      <text class="subtitle">两种邀请共用同一套有效期、核销、撤销与审计模型。口令只在创建成功时显示一次。</text>

      <section class="create-box">
        <view class="kind-switch">
          <button :class="{ active: kind === 'generic_code' }" @click="kind = 'generic_code'">通用邀请码</button>
          <button :class="{ active: kind === 'email_bound' }" @click="kind = 'email_bound'">邮箱定向</button>
        </view>
        <label v-if="kind === 'email_bound'" class="field"><text>指定邮箱</text><input v-model.trim="email" inputmode="email" placeholder="person@example.com" /></label>
        <label v-else class="field"><text>最多使用次数</text><input v-model.number="maxUses" type="number" /></label>
        <label class="field"><text>有效天数</text><input v-model.number="days" type="number" /></label>
        <button class="primary" :disabled="busy || (kind === 'email_bound' && !email)" @click="submit">创建{{ kindLabel }}</button>
        <view v-if="createdSecret" class="secret"><text>请立即复制，之后无法再次查看</text><text selectable class="secret-value">{{ createdSecret }}</text></view>
        <text v-if="errorMessage" class="error">{{ errorMessage }}</text>
      </section>

      <section class="list">
        <view class="list-heading"><text>最近邀请</text><text>{{ items.length }} 条</text></view>
        <view v-for="item in items" :key="item.id" class="row">
          <view class="row-main">
            <text class="row-title">{{ item.kind === 'generic_code' ? '通用邀请码' : item.email }}</text>
            <text class="row-meta">{{ item.useCount }} / {{ item.maxUses }} 次 · {{ new Date(item.expiresAt).toLocaleDateString() }} 到期</text>
          </view>
          <text :class="['status', { muted: state(item) !== '可使用' }]">{{ state(item) }}</text>
          <button v-if="state(item) === '可使用'" class="revoke" @click="revoke(item.id)">撤销</button>
        </view>
        <text v-if="!items.length" class="empty">尚未创建邀请。</text>
      </section>
    </main>
  </view>
</template>

<style lang="scss" scoped>
.admin-page { min-height: 100vh; background: #f7f5f2; }
.topbar { display: flex; align-items: center; gap: 18rpx; height: 112rpx; padding: calc(20rpx + env(safe-area-inset-top)) 34rpx 12rpx; border-bottom: 1rpx solid #e5e0d8; font-size: 25rpx; font-weight: 650; }
.back { width: 52rpx; height: 52rpx; margin: 0; padding: 0; background: transparent; color: #77736b; font-size: 48rpx; line-height: 46rpx; }
.content { width: min(100%, 900rpx); margin: 0 auto; padding: 52rpx 34rpx 100rpx; }
.title { display: block; font-family: Georgia,"Songti SC",serif; font-size: 56rpx; }
.subtitle { display: block; margin-top: 14rpx; color: #77736b; font-size: 21rpx; line-height: 1.6; }
.create-box { display: flex; margin-top: 38rpx; padding: 28rpx; border: 1rpx solid #e1ddd5; border-radius: 20rpx; background: #fffefa; flex-direction: column; gap: 22rpx; }
.kind-switch { display: grid; grid-template-columns: 1fr 1fr; padding: 5rpx; border-radius: 13rpx; background: #efede8; }
.kind-switch button { margin: 0; padding: 15rpx; background: transparent; color: #77736b; font-size: 20rpx; line-height: 1.2; }
.kind-switch button.active { border-radius: 10rpx; background: #fffefa; color: #37352f; font-weight: 650; }
.field { display: flex; flex-direction: column; gap: 9rpx; color: #5f5a52; font-size: 20rpx; }
.field input { height: 78rpx; padding: 0 20rpx; border: 1rpx solid #ddd8d0; border-radius: 13rpx; background: #fff; font-size: 23rpx; }
.primary { margin: 0; padding: 21rpx; border-radius: 13rpx; background: #37352f; color: #fff; font-size: 22rpx; line-height: 1.2; }
.secret { display: flex; gap: 10rpx; padding: 20rpx; border-radius: 13rpx; background: #eef4ee; color: #506b57; flex-direction: column; font-size: 19rpx; }
.secret-value { color: #294733; font-family: ui-monospace,SFMono-Regular,Menlo,monospace; font-size: 24rpx; font-weight: 700; }
.error { padding: 18rpx; border-radius: 12rpx; background: #faeeeb; color: #974f43; font-size: 20rpx; }
.list { margin-top: 46rpx; }
.list-heading { display: flex; justify-content: space-between; margin-bottom: 15rpx; color: #77736b; font-size: 20rpx; }
.row { display: flex; align-items: center; gap: 16rpx; padding: 24rpx 5rpx; border-bottom: 1rpx solid #e5e0d8; }
.row-main { display: flex; flex: 1; min-width: 0; flex-direction: column; gap: 7rpx; }
.row-title { overflow: hidden; font-size: 23rpx; font-weight: 620; text-overflow: ellipsis; white-space: nowrap; }
.row-meta { color: #8b867d; font-size: 18rpx; }
.status { color: #3f6a52; font-size: 18rpx; }.status.muted { color: #918c83; }
.revoke { margin: 0; padding: 9rpx 13rpx; border: 1rpx solid #ded8d0; border-radius: 10rpx; background: transparent; color: #9b5c50; font-size: 18rpx; line-height: 1.2; }
.empty { display: block; padding: 36rpx 0; color: #918c83; font-size: 20rpx; text-align: center; }
</style>
