<script lang="ts" setup>
import {ref, shallowRef, onMounted, onUnmounted, nextTick, computed} from 'vue';
import {POST, GET} from '../modules/fetch.ts';

type ChatMessage = {
  role: 'user' | 'assistant';
  content: string;
  isWelcome?: boolean;
  toolCalls?: Array<{tool: string; server: string; query?: string; results_count?: number}>;
  usage?: {input_tokens: number; output_tokens: number; cost_usd: number};
};

type ChatConfig = {
  ui: {
    name: string;
    subtitle: string;
    icon: string;
    placeholder: string;
    welcome_message: string;
    quick_questions: string[];
    theme: {
      primary_color: string;
      assistant_avatar: string;
      user_avatar: string;
      max_height: string;
    };
  };
  llm?: {
    model: string;
  };
};

const props = defineProps<{
  repoLink: string;
  agentFile: string;
  agentName: string;
}>();

const emit = defineEmits<{
  close: [];
}>();

const messages = ref<ChatMessage[]>([]);
const inputText = ref('');
const isStreaming = shallowRef(false);
const conversationId = shallowRef<string | null>(null);
const config = shallowRef<ChatConfig | null>(null);
const lastUsage = shallowRef<{input_tokens: number; output_tokens: number; cost_usd: number} | null>(null);
const totalToolCalls = shallowRef(0);
const messagesContainer = ref<HTMLElement | null>(null);
const inputEl = ref<HTMLTextAreaElement | null>(null);

const headerColor = computed(() => config.value?.ui?.theme?.primary_color || '#1a5276');
const assistantAvatar = computed(() => config.value?.ui?.theme?.assistant_avatar || '\u{1F916}');
const userAvatar = computed(() => config.value?.ui?.theme?.user_avatar || '\u{1F464}');
const placeholder = computed(() => config.value?.ui?.placeholder || 'Ask a question...');
const quickQuestions = computed(() => config.value?.ui?.quick_questions || []);
const showQuickQuestions = computed(() => messages.value.filter((m) => m.role === 'user').length === 0);
const maxHeight = computed(() => config.value?.ui?.theme?.max_height || '600px');
const modelName = computed(() => {
  const model = config.value?.llm?.model || 'claude';
  return model.replace('claude-', '').replace(/-\d{8}$/, '');
});

const sessionStats = ref({
  turns: 0,
  totalTokens: 0,
  totalCost: 0,
  toolsCalled: 0,
});

async function loadConfig() {
  try {
    const agentsResp = await GET(`${props.repoLink}/chat/agents`);
    if (agentsResp.ok) {
      const agents = await agentsResp.json();
      for (const agent of agents) {
        if (agent.file_path === props.agentFile) {
          config.value = agent.config;
          break;
        }
      }
    }
  } catch {
    // Use defaults if agents endpoint fails
  }

  const welcome = config.value?.ui?.welcome_message?.trim() || `Sveiki! Es esmu ${props.agentName}. Kā varu palīdzēt?`;
  messages.value.push({
    role: 'assistant',
    content: welcome,
    isWelcome: true,
  });
}

function scrollToBottom() {
  nextTick(() => {
    if (messagesContainer.value) {
      messagesContainer.value.scrollTop = messagesContainer.value.scrollHeight;
    }
  });
}

async function sendMessage(text: string) {
  if (!text.trim() || isStreaming.value) return;

  inputText.value = '';
  messages.value.push({role: 'user', content: text});
  scrollToBottom();

  isStreaming.value = true;

  const assistantMsg: ChatMessage = {role: 'assistant', content: '', toolCalls: []};
  messages.value.push(assistantMsg);
  scrollToBottom();

  try {
    const response = await POST(`${props.repoLink}/chat`, {
      data: {
        message: text,
        conversation_id: conversationId.value || '',
        agent_file: props.agentFile,
      },
    });

    if (!response.ok) {
      const errData = await response.json();
      assistantMsg.content = `Error: ${errData.error || response.statusText}`;
      isStreaming.value = false;
      return;
    }

    const reader = response.body?.getReader();
    if (!reader) {
      assistantMsg.content = 'Error: streaming not supported';
      isStreaming.value = false;
      return;
    }

    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const {done, value} = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, {stream: true});
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      let eventType = '';
      for (const line of lines) {
        if (line.startsWith('event: ')) {
          eventType = line.slice(7).trim();
        } else if (line.startsWith('data: ')) {
          const data = line.slice(6);
          try {
            const parsed = JSON.parse(data);
            switch (parsed.type) {
              case 'text':
                assistantMsg.content += parsed.text;
                scrollToBottom();
                break;
              case 'tool_call':
                assistantMsg.toolCalls?.push({
                  tool: parsed.tool,
                  server: parsed.server,
                });
                totalToolCalls.value++;
                sessionStats.value.toolsCalled++;
                break;
              case 'done':
                conversationId.value = parsed.conversation_id;
                lastUsage.value = parsed.usage;
                assistantMsg.usage = parsed.usage;
                if (parsed.usage) {
                  sessionStats.value.turns++;
                  sessionStats.value.totalTokens += (parsed.usage.input_tokens || 0) + (parsed.usage.output_tokens || 0);
                  sessionStats.value.totalCost += parsed.usage.cost_usd || 0;
                }
                break;
              case 'error':
                assistantMsg.content += `\n\nError: ${parsed.text}`;
                break;
            }
          } catch {
            // skip malformed JSON
          }
          eventType = '';
        }
      }
    }
  } catch (err: unknown) {
    assistantMsg.content = `Error: ${err instanceof Error ? err.message : 'request failed'}`;
  }

  isStreaming.value = false;
  scrollToBottom();
}

