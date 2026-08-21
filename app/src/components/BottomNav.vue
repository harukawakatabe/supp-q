<script setup lang="ts">
type Entry = { key: string; label: string; icon: string; path: string; action?: "push" };
const props = defineProps<{ current: "today" | "records" | "cabinet" | "me" }>();
const entries: Entry[] = [
  { key: "today", label: "今日", icon: "●", path: "/pages/today/index" },
  { key: "records", label: "记录", icon: "≡", path: "/pages/records/index" },
  { key: "add", label: "添加", icon: "＋", path: "/pages/product/add", action: "push" },
  { key: "cabinet", label: "补充柜", icon: "□", path: "/pages/cabinet/index" },
  { key: "me", label: "我的", icon: "○", path: "/pages/me/index" },
];
function go(entry: Entry) {
  if (entry.key === props.current) return;
  if (entry.action === "push") uni.navigateTo({ url: entry.path });
  else uni.reLaunch({ url: entry.path });
}
</script>

<template>
  <nav class="bottom-nav" aria-label="主导航">
    <button v-for="entry in entries" :key="entry.key" :aria-label="entry.label" :class="['nav-item', { 'nav-item--active': entry.key === current, 'nav-add': entry.key === 'add' }]" @click="go(entry)">
      <text class="nav-icon">{{ entry.icon }}</text><text v-if="entry.key !== 'add'">{{ entry.label }}</text>
    </button>
  </nav>
</template>

<style lang="scss" scoped>
.bottom-nav{position:fixed;z-index:10;right:0;bottom:0;left:0;display:grid;grid-template-columns:repeat(5,1fr);align-items:center;height:calc(104rpx + env(safe-area-inset-bottom));padding:8rpx 12rpx env(safe-area-inset-bottom);border-top:1rpx solid rgba(213,208,199,.9);background:rgba(255,254,250,.94);backdrop-filter:blur(18px)}
.nav-item{display:flex;align-items:center;flex-direction:column;gap:5rpx;margin:0;padding:0;background:transparent;color:#918c83;font-size:18rpx;line-height:1.2}.nav-item::after{display:none}.nav-icon{height:30rpx;font-size:24rpx;line-height:30rpx}.nav-item--active{color:#3f6a52;font-weight:650}
.nav-add{display:grid;place-items:center;width:72rpx;height:72rpx;margin:-32rpx auto 0;border-radius:24rpx;background:#3f6a52;box-shadow:0 12rpx 26rpx rgba(49,85,64,.22);color:#fffefa}.nav-add .nav-icon{height:auto;font-size:38rpx;line-height:1}
@media (min-width:780px){.bottom-nav{right:auto;top:96px;bottom:auto;left:28px;display:flex;flex-direction:column;gap:24px;width:78px;height:auto;padding:20px 10px;border:1px solid #e2ddd5;border-radius:24px;background:rgba(255,254,250,.9)}.nav-add{order:3;margin:0}}
</style>
