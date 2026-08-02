<script setup lang="ts">
import { computed, onMounted, ref } from "vue";
import { createIntake, getServiceHealth, getSession, getToday, logout, undoIntake, type Actor, type TodayItem } from "@/services/api";

type HealthState = "checking" | "online" | "offline";
const healthState = ref<HealthState>("checking");
const actor = ref<Actor | null>(null);
const items = ref<TodayItem[]>([]);
const loading = ref(true);
const actionId = ref("");
const message = ref("");
const today = new Date();
const dateKey = `${today.getFullYear()}-${String(today.getMonth()+1).padStart(2,"0")}-${String(today.getDate()).padStart(2,"0")}`;
const weekdays = ["星期日","星期一","星期二","星期三","星期四","星期五","星期六"];
const dateLabel = `${today.getMonth()+1}月${today.getDate()}日 · ${weekdays[today.getDay()]}`;
const doneCount = computed(()=>items.value.filter(item=>item.done).length);
const healthLabel = computed(()=>healthState.value==="online"?"本地 API 已连接":healthState.value==="offline"?"本地 API 未连接":"正在检查本地 API");

async function refresh(){
	loading.value=true;message.value="";
	try{items.value=await getToday(dateKey)}catch(error){message.value=error instanceof Error?error.message:"今天的数据加载失败。"}finally{loading.value=false}
}

onMounted(async()=>{
	const [health,session]=await Promise.allSettled([getServiceHealth(),getSession()]);
	healthState.value=health.status==="fulfilled"?"online":"offline";
	if(session.status==="fulfilled"){actor.value=session.value;await refresh()}else{message.value="无法建立演示空间，请检查本地服务。";loading.value=false}
});

async function toggle(item:TodayItem){
	actionId.value=item.product.id;message.value="";
	try{
		if(item.done&&item.lastIntakeId){await undoIntake(item.lastIntakeId)}
		else{
			const remaining=Math.max(0,item.scheduledQuantity-item.takenQuantity);
			const quantity=Math.min(item.product.doseQuantity,remaining||item.product.doseQuantity);
			const now=new Date();const time=`${String(now.getHours()).padStart(2,"0")}:${String(now.getMinutes()).padStart(2,"0")}`;
			await createIntake({productId:item.product.id,date:dateKey,time,quantity,source:"scheduled"},`${item.product.id}:${dateKey}:${item.takenQuantity}:${Date.now()}`);
		}
		await refresh();
	}catch(error){message.value=error instanceof Error?error.message:"操作失败，库存未改变。"}finally{actionId.value=""}
}

function openAuth(){uni.navigateTo({url:"/pages/auth/index"})}
function openAdmin(){uni.navigateTo({url:"/pages/admin/invitations"})}
function openCabinet(){uni.navigateTo({url:"/pages/cabinet/index"})}
function openAdd(){uni.navigateTo({url:"/pages/product/add"})}
async function signOut(){try{await logout();actor.value=await getSession();await refresh()}catch{message.value="退出失败，请稍后再试。"}}
</script>

