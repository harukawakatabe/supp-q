<script setup lang="ts">
import { computed, onUnmounted, reactive, ref } from "vue";
import { confirmRecognitionSet, createProduct, getRecognitionSet, retryRecognitionJob, uploadRecognitionSet, type RecognitionJob, type RecognitionRole, type RecognitionSet } from "@/services/api";

type Mode="choose"|"capture"|"manual"|"confirm";
type SelectedImage={path:string;name:string;size:number};
const now=new Date();const today=`${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,"0")}-${String(now.getDate()).padStart(2,"0")}`;
const weekdays=[{v:1,l:"一"},{v:2,l:"二"},{v:3,l:"三"},{v:4,l:"四"},{v:5,l:"五"},{v:6,l:"六"},{v:0,l:"日"}];
const roleCopy:Record<RecognitionRole,{title:string;hint:string}>={front:{title:"产品正面",hint:"品牌、品名和包装数量"},facts:{title:"成分表",hint:"Serving Size 与每种成分含量"},expiry:{title:"有效期",hint:"EXP / Best Before 的可见证据"}};
const roles:RecognitionRole[]=["front","facts","expiry"];
const mode=ref<Mode>("choose");const images=reactive<Partial<Record<RecognitionRole,SelectedImage>>>({});const recognitionSet=ref<RecognitionSet|null>(null);const saving=ref(false);const message=ref("");let polling=true;
const openEvidence=reactive<Record<string,boolean>>({});
const form=reactive({name:"",brand:"",unit:"粒",doseQuantity:1,doseTimesPerDay:1,quantity:30,expiryDate:"",priceCny:0,reminderTime:"09:00",startDate:today,selectedWeekdays:[0,1,2,3,4,5,6] as number[],dayEnabled:false,cycleDays:28,takeDays:21,longEnabled:false,takeWeeks:6,restWeeks:4,ingredientName:"",ingredientAmount:0,ingredientUnit:"mg"});
const allSelected=computed(()=>Boolean(images.front&&images.facts&&images.expiry));
const fakeProvider=computed(()=>recognitionSet.value?.jobs.some(job=>job.provider.startsWith("fake:"))??false);
const processing=computed(()=>recognitionSet.value?.status==="processing");

onUnmounted(()=>{polling=false});
function back(){if(mode.value!=="choose"){mode.value="choose";message.value=""}else uni.navigateBack()}
function startCapture(){mode.value="capture"}
function startManual(){mode.value="manual"}
function toggleWeekday(value:number){const index=form.selectedWeekdays.indexOf(value);if(index>=0)form.selectedWeekdays.splice(index,1);else form.selectedWeekdays.push(value)}
function switchValue(event:Event){return (event as Event&{detail:{value:boolean}}).detail.value}

async function choose(role:RecognitionRole){
	try{const result=await uni.chooseImage({count:1,sizeType:["compressed"],sourceType:["album","camera"]});const selected=Array.isArray(result.tempFiles)?result.tempFiles[0]:result.tempFiles;const path="path" in selected?selected.path:URL.createObjectURL(selected as File);const name="name" in selected&&selected.name?selected.name:path.split("/").pop()||`${role}.jpg`;images[role]={path,name,size:selected.size};message.value=""}catch(error){if((error as {errMsg?:string})?.errMsg?.includes("cancel"))return;message.value="无法读取图片，请重试。"}
}

async function beginRecognition(){
	if(!allSelected.value)return;saving.value=true;message.value="";
	try{recognitionSet.value=await uploadRecognitionSet(images as Record<RecognitionRole,SelectedImage>);await pollRecognition()}catch(error){message.value=error instanceof Error?error.message:"上传失败，已选择的图片仍保留在本页。"}finally{saving.value=false}
}

async function pollRecognition(){
	if(!recognitionSet.value)return;
	const setID=recognitionSet.value.id;
	for(let attempt=0;attempt<60&&polling;attempt++){
		const current:RecognitionSet=await getRecognitionSet(setID);recognitionSet.value=current;
		if(current.status!=="processing"){prefill(current);mode.value="confirm";return}
		await new Promise(resolve=>setTimeout(resolve,1000));
	}
	message.value="识别仍在后台运行。图片和任务已经保存，可以稍后继续。";
}

