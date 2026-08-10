<script setup>
import { Bot, CheckSquare, ChevronRight, Clock3, FileText, ListChecks, MessageSquareText, RefreshCw, Sparkles, ThumbsDown, ThumbsUp, X } from "@lucide/vue";

defineProps({ conversation: { type: Object, default: null } });
defineEmits(["close", "unsupported"]);

const tabs = [
  { id: "summary", label: "摘要", icon: Sparkles },
  { id: "topics", label: "话题", icon: MessageSquareText },
  { id: "todos", label: "待办", icon: CheckSquare, count: 3 },
  { id: "files", label: "文件", icon: FileText, count: 2 },
  { id: "mine", label: "与我对话", icon: Bot },
];

const keyPoints = [
  { title: "设计稿方向已确认", detail: "整体风格和交互逻辑基本通过，继续细化视觉细节。", time: "10:20", highlight: false },
  { title: "消息气泡间距需要调整", detail: "建议增大气泡间距，提升可读性和点击体验。", time: "10:21", highlight: false },
  { title: "文件传输样式确定", detail: "进度条样式未定稿，王磊将补充方案。", time: "10:22", highlight: false },
  { title: "后续输出优化建议", detail: "你希望量化建议，供团队评审。", time: "10:23", highlight: true },
];
</script>