<template>
  <view class="page-shell">
    <view class="topbar">
      <view class="brand-mark">Q</view><view class="brand-copy"><text class="brand-name">小补Q</text><text class="brand-en">SUPP Q</text></view>
      <button class="profile-button" aria-label="账户" @click="openAuth">{{actor?.kind==="registered"?"我":"演"}}</button>
    </view>
    <main class="content">
      <view class="eyebrow-row"><text class="eyebrow">{{dateLabel}}</text><view class="health-pill" :class="`health-pill--${healthState}`"><view class="health-dot"/><text>{{healthLabel}}</text></view></view>
      <view class="intro"><text class="title">今天</text><text class="subtitle">只处理今天真正需要服用的项目。</text></view>
      <view v-if="actor" class="account-strip">
        <view class="account-main"><text class="account-title">{{actor.kind==="demo_ephemeral"?"独立演示空间":actor.email}}</text><text class="account-meta">{{actor.kind==="demo_ephemeral"?"24 小时无活动后自动删除；演示修改不会迁移":"真实空间 · 数据与演示空间隔离"}}</text></view>
        <button v-if="actor.kind==='demo_ephemeral'" class="text-button" @click="openAuth">登录</button><button v-else class="text-button" @click="signOut">退出</button>
      </view>
      <view v-if="actor?.role==='admin'" class="admin-entry" @click="openAdmin"><text>邀请管理</text><text>创建和撤销邀请 ›</text></view>
      <view v-if="message" class="notice notice--error"><text>{{message}}</text></view>

      <section class="section-block">
        <view class="section-heading"><view><text class="section-title">今日计划</text><text class="section-meta">三层计划交集后的结果</text></view><text class="progress">{{doneCount}} / {{items.length}}</text></view>
        <view v-if="loading" class="empty-card"><text>正在计算今天的计划…</text></view>
        <view v-else-if="items.length===0" class="empty-card"><text class="empty-title">今天没有计划</text><text>你可以休息，或去补充柜添加产品。</text></view>
        <button v-for="item in items" :key="item.product.id" class="task-card" :class="{'task-card--done':item.done}" :disabled="actionId===item.product.id||(!item.available&&!item.done)" @click="toggle(item)">
          <view class="task-check"><text v-if="item.done">✓</text></view>
          <view class="task-main"><text class="task-name">{{item.product.name}}</text><text class="task-detail">{{item.takenQuantity}} / {{item.scheduledQuantity}} {{item.product.unit}}<template v-if="!item.available&&!item.done"> · 库存不足</template></text></view>
          <text class="task-time">{{item.product.schedule.reminderTimes[0]||"--:--"}}</text>
        </button>
      </section>

      <section class="section-block">
        <view class="section-heading"><view><text class="section-title">需要留意</text><text class="section-meta">根据真实批次余额与有效期计算</text></view></view>
        <view v-for="item in items.filter(value=>value.product.expiryRisk.level==='warn'||value.product.expiryRisk.level==='danger'||value.product.currentQuantity<=value.product.dailyQuantity*value.product.restockThresholdDays)" :key="`risk-${item.product.id}`" class="attention-card" @click="openCabinet">
          <view class="attention-icon">!</view><view class="task-main"><text class="task-name">{{item.product.name}}</text><text class="task-detail">{{item.product.expiryRisk.message}} · 余 {{item.product.currentQuantity}} {{item.product.unit}}</text></view><text class="chevron">›</text>
        </view>
        <view v-if="!items.some(value=>value.product.expiryRisk.level==='warn'||value.product.expiryRisk.level==='danger'||value.product.currentQuantity<=value.product.dailyQuantity*value.product.restockThresholdDays)" class="empty-card"><text>当前没有需要处理的库存风险。</text></view>
      </section>
    </main>
    <nav class="bottom-nav" aria-label="主导航">
      <view class="nav-item nav-item--active"><text class="nav-icon">●</text><text>今日</text></view>
      <view class="nav-item nav-item--muted"><text class="nav-icon">≡</text><text>记录</text></view>
      <view class="nav-add" @click="openAdd"><text>＋</text></view>
      <view class="nav-item" @click="openCabinet"><text class="nav-icon">□</text><text>补充柜</text></view>
      <view class="nav-item" @click="openAuth"><text class="nav-icon">○</text><text>我的</text></view>
    </nav>
  </view>
</template>