function onQuickQuestion(text: string) {
  sendMessage(text);
}

function newConversation() {
  conversationId.value = null;
  lastUsage.value = null;
  totalToolCalls.value = 0;
  sessionStats.value = {turns: 0, totalTokens: 0, totalCost: 0, toolsCalled: 0};
  messages.value = [];
  const welcome = config.value?.ui?.welcome_message?.trim() || `Sveiki! Es esmu ${props.agentName}. Kā varu palīdzēt?`;
  messages.value.push({
    role: 'assistant',
    content: welcome,
    isWelcome: true,
  });
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    sendMessage(inputText.value);
  }
}

onMounted(() => {
  loadConfig();
  inputEl.value?.focus();
});
</script>

<template>
  <div class="chat-panel" :style="{'--chat-primary': headerColor, '--chat-max-height': maxHeight}">
    <!-- Header -->
    <div class="chat-header">
      <div class="chat-header-info">
        <span class="chat-header-icon">{{ config?.ui?.icon || '\u{1F916}' }}</span>
        <div class="chat-header-text">
          <div class="chat-header-name">{{ config?.ui?.name || agentName }}</div>
          <div v-if="config?.ui?.subtitle" class="chat-header-subtitle">{{ config.ui.subtitle }}</div>
        </div>
      </div>
      <div class="chat-header-actions">
        <button class="chat-btn-icon" title="New conversation" @click="newConversation">
          &#x2795;
        </button>
        <button class="chat-btn-icon" title="Close" @click="emit('close')">
          &#x2715;
        </button>
      </div>
    </div>

    <!-- Messages -->
    <div ref="messagesContainer" class="chat-messages">
      <div v-for="(msg, idx) in messages" :key="idx"
        class="chat-message" :class="{'chat-message-user': msg.role === 'user', 'chat-message-assistant': msg.role === 'assistant'}">
        <span class="chat-avatar">{{ msg.role === 'user' ? userAvatar : assistantAvatar }}</span>
        <div class="chat-bubble">
          <div class="chat-bubble-content" v-text="msg.content"/>
          <div v-if="msg.toolCalls?.length" class="tool-calls">
            <span v-for="(tc, ti) in msg.toolCalls" :key="ti" class="tool-call-chip">
              &#x1F527; {{ tc.server }}:{{ tc.tool }}
            </span>
          </div>
          <div v-if="msg.usage" class="chat-usage">
            &#x1F4B0; ${{ msg.usage.cost_usd.toFixed(4) }}
          </div>
        </div>
      </div>
      <div v-if="isStreaming" class="chat-typing">
        <span class="chat-typing-dot"/>
        <span class="chat-typing-dot"/>
        <span class="chat-typing-dot"/>
      </div>
    </div>

    <!-- Quick Questions -->
    <div v-if="showQuickQuestions && quickQuestions.length > 0" class="chat-quick-questions">
      <button
        v-for="(q, qi) in quickQuestions" :key="qi"
        class="quick-question-btn"
        :disabled="isStreaming"
        @click="onQuickQuestion(q)"
      >
        {{ q }}
      </button>
    </div>

    <!-- Input -->
    <div class="chat-input-area">
      <div class="chat-input-wrapper">
        <textarea
          ref="inputEl"
          v-model="inputText"
          class="chat-input"
          :placeholder="placeholder"
          :disabled="isStreaming"
          rows="1"
          @keydown="onKeydown"
        />
        <button
          class="chat-send-btn"
          :disabled="isStreaming || !inputText.trim()"
          @click="sendMessage(inputText)"
        >
          &#x27A4;
        </button>
      </div>
    </div>

    <!-- Status Bar -->
    <div class="chat-status-bar">
      <span>&#x1F9E0; {{ modelName }}</span>
      <span>&#x1F4AC; {{ sessionStats.turns }} turns</span>
      <span>&#x1F4CA; {{ sessionStats.totalTokens.toLocaleString() }} tokens</span>
      <span>&#x1F4B0; ${{ sessionStats.totalCost.toFixed(4) }}</span>
      <span>&#x1F527; {{ sessionStats.toolsCalled }} tools</span>
    </div>
  </div>