function numberValue(value:unknown,fallback:number){const parsed=Number(value);return Number.isFinite(parsed)&&parsed>0?parsed:fallback}
function textValue(value:unknown){return typeof value==="string"?value:""}
function prefill(set:RecognitionSet){const fields:Record<string,unknown>={};for(const job of set.jobs){if(job.result?.fields)Object.assign(fields,job.result.fields);if(job.role==="expiry"&&job.result?.date)form.expiryDate=job.result.date}form.name=textValue(fields.productName);form.brand=textValue(fields.brand);form.unit=textValue(fields.unit)||"粒";form.quantity=numberValue(fields.count,30);form.doseQuantity=numberValue(fields.dose,1);form.doseTimesPerDay=numberValue(fields.times,1);form.reminderTime=textValue(fields.reminder)||"09:00";const raw=textValue(fields.ingredientsZh)||textValue(fields.ingredientsRaw);if(raw)form.ingredientName=raw.slice(0,80)}
async function retry(job:RecognitionJob){saving.value=true;try{recognitionSet.value=await retryRecognitionJob(job.id);mode.value="capture";await pollRecognition()}catch(error){message.value=error instanceof Error?error.message:"重试失败。"}finally{saving.value=false}}

function productPayload(){const ingredientAmount=Number(form.ingredientAmount);return{name:form.name,brand:form.brand,productType:"supplement",unit:form.unit,doseQuantity:Number(form.doseQuantity),doseTimesPerDay:Number(form.doseTimesPerDay),ingredientServingQuantity:Number(form.doseQuantity),restockThresholdDays:7,expiryReminderDays:30,schedule:{startDate:form.startDate,weekdays:form.selectedWeekdays,dayCycle:{enabled:form.dayEnabled,cycleDays:Number(form.cycleDays),takeDays:Number(form.takeDays),anchorDate:form.startDate},longCycle:{enabled:form.longEnabled,takeWeeks:Number(form.takeWeeks),restWeeks:Number(form.restWeeks),startDate:form.startDate},reminderTimes:[form.reminderTime]},openingBatch:{quantity:Number(form.quantity),expiryDate:form.expiryDate,priceCny:Number(form.priceCny)},ingredients:form.ingredientName.trim()&&ingredientAmount>0?[{key:form.ingredientName,name:form.ingredientName,amount:ingredientAmount,unit:form.ingredientUnit}]:[]}}
async function submit(){message.value="";if(!form.name.trim()){message.value="请填写并确认产品名称。";return}if(form.selectedWeekdays.length===0){message.value="至少选择一个星期。";return}saving.value=true;try{if(recognitionSet.value)await confirmRecognitionSet(recognitionSet.value.id,productPayload());else await createProduct(productPayload());uni.showToast({title:"已加入补充柜",icon:"success"});setTimeout(()=>uni.reLaunch({url:"/pages/today/index"}),500)}catch(error){message.value=error instanceof Error?error.message:"保存失败，图片和已填内容仍然保留。"}finally{saving.value=false}}
function statusLabel(job:RecognitionJob){return job.status==="queued"?"排队中":job.status==="running"?"识别中":job.status==="failed"?"失败":job.status==="succeeded"?"已识别":job.status==="cancelled"?"已取消":"需要核对"}
function routeLabel(job:RecognitionJob){return job.trace?.selectedRoute==="ocr_llm"?"OCR → Kimi":job.trace?.selectedRoute==="direct_vl"?"直连 VL":job.trace?.selectedRoute==="fake"?"假识别":""}
function evidenceText(job:RecognitionJob){const text=job.ocrEvidence?.rawText||"";return text.length>2400?`${text.slice(0,2400)}\n……（界面仅展示前 2400 字，服务端已保存完整 OCR 文本）`:text}
</script>

