<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import BottomNav from "@/components/BottomNav.vue";
import { createIntake, listIntakes, listProducts, undoIntake, type IntakeRecord, type Product } from "@/services/api";

function dateKey(date: Date) { return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,"0")}-${String(date.getDate()).padStart(2,"0")}`; }
const today = new Date();
const to = ref(dateKey(today));
const fromDate = new Date(today.getFullYear(), today.getMonth(), today.getDate()-29, 12);
const from = ref(dateKey(fromDate));
const items = ref<IntakeRecord[]>([]);
const products = ref<Product[]>([]);
const loading = ref(true);
const busy = ref("");
const message = ref("");
const showBackfill = ref(false);
const productIndex = ref(0);
const recordDate = ref(dateKey(today));
const recordTime = ref("09:00");
const quantity = ref(1);
const note = ref("");
const groups = computed(() => {
  const grouped = new Map<string, IntakeRecord[]>();
  for (const item of items.value) grouped.set(item.date, [...(grouped.get(item.date) ?? []), item]);
  return [...grouped.entries()].map(([date, records]) => ({ date, records }));
});

async function refresh() {
  loading.value = true; message.value = "";
  try { [items.value, products.value] = await Promise.all([listIntakes(from.value, to.value), listProducts()]); }
  catch (error) { message.value = error instanceof Error ? error.message : "服用记录加载失败。"; }
  finally { loading.value = false; }
}
onMounted(refresh);
async function submitBackfill() {
  const product = products.value[productIndex.value];
  if (!product) { message.value = "请先添加产品。"; return; }
  busy.value = "create"; message.value = "";
  try {
    await createIntake({ productId: product.id, date: recordDate.value, time: recordTime.value, quantity: Number(quantity.value), source: recordDate.value === to.value ? "ad_hoc" : "backfill", note: note.value }, `manual:${product.id}:${recordDate.value}:${Date.now()}`);
    showBackfill.value = false; note.value = ""; await refresh();
  } catch (error) { message.value = error instanceof Error ? error.message : "补记失败，库存未改变。"; }
  finally { busy.value = ""; }
}
async function undo(item: IntakeRecord) {
  busy.value = item.id; message.value = "";
  try { await undoIntake(item.id); await refresh(); }
  catch (error) { message.value = error instanceof Error ? error.message : "撤销失败。"; }
  finally { busy.value = ""; }
}
</script>

<template>
  <view class="page">
    <view class="bar"><view><text class="brand">小补Q</text><text class="bar-note">真实记录</text></view><button class="plain" @click="showBackfill=!showBackfill">{{showBackfill?'取消':'补记'}}</button></view>
    <main class="content">
      <view class="intro"><text class="title">服用记录</text><text class="subtitle">每次扣减和撤销都关联到原始库存批次。</text></view>
      <view v-if="message" class="error">{{message}}</view>
      <section v-if="showBackfill" class="editor">
        <text class="section-title">补记一次服用</text>
        <label><text>产品</text><picker :range="products.map(item=>item.name)" :value="productIndex" @change="productIndex=Number(($event as any).detail.value)"><view class="picker-value">{{products[productIndex]?.name||'暂无产品'}} ›</view></picker></label>
        <view class="grid"><label><text>日期</text><picker mode="date" :end="to" :value="recordDate" @change="recordDate=($event as any).detail.value"><view class="picker-value">{{recordDate}}</view></picker></label><label><text>时间</text><picker mode="time" :value="recordTime" @change="recordTime=($event as any).detail.value"><view class="picker-value">{{recordTime}}</view></picker></label></view>
        <label><text>数量</text><input v-model="quantity" type="digit"/></label><label><text>备注（可选）</text><input v-model.trim="note" maxlength="500" placeholder="例如：随餐"/></label>
        <button class="primary" :disabled="busy==='create'||!products.length||Number(quantity)<=0" @click="submitBackfill">确认补记并扣减库存</button>
      </section>
      <view v-if="loading" class="empty">正在读取记录…</view>
      <view v-else-if="!groups.length" class="empty"><text class="empty-title">最近 30 天没有记录</text><text>在“今天”打卡，或在这里补记。</text></view>
      <section v-for="group in groups" :key="group.date" class="day">
        <view class="day-head"><text>{{group.date}}</text><text>{{group.records.filter(item=>item.status==='active').length}} 条有效记录</text></view>
        <view v-for="item in group.records" :key="item.id" :class="['record',{'record--revoked':item.status==='revoked'}]">
          <view class="record-main"><text class="record-name">{{item.productName}}</text><text class="record-meta">{{item.time||'未记时间'}} · {{item.quantity}} {{item.productUnit}} · {{item.source==='scheduled'?'计划打卡':item.source==='backfill'?'历史补记':'临时服用'}}</text><text v-if="item.note" class="record-note">{{item.note}}</text></view>
          <text v-if="item.status==='revoked'" class="revoked">已撤销</text><button v-else class="undo" :disabled="busy===item.id" @click="undo(item)">撤销</button>
        </view>
      </section>
    </main>
    <BottomNav current="records" />
  </view>
</template>

<style lang="scss" scoped>
.page{min-height:100vh;padding-bottom:calc(116rpx + env(safe-area-inset-bottom));background:#f7f5f2;color:#292824}.bar{display:flex;align-items:center;justify-content:space-between;padding:calc(24rpx + env(safe-area-inset-top)) 34rpx 22rpx;border-bottom:1rpx solid #e6e1d9}.bar>view{display:flex;align-items:baseline;gap:14rpx}.brand{font-size:26rpx;font-weight:650}.bar-note{color:#928d84;font-size:17rpx}.plain{margin:0;padding:10rpx 0;background:transparent;color:#3f6a52;font-size:22rpx}.plain::after{display:none}.content{width:min(100%,900rpx);margin:auto;padding:48rpx 34rpx 100rpx}.intro{display:flex;flex-direction:column;margin-bottom:34rpx}.title{font-family:Georgia,"Songti SC",serif;font-size:60rpx}.subtitle{margin-top:10rpx;color:#77736b;font-size:22rpx;line-height:1.55}.editor,.empty,.error{margin-bottom:24rpx;padding:28rpx;border:1rpx solid #e3ded6;border-radius:20rpx;background:#fffefa}.editor{display:flex;flex-direction:column;gap:22rpx}.section-title{font-size:26rpx;font-weight:650}.editor label{display:flex;flex-direction:column;gap:9rpx;color:#706b63;font-size:19rpx}.editor input,.picker-value{height:76rpx;padding:0 18rpx;border:1rpx solid #ded9d1;border-radius:12rpx;background:#faf9f6;color:#292824;font-size:22rpx;line-height:76rpx}.grid{display:grid;grid-template-columns:1fr 1fr;gap:16rpx}.primary{margin:0;padding:22rpx;border-radius:14rpx;background:#47735a;color:#fffefa;font-size:22rpx}.day{margin:34rpx 0}.day-head{display:flex;justify-content:space-between;margin-bottom:10rpx;padding:0 5rpx;color:#7f7a72;font-size:19rpx}.record{display:flex;align-items:center;gap:18rpx;padding:24rpx 6rpx;border-bottom:1rpx solid #e5e0d8}.record-main{display:flex;flex:1;min-width:0;flex-direction:column;gap:6rpx}.record-name{font-size:24rpx;font-weight:620}.record-meta,.record-note{color:#89847b;font-size:19rpx;line-height:1.4}.record-note{color:#666158}.record--revoked{opacity:.52}.undo{margin:0;padding:10rpx 14rpx;border:1rpx solid #dcd7cf;border-radius:10rpx;background:transparent;color:#885c51;font-size:18rpx;line-height:1.2}.revoked{color:#8b867d;font-size:18rpx}.empty{display:flex;flex-direction:column;gap:10rpx;color:#7e7971;font-size:20rpx}.empty-title{color:#35332f;font-size:26rpx;font-weight:650}.error{border-color:#e4c3bc;background:#fbf1ef;color:#955446;font-size:20rpx}@media(min-width:780px){.page{padding-bottom:40px}.content{padding-top:70px}}
</style>
