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
  <aside v-if="conversation" class="ai-workspace-panel">
    <header class="ai-workspace-header">
      <span class="ai-spark"><Sparkles :size="22" /></span>
      <div>
        <h2>AI 摘要 · {{ conversation.name }}</h2>
        <p>6月3日 10:00 - 10:24 · 共 18 条消息</p>
      </div>
      <span class="privacy-chip">仅对你可见</span>
      <button class="ai-regenerate" type="button" @click="$emit('unsupported', '重新生成摘要')"><RefreshCw :size="15" />重新生成</button>
      <button class="icon-button" type="button" title="关闭" @click="$emit('close')"><X :size="19" /></button>
    </header>

    <nav class="ai-tabs" aria-label="AI 摘要功能">
      <button v-for="tab in tabs" :key="tab.id" :class="{ active: tab.id === 'summary' }" type="button">
        <component :is="tab.icon" :size="16" />
        <span>{{ tab.label }}</span>
        <b v-if="tab.count">{{ tab.count }}</b>
      </button>
    </nav>

    <div class="ai-workspace-body">
      <main class="ai-summary-main">
        <section class="ai-summary-section">
          <h3>本次讨论概览</h3>
          <p>围绕 IM 项目 v1.2 设计稿展开，整体方向已确认，重点讨论消息气泡间距、文件传输样式等细节优化建议，并形成了优化方案和后续待办。</p>
        </section>

        <section class="ai-stat-grid" aria-label="摘要统计">
          <div><strong>18</strong><span>消息总数</span></div>
          <div><strong>6</strong><span>参与人数</span></div>
          <div><strong>3</strong><span>待办事项</span></div>
          <div><strong>2</strong><span>相关文件</span></div>
        </section>

        <section class="ai-summary-section">
          <h3>关键结论</h3>
          <div class="ai-key-list">
            <article v-for="item in keyPoints" :key="item.title" :class="{ highlight: item.highlight }">
              <span><component :is="item.highlight ? Sparkles : Bot" :size="15" /></span>
              <div>
                <strong>{{ item.title }}</strong>
                <p>{{ item.detail }}</p>
              </div>
              <time>{{ item.time }}</time>
            </article>
          </div>
        </section>

        <button class="ai-full-summary" type="button">查看完整摘要 <ChevronRight :size="16" /></button>
      </main>

      <aside class="ai-side-panel">
        <section>
          <h3>快捷操作</h3>
          <button type="button"><RefreshCw :size="16" />总结讨论重点</button>
          <button type="button"><MessageSquareText :size="16" />查找某个问题的讨论过程</button>
          <button type="button"><ListChecks :size="16" />生成待办清单</button>
          <button type="button"><FileText :size="16" />提取相关文件或链接</button>
        </section>

        <section>
          <h3>时间范围</h3>
          <p><Clock3 :size="15" />6月3日 10:00 - 10:24<br />共 18 条消息</p>
          <button class="text-action" type="button">调整范围</button>
        </section>

        <section>
          <h3>AI 提示</h3>
          <p>AI 生成内容可能存在不准确</p>
          <div class="ai-feedback"><button type="button"><ThumbsUp :size="17" /></button><button type="button"><ThumbsDown :size="17" /></button></div>
        </section>
      </aside>
    </div>
  </aside>
</template>