<template>
  <view class="page"><view class="bar"><button class="plain" @click="back">{{mode==='choose'?'取消':'‹ 返回'}}</button><text class="bar-title">添加产品</text><view/></view>
    <main class="content">
      <view v-if="message" class="error">{{message}}</view>
      <template v-if="mode==='choose'">
        <view class="intro"><text class="title">添加</text><text class="subtitle">识别只是候选；只有你检查并确认后，数据才会进入补充柜。</text></view>
        <button class="choice choice--primary" @click="startCapture"><view><text class="choice-title">拍照或上传三张图</text><text class="choice-hint">正面、成分表、有效期 · 异步识别</text></view><text>›</text></button>
        <button class="choice" @click="startManual"><view><text class="choice-title">直接手工填写</text><text class="choice-hint">不上传图片，不调用识别服务</text></view><text>›</text></button>
      </template>

      <template v-else-if="mode==='capture'">
        <view class="intro"><text class="title">三张标签图</text><text class="subtitle">图片私密保存。开发环境使用明确标识的假识别；真实环境会把图片发送到所配置的视觉供应商。</text></view>
        <section class="capture-list"><button v-for="role in roles" :key="role" class="capture-slot" @click="choose(role)"><image v-if="images[role]" class="preview" :src="images[role]!.path" mode="aspectFill"/><view v-else class="placeholder">＋</view><view class="capture-copy"><text class="choice-title">{{roleCopy[role].title}}</text><text class="choice-hint">{{images[role]?`${(images[role]!.size/1024/1024).toFixed(1)} MB`:roleCopy[role].hint}}</text></view><text>{{images[role]?"更换":"选择"}}</text></button></section>
        <section v-if="recognitionSet" class="job-panel"><view v-for="job in recognitionSet.jobs" :key="job.id" class="job-row"><view><text>{{roleCopy[job.role].title}}</text><text class="job-meta">{{statusLabel(job)}} · {{job.provider}}</text></view><button v-if="job.status==='failed'||job.status==='partial'" class="retry" @click="retry(job)">重试</button></view></section>
        <button class="submit" :disabled="!allSelected||saving||processing" @click="beginRecognition">{{saving||processing?"图片已保存，正在识别…":"上传并开始识别"}}</button>
      </template>

      <template v-else>
        <view class="intro"><text class="title">{{mode==='confirm'?'确认候选':'手工添加'}}</text><text class="subtitle">{{mode==='confirm'?'逐项核对。模型没有看到的字段必须由你填写。':'不经过识别，直接创建产品、计划和首批库存。'}}</text></view>
        <view v-if="mode==='confirm'" class="provider-banner" :class="{'provider-banner--fake':fakeProvider}"><text>{{fakeProvider?'开发假识别候选，不代表图片真实内容':'识别候选，尚未写入补充柜'}}</text><text>确认前可任意修改</text></view>
        <section v-if="mode==='confirm'&&recognitionSet" class="job-panel"><view v-for="job in recognitionSet.jobs" :key="job.id" class="job-block"><view class="job-row"><view><text>{{roleCopy[job.role].title}}</text><text class="job-meta">{{statusLabel(job)}} · {{Math.round(job.confidence*100)}}% · {{routeLabel(job)||job.provider}}</text></view><view class="job-actions"><button v-if="job.ocrEvidence" class="retry" @click="openEvidence[job.id]=!openEvidence[job.id]">{{openEvidence[job.id]?'收起 OCR':'查看 OCR'}}</button><button v-if="job.status==='failed'||job.status==='partial'" class="retry" @click="retry(job)">重试</button></view></view><view v-if="job.ocrEvidence&&openEvidence[job.id]" class="ocr-evidence"><text class="evidence-title">原始文字证据 · {{job.ocrEvidence.model}} · {{job.ocrEvidence.durationMs}} ms</text><text class="evidence-body">{{evidenceText(job)}}</text></view></view></section>
        <section class="card"><text class="section-title">基本信息</text><label><text>产品名称</text><input v-model="form.name" placeholder="例如：维生素 D3"/></label><label><text>品牌</text><input v-model="form.brand" placeholder="可选"/></label><view class="grid"><label><text>库存单位</text><input v-model="form.unit"/></label><label><text>每次用量</text><input v-model="form.doseQuantity" type="digit"/></label><label><text>每日次数</text><input v-model="form.doseTimesPerDay" type="number"/></label></view></section>
        <section class="card"><text class="section-title">首批库存</text><view class="grid"><label><text>数量</text><input v-model="form.quantity" type="digit"/></label><label><text>价格（元）</text><input v-model="form.priceCny" type="digit"/></label></view><label><text>有效期</text><input v-model="form.expiryDate" placeholder="YYYY-MM-DD，可选"/></label></section>
        <section class="card"><text class="section-title">每周计划</text><label><text>开始日期</text><input v-model="form.startDate"/></label><view class="weekday-row"><button v-for="day in weekdays" :key="day.v" class="weekday" :class="{'weekday--active':form.selectedWeekdays.includes(day.v)}" @click="toggleWeekday(day.v)">{{day.l}}</button></view><label><text>提醒时间</text><input v-model="form.reminderTime"/></label></section>
        <section class="card"><view class="switch-row"><view><text class="section-title">日周期</text><text class="hint">例如吃 21 天、停 7 天</text></view><switch :checked="form.dayEnabled" color="#47735a" @change="form.dayEnabled=switchValue($event)"/></view><view v-if="form.dayEnabled" class="grid"><label><text>完整周期（天）</text><input v-model="form.cycleDays" type="number"/></label><label><text>服用天数</text><input v-model="form.takeDays" type="number"/></label></view><view class="switch-row divided"><view><text class="section-title">长周期</text><text class="hint">例如吃 6 周、停 4 周</text></view><switch :checked="form.longEnabled" color="#47735a" @change="form.longEnabled=switchValue($event)"/></view><view v-if="form.longEnabled" class="grid"><label><text>服用周数</text><input v-model="form.takeWeeks" type="number"/></label><label><text>停用周数</text><input v-model="form.restWeeks" type="number"/></label></view></section>
        <section class="card"><text class="section-title">主要成分（可选）</text><label><text>成分名或原始成分文本</text><textarea v-model="form.ingredientName" placeholder="例如：维生素 D3 25 μg"/></label><text class="hint">填写大于 0 的结构化含量后，才会写入成分记录；原始候选可留在这里核对。</text><view class="grid"><label><text>结构化含量</text><input v-model="form.ingredientAmount" type="digit"/></label><label><text>单位</text><input v-model="form.ingredientUnit"/></label></view></section>
        <button class="submit" :disabled="saving" @click="submit">{{saving?"正在保存…":mode==='confirm'?"确认并加入补充柜":"保存并开始计划"}}</button>
      </template>
    </main>
  </view>