</template>

<style scoped>
.chat-panel {
  display: flex;
  flex-direction: column;
  border: 1px solid var(--color-secondary);
  border-radius: 8px;
  overflow: hidden;
  max-height: var(--chat-max-height);
  background: var(--color-body);
}

.chat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  background: var(--chat-primary);
  color: white;
}

.chat-header-info {
  display: flex;
  align-items: center;
  gap: 10px;
}

.chat-header-icon {
  font-size: 24px;
}

.chat-header-name {
  font-weight: 600;
  font-size: 15px;
}

.chat-header-subtitle {
  font-size: 12px;
  opacity: 0.75;
  font-weight: 400;
  margin-top: 2px;
}

.chat-header-actions {
  display: flex;
  gap: 8px;
}

.chat-btn-icon {
  background: none;
  border: none;
  color: white;
  cursor: pointer;
  font-size: 16px;
  padding: 4px 6px;
  border-radius: 4px;
  opacity: 0.8;
}

.chat-btn-icon:hover {
  opacity: 1;
  background: rgba(255,255,255,0.15);
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 200px;
}

.chat-message {
  display: flex;
  gap: 8px;
  max-width: 85%;
}

.chat-message-user {
  align-self: flex-end;
  flex-direction: row-reverse;
}

.chat-message-assistant {
  align-self: flex-start;
}

.chat-avatar {
  font-size: 20px;
  flex-shrink: 0;
  width: 28px;
  text-align: center;
}

.chat-bubble {
  padding: 10px 14px;
  border-radius: 12px;
  font-size: 14px;
  line-height: 1.5;
  white-space: pre-wrap;
  word-break: break-word;
}

.chat-message-assistant .chat-bubble {
  background: var(--color-secondary-alpha-20, #f0f0f0);
  border-bottom-left-radius: 4px;
}

.chat-message-user .chat-bubble {
  background: var(--chat-primary);
  color: white;
  border-bottom-right-radius: 4px;
}

.tool-calls {
  margin-top: 6px;
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.tool-call-chip {
  background: var(--color-info-bg);
  border: 1px solid var(--color-info-border);
  border-radius: 12px;
  padding: 2px 8px;
  font-size: 11px;
  font-family: var(--fonts-monospace);
  color: var(--color-info-text);
}

.chat-usage {
  font-size: 11px;
  color: var(--color-text-light);
  margin-top: 4px;
}

.chat-typing {
  display: flex;
  gap: 4px;
  padding: 8px 16px;
  align-self: flex-start;
}

.chat-typing-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-text-light);
  animation: typing-bounce 1.4s ease-in-out infinite;
}

.chat-typing-dot:nth-child(2) { animation-delay: 0.2s; }
.chat-typing-dot:nth-child(3) { animation-delay: 0.4s; }

@keyframes typing-bounce {
  0%, 80%, 100% { opacity: 0.3; transform: scale(0.8); }
  40% { opacity: 1; transform: scale(1); }
}

.chat-input-area {
  padding: 12px 16px 8px;
  border-top: 1px solid var(--color-secondary);
}

.chat-input-wrapper {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  background: var(--color-input-background, #fff);
  border: 1px solid var(--color-secondary);
  border-radius: 8px;
  padding: 8px 12px;
}

.chat-input {
  flex: 1;
  border: none;
  outline: none;
  resize: none;
  font-size: 14px;
  line-height: 1.5;
  background: transparent;
  color: var(--color-text);
  font-family: inherit;
  max-height: 120px;
}

.chat-send-btn {
  background: var(--chat-primary);
  color: white;
  border: none;
  border-radius: 6px;
  width: 32px;
  height: 32px;
  cursor: pointer;
  font-size: 16px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.chat-send-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.chat-quick-questions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 16px;
}

.quick-question-btn {
  background: var(--color-body);
  border: 1px solid var(--color-secondary-alpha-40);
  border-radius: 18px;
  padding: 6px 14px;
  font-size: 12.5px;
  color: var(--color-primary);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.quick-question-btn:hover {
  background: var(--color-primary);
  color: var(--color-primary-contrast);
  border-color: var(--color-primary);
}

.quick-question-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.chat-status-bar {
  padding: 5px 16px;
  background: var(--color-box-body-highlight);
  border-top: 1px solid var(--color-secondary-alpha-20);
  display: flex;
  gap: 14px;
  font-size: 11px;
  color: var(--color-text-light-2);
  font-family: var(--fonts-monospace);
}
</style>