<template>
  <aside v-if="conversation" class="ai-workspace-panel col-[5] flex min-w-0 flex-col overflow-hidden bg-white max-[767px]:fixed max-[767px]:inset-0 max-[767px]:z-[45]">
    <header class="flex min-h-[68px] flex-none items-center gap-3 border-b border-[#ebe9f7] px-5">
      <span class="grid size-9 place-items-center rounded-xl bg-[#f0edff] text-[#5b35f5]"><Sparkles :size="20" /></span>
      <div class="min-w-0 flex-1"><h2 class="truncate text-[15px] font-bold text-[#20183b]">AI 摘要 · {{ conversation.name }}</h2><p class="mt-1 text-[11px] text-[#9893a6]">6月3日 10:00 - 10:24 · 共 18 条消息</p></div>
      <span class="hidden rounded-full bg-[#edf8f2] px-2.5 py-1 text-[10px] font-semibold text-[#42a977] min-[1200px]:inline-flex">仅对你可见</span>
      <button class="hidden items-center gap-1 rounded-lg bg-transparent text-[11px] font-semibold text-[#5b35f5] hover:bg-[#f8f7ff] min-[1100px]:inline-flex" type="button" @click="$emit('unsupported', '重新生成摘要')"><RefreshCw :size="15" />重新生成</button>
      <button class="grid size-9 place-items-center rounded-lg bg-transparent text-[#9893a6] hover:bg-[#f8f7ff] hover:text-[#5b35f5]" type="button" title="关闭" @click="$emit('close')"><X :size="19" /></button>
    </header>

    <nav class="flex flex-none gap-1 overflow-x-auto border-b border-[#ebe9f7] px-5" aria-label="AI 摘要功能">
      <button v-for="tab in tabs" :key="tab.id" :class="tab.id === 'summary' ? 'border-[#5b35f5] text-[#5b35f5]' : 'border-transparent text-[#9893a6]'" class="flex min-h-11 flex-none items-center gap-1.5 border-b-2 bg-transparent px-2 text-[11px] font-semibold" type="button"><component :is="tab.icon" :size="16" /><span>{{ tab.label }}</span><b v-if="tab.count" class="rounded-full bg-[#f0edff] px-1.5 py-0.5 text-[9px] text-[#5b35f5]">{{ tab.count }}</b></button>
    </nav>

    <div class="grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_260px] overflow-hidden max-[1100px]:grid-cols-1">
      <main class="min-h-0 overflow-y-auto px-6 py-6 max-[1100px]:px-5">
        <section class="max-w-[680px]"><h3 class="text-sm font-bold text-[#2c2345]">本次讨论概览</h3><p class="mt-3 text-[13px] leading-[1.8] text-[#6f6980]">围绕 IM 项目 v1.2 设计稿展开，整体方向已确认，重点讨论消息气泡间距、文件传输样式等细节优化建议，并形成了优化方案和后续待办。</p></section>

        <section class="mt-6 grid max-w-[680px] grid-cols-4 gap-2 max-[600px]:grid-cols-2" aria-label="摘要统计">
          <div v-for="item in [{ value: '18', label: '消息总数' }, { value: '6', label: '参与人数' }, { value: '3', label: '待办事项' }, { value: '2', label: '相关文件' }]" :key="item.label" class="grid gap-1 rounded-xl border border-[#eeeaf7] bg-[#fcfbff] px-3 py-3"><strong class="text-lg text-[#5b35f5]">{{ item.value }}</strong><span class="text-[10px] text-[#9893a6]">{{ item.label }}</span></div>
        </section>

        <section class="mt-7 max-w-[680px]"><h3 class="text-sm font-bold text-[#2c2345]">关键结论</h3><div class="mt-3 grid gap-2">
          <article v-for="item in keyPoints" :key="item.title" :class="item.highlight ? 'border-[#cfc3ff] bg-[#f8f6ff]' : 'border-[#eeeaf7] bg-white'" class="grid grid-cols-[30px_minmax(0,1fr)_42px] items-start gap-3 rounded-xl border px-3 py-3">
            <span :class="item.highlight ? 'bg-[#e9e3ff] text-[#5b35f5]' : 'bg-[#f2f0f8] text-[#8d839f]'" class="grid size-7 place-items-center rounded-lg"><component :is="item.highlight ? Sparkles : Bot" :size="15" /></span>
            <div><strong class="text-xs text-[#332d45]">{{ item.title }}</strong><p class="mt-1 text-[11px] leading-[1.6] text-[#8d889d]">{{ item.detail }}</p></div><time class="text-right text-[10px] text-[#aaa6b5]">{{ item.time }}</time>
          </article>
        </div></section>
        <button class="mx-auto mt-6 flex items-center gap-1 rounded-lg bg-transparent text-xs font-semibold text-[#5b35f5] hover:bg-[#f8f7ff]" type="button">查看完整摘要 <ChevronRight :size="16" /></button>
      </main>

      <aside class="overflow-y-auto border-l border-[#ebe9f7] bg-[#fcfbff] px-4 py-6 max-[1100px]:hidden">
        <section><h3 class="mb-3 text-xs font-bold text-[#332d45]">快捷操作</h3><div class="grid gap-1.5"><button v-for="item in [{ label: '总结讨论重点', icon: RefreshCw }, { label: '查找某个问题的讨论过程', icon: MessageSquareText }, { label: '生成待办清单', icon: ListChecks }, { label: '提取相关文件或链接', icon: FileText }]" :key="item.label" class="flex min-h-9 items-center gap-2 rounded-lg bg-white px-2.5 text-left text-[11px] text-[#706a7f] hover:bg-[#f3f0ff] hover:text-[#5b35f5]" type="button"><component :is="item.icon" :size="16" />{{ item.label }}</button></div></section>
        <section class="mt-7 border-t border-[#ebe9f7] pt-5"><h3 class="mb-3 text-xs font-bold text-[#332d45]">时间范围</h3><p class="flex gap-1.5 text-[11px] leading-[1.7] text-[#8d889d]"><Clock3 :size="15" />6月3日 10:00 - 10:24<br />共 18 条消息</p><button class="mt-2 bg-transparent text-[11px] font-semibold text-[#5b35f5]" type="button">调整范围</button></section>
        <section class="mt-7 border-t border-[#ebe9f7] pt-5"><h3 class="mb-3 text-xs font-bold text-[#332d45]">AI 提示</h3><p class="text-[11px] text-[#8d889d]">AI 生成内容可能存在不准确</p><div class="mt-3 flex gap-2"><button class="grid size-8 place-items-center rounded-lg bg-white text-[#8d889d] hover:bg-[#f0edff] hover:text-[#5b35f5]" type="button"><ThumbsUp :size="17" /></button><button class="grid size-8 place-items-center rounded-lg bg-white text-[#8d889d] hover:bg-[#fff0f3] hover:text-[#dc6570]" type="button"><ThumbsDown :size="17" /></button></div></section>
      </aside>
    </div>
  </aside>
</template>