</template>

<style lang="scss" scoped>
.page{min-height:100vh;background:#f7f5f2;color:#292824}.bar{position:sticky;top:0;z-index:2;display:grid;grid-template-columns:1fr auto 1fr;align-items:center;padding:calc(18rpx + env(safe-area-inset-top)) 28rpx 18rpx;border-bottom:1rpx solid #e6e1d9;background:rgba(247,245,242,.94);backdrop-filter:blur(16px)}.bar-title{font-size:26rpx;font-weight:650}.plain{margin:0;padding:8rpx 0;background:transparent;color:#77736b;font-size:22rpx;text-align:left}.plain::after,.choice::after,.capture-slot::after,.retry::after{display:none}.content{width:min(100%,860rpx);margin:auto;padding:48rpx 34rpx 100rpx}.intro{display:flex;flex-direction:column;margin-bottom:34rpx}.title{font-family:Georgia,"Songti SC",serif;font-size:56rpx}.subtitle{margin-top:10rpx;color:#77736b;font-size:22rpx;line-height:1.55}.choice,.capture-slot{display:flex;align-items:center;justify-content:space-between;box-sizing:border-box;width:100%;margin:0 0 18rpx;padding:30rpx;border:1rpx solid #e3ded6;border-radius:20rpx;background:#fffefa;color:#4f4b44;text-align:left}.choice--primary{border-color:#cfdad1;background:#f1f5f1}.choice>view,.capture-copy{display:flex;flex-direction:column;gap:8rpx}.choice-title{font-size:26rpx;font-weight:650}.choice-hint{color:#858078;font-size:20rpx;line-height:1.4}.capture-slot{gap:20rpx}.preview,.placeholder{width:104rpx;height:104rpx;flex:0 0 auto;border-radius:15rpx}.placeholder{display:grid;place-items:center;background:#f1efea;color:#908a81;font-size:36rpx}.capture-copy{flex:1}.job-panel,.provider-banner{margin-bottom:20rpx;padding:22rpx 26rpx;border:1rpx solid #ded8e7;border-radius:18rpx;background:#f5f2f8}.job-row{display:flex;align-items:center;justify-content:space-between;padding:14rpx 0;border-bottom:1rpx solid #e8e2ed;font-size:22rpx}.job-row:last-child{border-bottom:0}.job-row>view{display:flex;flex-direction:column;gap:5rpx}.job-meta{color:#82778a;font-size:18rpx}.retry{margin:0;padding:8rpx 14rpx;border-radius:10rpx;background:#e8e0ee;color:#695779;font-size:18rpx}.provider-banner{display:flex;justify-content:space-between;color:#655777;font-size:19rpx}.provider-banner--fake{border-color:#e4c995;background:#fbf3e1;color:#8a6525}.card{display:flex;flex-direction:column;gap:24rpx;margin-bottom:20rpx;padding:30rpx;border:1rpx solid #e3ded6;border-radius:22rpx;background:#fffefa}.section-title{font-size:25rpx;font-weight:650}.card label{display:flex;flex-direction:column;gap:10rpx;color:#77736b;font-size:19rpx}.card input,.card textarea{box-sizing:border-box;width:100%;height:74rpx;padding:0 18rpx;border:1rpx solid #ded9d1;border-radius:12rpx;background:#faf9f6;color:#292824;font-size:23rpx}.card textarea{height:140rpx;padding:16rpx}.grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:18rpx}.grid label:last-child:nth-child(3){grid-column:span 2}.weekday-row{display:flex;justify-content:space-between;gap:8rpx}.weekday{display:grid;place-items:center;width:60rpx;height:60rpx;margin:0;padding:0;border:1rpx solid #ded9d1;border-radius:50%;background:#faf9f6;color:#716c64;font-size:20rpx;line-height:1}.weekday::after{display:none}.weekday--active{border-color:#47735a;background:#47735a;color:white}.switch-row{display:flex;align-items:center;justify-content:space-between}.switch-row>view{display:flex;flex-direction:column;gap:5rpx}.hint{color:#8b867d;font-size:18rpx}.divided{margin-top:8rpx;padding-top:26rpx;border-top:1rpx solid #eee9e2}.submit{margin-top:14rpx;padding:24rpx;border-radius:18rpx;background:#47735a;color:#fffefa;font-size:25rpx;font-weight:650}.submit[disabled]{opacity:.55}.error{margin-bottom:20rpx;padding:22rpx 26rpx;border:1rpx solid #e4c3bc;border-radius:18rpx;background:#fbf1ef;color:#955446;font-size:21rpx}
.job-block{border-bottom:1rpx solid #e8e2ed}.job-block:last-child{border-bottom:0}.job-block .job-row{border-bottom:0}.job-row>.job-actions{flex-direction:row;gap:8rpx}.ocr-evidence{display:flex;flex-direction:column;gap:10rpx;margin:0 0 14rpx;padding:18rpx;border-radius:12rpx;background:#fffefa}.evidence-title{color:#766b7f;font-size:17rpx}.evidence-body{white-space:pre-wrap;word-break:break-word;color:#4f4b44;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:17rpx;line-height:1.55}
</style>