<style lang="scss" scoped>
.page-shell{min-height:100vh;padding-bottom:calc(112rpx + env(safe-area-inset-bottom));background:radial-gradient(circle at 95% 0%,rgba(218,228,218,.38),transparent 32%),#f7f5f2;color:#292824}.topbar{display:flex;align-items:center;gap:18rpx;height:124rpx;padding:calc(20rpx + env(safe-area-inset-top)) 34rpx 12rpx}.brand-mark{display:grid;place-items:center;width:58rpx;height:58rpx;border-radius:17rpx;background:#3f6a52;color:#fffefa;font-size:30rpx;font-weight:700}.brand-copy{display:flex;flex:1;flex-direction:column}.brand-name{font-size:28rpx;font-weight:650}.brand-en{margin-top:2rpx;color:#8e8980;font-size:16rpx;letter-spacing:3rpx}.profile-button{display:grid;place-items:center;width:58rpx;height:58rpx;margin:0;padding:0;border-radius:50%;background:#ebe6de;color:#625e56;font-size:22rpx;line-height:1}.content{width:min(100%,920rpx);margin:0 auto;padding:42rpx 34rpx 80rpx}.eyebrow-row{display:flex;align-items:center;justify-content:space-between;gap:24rpx}.eyebrow{color:#77736b;font-size:22rpx}.health-pill{display:flex;align-items:center;gap:10rpx;padding:10rpx 18rpx;border:1rpx solid #ddd8d0;border-radius:999rpx;color:#77736b;background:rgba(255,254,250,.72);font-size:18rpx}.health-dot{width:10rpx;height:10rpx;border-radius:50%;background:#b9b4ab}.health-pill--online .health-dot{background:#4d8462}.health-pill--offline .health-dot{background:#b96955}.intro{display:flex;flex-direction:column;margin:24rpx 0 42rpx}.title{font-family:Georgia,"Songti SC",serif;font-size:68rpx;font-weight:500;letter-spacing:-2rpx}.subtitle{margin-top:10rpx;color:#77736b;font-size:25rpx}.account-strip{display:flex;align-items:center;gap:20rpx;margin-bottom:20rpx;padding:24rpx 26rpx;border:1rpx solid #d9ded8;border-radius:18rpx;background:#f0f4ef}.account-main{display:flex;flex:1;flex-direction:column;gap:6rpx;min-width:0}.account-title{overflow:hidden;color:#3f5e4b;font-size:23rpx;font-weight:650;text-overflow:ellipsis;white-space:nowrap}.account-meta{color:#6f7c72;font-size:19rpx;line-height:1.45}.text-button{flex:0 0 auto;margin:0;padding:10rpx 16rpx;border-radius:12rpx;background:transparent;color:#3f6a52;font-size:20rpx}.admin-entry{display:flex;justify-content:space-between;margin-bottom:20rpx;padding:22rpx 26rpx;border:1rpx solid #ddd8d0;border-radius:18rpx;background:#fffefa;color:#655f56;font-size:21rpx}.notice{margin-bottom:22rpx;padding:22rpx 26rpx;border-radius:18rpx;font-size:21rpx}.notice--error{border:1rpx solid #e4c3bc;background:#fbf1ef;color:#955446}.section-block{margin:44rpx 0}.section-heading{display:flex;align-items:flex-end;justify-content:space-between;margin-bottom:18rpx;padding:0 4rpx}.section-heading>view{display:flex;flex-direction:column;gap:6rpx}.section-title{font-size:28rpx;font-weight:650}.section-meta,.progress{color:#8b867d;font-size:20rpx}.task-card,.attention-card,.empty-card{display:flex;align-items:center;gap:22rpx;box-sizing:border-box;width:100%;min-height:120rpx;margin:0 0 14rpx;padding:24rpx 26rpx;border:1rpx solid #e7e2da;border-radius:20rpx;background:#fffefa;box-shadow:0 8rpx 28rpx rgba(55,50,43,.035);text-align:left}.task-card::after{display:none}.task-card[disabled]{opacity:.58}.task-card--done{background:#eff4ef}.task-check{display:grid;place-items:center;width:40rpx;height:40rpx;flex:0 0 auto;border:2rpx solid #b9b4ab;border-radius:50%;color:#fff;background:transparent;font-size:22rpx}.task-card--done .task-check{border-color:#47735a;background:#47735a}.task-main{display:flex;flex:1;flex-direction:column;gap:8rpx;min-width:0}.task-name{font-size:26rpx;font-weight:580}.task-detail{overflow:hidden;color:#878279;font-size:21rpx;line-height:1.4;text-overflow:ellipsis;white-space:nowrap}.task-time{color:#67635c;font-size:22rpx;font-variant-numeric:tabular-nums}.attention-card{background:#f2eee6;border-color:#e1dacd}.attention-icon{display:grid;place-items:center;width:40rpx;height:40rpx;border-radius:50%;background:#d8c6a6;color:#6d5734;font-size:23rpx;font-weight:700}.chevron{color:#8b867d;font-size:38rpx}.empty-card{align-items:flex-start;flex-direction:column;justify-content:center;color:#817c74;font-size:21rpx;line-height:1.5}.empty-title{color:#4d4942;font-size:25rpx;font-weight:650}.bottom-nav{position:fixed;z-index:10;right:0;bottom:0;left:0;display:grid;grid-template-columns:repeat(5,1fr);align-items:center;height:calc(104rpx + env(safe-area-inset-bottom));padding:8rpx 12rpx env(safe-area-inset-bottom);border-top:1rpx solid rgba(213,208,199,.9);background:rgba(255,254,250,.94);backdrop-filter:blur(18px)}.nav-item{display:flex;align-items:center;flex-direction:column;gap:5rpx;color:#918c83;font-size:18rpx}.nav-icon{height:30rpx;font-size:24rpx;line-height:30rpx}.nav-item--active{color:#3f6a52;font-weight:650}.nav-item--muted{opacity:.45}.nav-add{display:grid;place-items:center;width:72rpx;height:72rpx;margin:-32rpx auto 0;border-radius:24rpx;background:#3f6a52;box-shadow:0 12rpx 26rpx rgba(49,85,64,.22);color:#fffefa;font-size:38rpx}@media (min-width:780px){.page-shell{padding-bottom:48px}.topbar{height:72px;padding:14px 28px;border-bottom:1px solid rgba(221,216,208,.75)}.content{padding-top:64px}.bottom-nav{right:auto;top:96px;bottom:auto;left:28px;display:flex;flex-direction:column;gap:24px;width:78px;height:auto;padding:20px 10px;border:1px solid #e2ddd5;border-radius:24px;background:rgba(255,254,250,.9)}.nav-add{order:3;margin:0}}
</style>
