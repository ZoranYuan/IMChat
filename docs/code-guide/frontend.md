# 前端业务代码

## 1. 前端分层

前端没有引入 Pinia，而是使用 Vue Composition API 按业务拆分状态：

```text
App.vue                       页面布局和模块组合
  -> useImClient              总协调器
     -> useAuthAndFriends     登录、好友、WebSocket 生命周期
     -> useConversation       会话、消息、ACK、离线数据
     -> useRoomWatch          房间、一起看、弹幕
        -> useChunkUpload     大文件分片上传
  -> api.js                   HTTP 边界
  -> wsProto.js               WebSocket Protobuf 边界
```

组件负责展示和发出 UI 事件，Composable 保存状态并执行业务流程，API/Protocol 文件负责网络契约。这种拆分避免把登录、聊天、上传和播放同步全部堆在 `App.vue`。

## 2. 文件总索引

| 文件 | 作用 | 当前状态 |
|---|---|---|
| `src/main.js` | 创建 Vue 应用、加载全局样式并挂载 `App` | 使用中 |
| `src/App.vue` | 登录页/聊天页/一起看页面布局，组合所有业务状态 | 核心入口 |
| `src/api.js` | Axios 实例、统一错误、Token 注入和所有 HTTP API | 核心网络层 |
| `src/wsProto.js` | 浏览器端 Protobuf Schema，WebSocket 帧编码解码 | 核心网络层 |
| `src/composables/useImClient.js` | 组合 Auth、Conversation、Room 三个业务模块 | 核心协调器 |
| `src/composables/useAuthAndFriends.js` | 登录、好友、提示、WebSocket 连接和重连 | 核心业务 |
| `src/composables/useConversation.js` | 会话列表、消息收发、ACK、历史、离线、媒体消息 | 核心业务 |
| `src/composables/useRoomWatch.js` | 房间与一起看播放状态、弹幕 | 核心业务 |
| `src/composables/useChunkUpload.js` | 文件指纹、并发分片、重试、暂停和续传 | 核心业务 |
| `src/components/LoginPage.vue` | 手机号登录/注册 UI、OAuth 预留入口 | 使用中 |
| `src/components/SidebarAuth.vue` | 旧版侧栏登录 UI | 当前未被 `App.vue` 引用，可视为遗留组件 |
| `src/components/ConversationList.vue` | 会话列表、未读数、头像降级和刷新入口 | 使用中 |
| `src/components/FriendsPanel.vue` | 好友列表、搜索申请、处理申请 | 使用中 |
| `src/components/ChatPanel.vue` | 消息渲染、输入、媒体选择、已读边界 | 使用中 |
| `src/components/WatchPanel.vue` | 房间操作、视频上传、播放器和同步控制 | 使用中 |

`src/styles.css` 是全局设计变量和布局样式，不属于业务代码，本篇不逐条展开。

## 3. `main.js`

典型内容：

```js
import { createApp } from "vue";
import App from "./App.vue";
import "./styles.css";

createApp(App).mount("#app");
```

它只负责启动，不放业务状态。这样入口稳定，业务变化不会不断修改启动文件。

## 4. `api.js`：统一 HTTP 边界

### 4.1 ApiError

```js
export class ApiError extends Error {
  constructor(message, { code = 0, status = 0, data = null, raw = null } = {}) {
    super(message);
    this.code = code;     // 后端业务码
    this.status = status; // HTTP 状态码
    this.data = data;     // 后端错误附加数据
    this.raw = raw;       // 调试时保留原响应
  }
}
```

为什么这样写：Axios 网络错误、HTTP 4xx/5xx、后端 HTTP 200 但业务码失败，结构并不相同。统一成 `ApiError` 后，组件只需要 `showMessage(err.message)`，同时高级逻辑仍可根据 `status/code` 分支。

### 4.2 请求与响应拦截器

```js
http.interceptors.request.use((config) => {
  // 单次调用显式 token 优先，否则使用本地登录态。
  const token = config.token || localStorage.getItem("im_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;

  // config.token 不是 Axios 标准字段，不应继续传给适配器。
  delete config.token;
  return config;
});

http.interceptors.response.use((response) => {
  const payload = response.data;
  if ("code" in payload && payload.code !== 0 && payload.code !== 200) {
    throw new ApiError(payload.message, { code: payload.code, status: response.status });
  }
  return payload.data ?? payload;
});
```

为什么这样写：所有 API 不再重复拼 Authorization，也不重复解包 `{code,message,data}`。新增接口只描述路径和参数。

### 4.3 API 函数职责

| 函数 | 对应业务 |
|---|---|
| `login`、`register` | 固定 `loginType=1`，当前只走手机号 |
| `getOfflineMessages` | 初始化或重连后拉会话摘要和未读 |
| `getHistoryMessages` | 按 conversationId、cursor、limit 拉历史 |
| `getDanmaku` | 按 roomId、videoId 和时间范围拉弹幕 |
| `createRoom`、`joinRoom`、`getInviteCode` | 房间创建、加入和邀请 |
| `uploadFile`、`getFile` | 普通上传和文件元数据读取 |
| `initMultipartUpload`、`uploadMultipartPart`、`completeMultipartUpload` | 分片上传三阶段 |
| `getFriends` | 好友列表 |
| `resolveUser` | 用用户名/手机号先解析目标 userId |
| `createFriendRequest`、`getFriendRequests`、`operateFriendRequest` | 好友申请完整流程 |

API 层不直接修改 Vue State，它只返回 Promise。这样网络代码可以独立测试，状态更新由具体业务 Composable 决定。

## 5. `wsProto.js`：WebSocket 二进制协议

这个文件在浏览器运行时解析一份与后端 `ws.proto` 对应的 Schema，并缓存消息类型。

```js
export function encodeFrame(op, typeName, payload) {
  const type = types[typeName];

  // 先编码具体 payload，再放进统一 WsFrame。
  const data = type.encode(type.create(payload)).finish();
  return types.frame.encode(types.frame.create({ op, data })).finish();
}

export function decodeFrame(buffer) {
  const frame = types.frame.decode(new Uint8Array(buffer));
  return toPlain(types.frame, frame);
}
```

为什么分两层：外层 `op` 用来路由，内层 `data` 使用该操作专属结构。一个 WebSocket 连接即可承载消息、ACK、已读和播放同步。

`toPlain` 把 Protobuf 的 Long 转成 Number、Bytes 转成 Uint8Array，否则业务代码会接触 protobufjs 的内部对象。当前 seq 和毫秒时间在 JavaScript 安全整数范围内；如果 ID 改为 int64 数值并超过 `2^53-1`，必须改成字符串处理。

Schema 同时存在于前后端，修改字段时必须保持 field number 不变并同步更新。更成熟的做法是构建时从同一个 `.proto` 生成前后端代码，减少手工漂移。

## 6. `useImClient.js`：模块协调器

关键代码：

```js
const auth = useAuthAndFriends({
  // 连接恢复后补拉服务端状态。
  onWsReconnect: () => conversation.loadOffline(),
});

const room = useRoomWatch({
  token: auth.token,
  sendFrame: auth.sendFrame,
  openConversation: (...args) => conversation.openConversation(...args),
});

const conversation = useConversation({
  token: auth.token,
  sendFrame: auth.sendFrame,
  onRoomConversationSelected: room.selectRoomConversation,
  getWatchVideoTime: () => Math.floor((room.videoRef.value?.currentTime || 0) * 1000),
});

// 一个 WS 帧可能同时影响聊天和一起看弹幕。
auth.setWsFrameHandler((frame) => {
  conversation.handleWsFrame(frame);
  room.handleWsFrame(frame);
});
```

为什么需要协调器：三个模块彼此有少量协作，但不应互相直接 import 形成环依赖。`useImClient` 通过回调和依赖注入连接它们。

登录成功后并行拉离线会话、好友和好友申请，再连接 WebSocket：

```js
await auth.submitAuth();
await Promise.all([
  conversation.loadOffline(),
  auth.loadFriends(),
  auth.loadFriendRequests(),
]);
auth.connectWs();
```

`Promise.all` 缩短初始化等待时间。先获得 Token 再并行拉数据，最后连接实时通道。这里存在一个很短的 HTTP 与 WebSocket 切换窗口，但随后重连/刷新仍可通过离线接口补齐。

退出时同时清理三个模块，解决只清 Token 却残留上一个账号会话、消息和播放状态的问题。

## 7. `useAuthAndFriends.js`

### 7.1 登录状态

```js
const token = ref(localStorage.getItem("im_token") || "");
const currentUser = reactive(JSON.parse(localStorage.getItem("im_user") || "{}"));

async function submitAuth() {
  const data = authMode.value === "login" ? await login(authForm) : await register(authForm);
  token.value = data.token;
  Object.assign(currentUser, data);
  localStorage.setItem("im_token", data.token);
  localStorage.setItem("im_user", JSON.stringify(data));
}
```

为什么同时用 Ref/Reactive 和 localStorage：Ref 驱动当前页面响应式更新，localStorage 让刷新页面后恢复登录界面。它不是最终安全方案，Access Token 存 localStorage 仍可能被 XSS 读取；生产方案可改为更严格的 Cookie 策略。

切换登录/注册时：

```js
watch(authMode, () => {
  authForm.phone = "";
  authForm.password = "";
});
```

这解决切换模式后上一个表单数据残留，尤其避免注册时误提交登录页留下的密码。

### 7.2 好友流程

发送申请不是直接把输入字符串当 userId：

```js
const user = await resolveUser(token.value, friendForm.keyword);
friendForm.toUserId = user.userId;
await createFriendRequest(token.value, friendForm);
```

用户可以输入手机号或用户名，后端先返回唯一 userId，再创建申请。UI 展示字段与数据库主键由此分离。

处理申请成功后并行刷新好友与申请列表，确保“待处理申请消失”和“新好友出现”同时反映。

### 7.3 WebSocket 连接和重连

```js
ws.value.onopen = () => {
  const wasReconnecting = wsReconnecting.value;
  wsConnected.value = true;
  wsReconnectAttempts.value = 0;
  if (wasReconnecting) onWsReconnect?.();
};

ws.value.onclose = () => {
  wsConnected.value = false;
  if (!wsManualClose && token.value) scheduleWsReconnect();
};
```

`wsManualClose` 区分“用户主动退出/替换连接”和“网络异常”，否则主动关闭旧连接也会触发自动重连。

```js
const delay = Math.min(1000 * attempts, 5000);
setTimeout(() => connectWs(false), delay);
```

逐步增加延迟避免服务端故障时所有浏览器高频重连。当前最多 5 次，并允许用户点击状态点重新尝试。

连接恢复后调用 `loadOffline`，因为 WebSocket 只保证在线期间尽力推送，断线窗口的数据要从持久化接口补回来。

### 7.4 收帧

`handleWsMessage` 先解外层 Frame，再按 op 解 payload。普通消息、ACK、播放状态使用 Protobuf；当前 `msg_read_notify` 使用 JSON 文本解码，这是后端当前发送格式决定的兼容分支。未来最好统一协议，减少特殊判断。

## 8. `useConversation.js`

这是前端最重要的业务文件。

### 8.1 会话 Upsert 而不是替换

```js
function upsertConversationPreview(conversationId, patch = {}, moveToTop = false) {
  const index = getConversationIndex(conversationId);
  const current = index >= 0 ? conversations.value[index] : { conversationId, unread: 0 };
  const next = { ...current, ...patch };

  // 只有 patch 明确提供字段时才覆盖；未提供时保留 existing.avatar。
  if (Object.prototype.hasOwnProperty.call(patch, "avatar")) {
    // 当前实现允许调用方用空字符串明确清空头像。
    next.avatar = patch.avatar || "";
  }

  // 新消息时移到顶部，普通选择时保持顺序。
  return replaceOrInsert(next, moveToTop);
}
```

为什么用 `hasOwnProperty.call`：要区分“调用方没有提供 avatar”和“调用方明确提供空 avatar”。前者保留已有头像，后者按当前实现清空头像。直接判断 `if (patch.avatar)` 无法区分这两种情况；使用 `Object.prototype...call` 也不受对象自己覆盖 `hasOwnProperty` 的影响。

解决的问题：选择会话时用信息不完整的 patch 覆盖完整会话，导致头像和名称消失。

### 8.2 消息缓存和去重

当前只把最后一个活跃会话及其消息缓存到 `localStorage` 的 `im_active_conversation_messages`，切回该会话时可以先显示缓存，再异步拉历史。它不是所有会话的完整缓存。

```js
function messageDedupKey(message) {
  return (
    message?.messageId ||
    message?.clientMsgId ||
    `${message?.conversationId}:${message?.seq}:${message?.sendTime}:${message?.content}`
  );
}

function mergeConversationMessages(existing, incoming) {
  const merged = [];
  const seen = new Set();
  for (const message of [...existing, ...incoming]) {
    const key = messageDedupKey(message);
    if (seen.has(key)) continue;
    seen.add(key);
    merged.push({ ...message });
  }
  return merged.sort(compareBySeqThenTime);
}
```

为什么有多级 Key：本地乐观消息尚无 messageId/seq，服务端消息有 messageId；兼容两种生命周期才能避免自己的消息显示两次。

### 8.3 乐观发送与 ACK

```js
const clientMsgId = crypto.randomUUID();
const ok = sendFrame("msg", "messageReq", { clientMsgId, ...payload });
if (!ok) return false;

// 不等服务器往返，先显示本地消息。
messages.value.push(localMessage);
trackPendingLocalMessage({ clientMsgId, conversationId, content });
```

当前收到成功 `msg_ack` 时没有把后端 `messageId/status` 回填到本地气泡；失败 ACK 会移除 Pending 记录并提示错误。服务端回推到达时，再通过 clientMsgId 或发送人、内容、类型、15 秒时间窗口识别自己的本地 Echo，并删除 Pending 记录。

为什么这样写：用户点击发送后立即看到气泡，降低感知延迟；clientMsgId 同时作为前后端幂等键，使 ACK 超时重试不会创建重复消息。

### 8.4 收到消息与未读

```js
if (msg.conversationId === activeConversation.value?.conversationId) {
  // 用户正在看该会话：加入消息并把列表未读保持为 0。
  appendMessage(msg);
  unread = 0;
} else {
  // 未进入会话才增加未读，并更新列表摘要。
  unread += msg.senderId === currentUser.userId ? 0 : 1;
}
```

这体现了一部分产品语义：未进入的会话才增加未读，自己的本地 Echo 会被去重，不增加自己的未读。

当前边界：实时收到“正在打开的会话”的新消息时，代码只把前端未读保持为 0，没有立即调用 `sendReadAck`。ACK 目前主要发生在 `selectConversation` 拉完历史之后。因此用户持续停留在会话内收到的新消息，后端 `LastReadSeq` 可能暂时落后，刷新后可能重新出现未读。这是后续应修复的业务缺口。

### 8.5 选择会话、历史和 ACK

```js
async function selectConversation(item) {
  const loadSeq = ++conversationLoadSeq;
  const requestedID = item.conversationId;

  activeConversation.value = upsertConversationPreview(item.conversationId, {
    displayName: item.displayName,
    convType: item.convType,
    unread: 0,
  });

  const history = await getHistoryMessages(token.value, requestedID);

  // 用户快速切换后，旧请求返回不能覆盖新会话。
  if (loadSeq !== conversationLoadSeq || activeConversation.value?.conversationId !== requestedID) return;

  messages.value = mergeConversationMessages(cached, history.messages || []);
  const latest = lastMessageWithSeq(messages.value);
  if (latest.seq > currentReadState.lastReadSeq) {
    sendReadAck(requestedID, latest.seq, latest.senderId);
  }
}
```

`conversationLoadSeq` 是轻量请求世代号，解决快速点击 A、B 后 A 的慢响应最后返回并覆盖 B 的竞态。

拉完历史后发 ACK 是合理的前提是该函数只在“用户明确进入会话”时调用。离线列表刷新不调用此 ACK，因此不会把没打开的会话标记已读。

### 8.6 离线列表

`loadOffline` 将后端返回转成统一会话结构，并保留当前 activeConversation 的引用。它不因为列表中最新消息来自自己就盲目增加未读，最终未读值以后端基于 `LastReadSeq` 的计算为准。

### 8.7 媒体消息与文字说明

```js
async function sendMediaPayload(file, cType) {
  // 用户选择文件前输入框里的文字成为媒体说明。
  const caption = messageText.value.trim();
  const uploaded = await uploadFile(token.value, file);

  const meta = await readImageOrVideoMetadata(file, cType);
  const content = caption || defaultMediaPreview(cType, file.name);

  sendFrame("msg", "messageReq", {
    cType,
    content,
    fileId: uploaded.fileId,
    mediaUrl: uploaded.url,
    fileName: uploaded.fileName,
    fileSize: uploaded.size,
    width: meta.width,
    height: meta.height,
    durationMs: meta.durationMs,
  });
}
```

为什么先上传再发消息：消息记录需要稳定 fileId。上传失败时不产生一条无法打开的媒体消息。尺寸让 UI 在资源加载前预留比例，时长用于视频卡片和业务校验。

当前 `sendStickerMessage` 已实现协议发送，但 `ChatPanel` 没有表情选择入口，因此属于能力已存在、UI 未接通。

## 9. `useChunkUpload.js`

### 9.1 文件指纹

```js
const samples = [
  file.slice(0, sampleSize),
  file.slice(middleStart, middleStart + sampleSize),
  file.slice(file.size - sampleSize),
];

const fingerprint = sha256(fileMetadata + sampledBytes);
```

为什么抽样而不是对超大文件全量 Hash：减少浏览器 CPU、内存和等待时间。代价是理论上存在抽样碰撞，因此它适合作为“快速秒传提示键”，严格去重仍应由服务端结合完整校验确认。

### 9.2 并发 Worker

```js
let cursor = 0;

async function worker() {
  while (cursor < queue.length) {
    if (canceled.value) throw new Error("上传已取消");
    while (paused.value) await sleep(200);

    const partNumber = queue[cursor++];
    await uploadPartWithRetry(token, file, uploadId.value, partNumber, totalChunks);
  }
}

await Promise.all(
  Array.from({ length: Math.min(concurrency, queue.length) }, () => worker()),
);
```

JavaScript 单线程中 `cursor++` 在两个 `await` 之间同步执行，因此各 Worker 不会领取同一个 Part。并发数默认 3，在吞吐和浏览器连接/内存占用之间折中。

每个 Part 最多重试 3 次；初始化返回 `uploadedParts`，刷新或中断后只把缺失分片放进队列；最后调用 Complete 让 MinIO 合并。

暂停只是停止领取新 Part，不会强行中止正在发送的请求；取消也没有调用服务端 Abort Multipart，过期状态依赖 Redis TTL/后续清理。这是当前边界。

## 10. `useRoomWatch.js`

### 10.1 房间状态

该文件保存：当前房间、邀请码表单、视频文件、播放器 Ref、共享会话、控制者、弹幕和上传状态。

创建/加入房间成功后调用 `openConversation(roomId, 2, roomName)`，让房间聊天复用普通群聊能力，而不是再建立一套聊天状态。

### 10.2 发送播放控制

```js
function sendWatchControl(action, patch = {}) {
  if (!activeRoomId.value) return showMessage("请先进入房间");
  if (hasActiveWatchOwner.value) return showMessage("当前由其他成员共享");

  sendFrame("watch_video_control", "watchControl", {
    roomId: activeRoomId.value,
    action,
    videoId: video.fileId,
    videoUrl: video.url,
    positionMs: Math.floor((videoRef.value?.currentTime || 0) * 1000),
    playbackRate: videoRef.value?.playbackRate || 1,
    clientTimeMs: Date.now(),
    ...patch,
  });
}
```

为什么传毫秒和 clientTime：整数毫秒跨语言更稳定，服务端/客户端可以结合发送时间估算正在播放状态的目标位置。

前端先做控制权提示改善体验，后端仍必须再次校验，不能把权限安全交给 UI。

### 10.3 应用远端状态

```js
applyingWatchState.value = true;
video.currentTime = state.positionMs / 1000;
video.playbackRate = state.playbackRate || 1;
if (state.isPlaying) video.play();
else video.pause();

setTimeout(() => {
  applyingWatchState.value = false;
}, 300);
```

`applyingWatchState` 防止程序为了同步而调用 `play/pause/seek` 时，又被原生事件监听器当作用户操作发送回服务端，形成反馈回路。

只有本地与目标进度差大于约 1.2 秒才强制跳转，避免播放器微小计时误差导致画面频繁抖动。

### 10.4 弹幕

房间文本消息如果带 `hasVideoTime/videoTime`，既进入聊天列表，也进入当前视频的弹幕集合。历史弹幕按 `roomId + videoId` 拉取，再用 messageId/clientMsgId 去重合并。

解决的问题：不额外设计一套弹幕发送通道，复用消息可靠性、权限和存储链路，同时用 VideoId/VideoTime 建立回放上下文。

## 11. `App.vue`

`App.vue` 的职责是页面编排而不是网络业务：

- 没 Token 时展示 `LoginPage`。
- 有 Token 时展示左侧应用菜单。
- 在会话目录和好友目录间切换。
- 在聊天工作区和一起看工作区间切换。
- 把 `useImClient` 暴露的状态与函数绑定给子组件。
- 管理主题、侧栏拖拽宽度、刷新按钮状态和全局提示。

关键绑定：

```vue
<ChatPanel
  :active-conversation="activeConversation"
  :messages="messages"
  :last-read-seq="activeReadState.lastReadSeq"
  :read-receivers="activeReadState.readers"
  @send-message="sendMessage"
  @send-image="sendImageMessage"
  @send-file="sendFileMessage"
  @send-video="sendVideoMessage"
/>
```

为什么子组件用 Props + Emits：ChatPanel 不知道 Token、Axios、WebSocket 和 Repository，只表达“显示什么”和“用户做了什么”。业务状态仍由父层 Composable 单向提供，降低组件耦合。

`watchConversation` 把房间转换成普通会话形状，一起看侧栏因此复用同一个 `ChatPanel`。

主题写入 `documentElement.dataset.theme` 和 localStorage，使 CSS 变量切换和刷新恢复保持一致。

第三方登录的 `handleOAuthLogin` 当前只提示功能未开放，不应把按钮存在理解为 OAuth 已完成。

## 12. 展示组件

### `LoginPage.vue`

职责：登录/注册表单、主题按钮、第三方登录预留和首屏打字动画。

它直接修改父层传入的 `authForm` 响应式对象，并通过 Emit 通知提交和模式切换。模式切换后的数据清理由 `useAuthAndFriends` 的 Watch 完成，因此不同页面复用同一逻辑。

动画 Timer 在 `onBeforeUnmount` 清理，避免离开登录页后定时器继续修改已销毁组件。

### `SidebarAuth.vue`

旧版侧栏登录组件，职责与 LoginPage 部分重叠，目前没有被 App 导入。保留它会增加理解成本；确认不再回退旧布局后可以删除。

### `ConversationList.vue`

职责：展示会话名、最后一条消息、未读 Badge、选中态和刷新按钮。

头像加载失败时：

```js
const brokenAvatarSources = ref(new Set());

function handleAvatarError(item) {
  // 创建新 Set，保证 Vue 能观察引用变化。
  const next = new Set(brokenAvatarSources.value);
  next.add(item.avatar);
  brokenAvatarSources.value = next;
}
```

失败 URL 被记录后改用名字首字母，不会让浏览器在每次渲染时反复请求坏地址。

当前搜索框只有 UI，没有过滤逻辑。

### `FriendsPanel.vue`

职责：好友申请表单、好友列表、待处理申请和操作按钮。

```js
const pendingRequests = computed(() =>
  props.friendRequests.filter((request) => request.status === 1),
);
```

原始列表仍由父层持有，组件只派生待处理视图。点击好友 Emit `open-chat`，不在展示组件中创建 conversationId。

### `ChatPanel.vue`

职责：按 CType 渲染消息、发送者头像、媒体卡片、已读边界、输入框和文件选择器。

```vue
<template v-if="isImageMessage(msg) || isStickerMessage(msg)">
  <img :src="mediaSource(msg)" />
</template>
<template v-else-if="isVideoMessage(msg)">
  <video :src="mediaSource(msg)" controls />
</template>
<template v-else-if="isFileMessage(msg)">
  <a :href="mediaSource(msg)" target="_blank">...</a>
</template>
<div v-else>{{ msg.content }}</div>
```

类型渲染集中在一个组件，历史消息和实时消息使用同一 DTO 后无需两套 UI。

已读边界从“当前用户发送且 seq <= lastReadSeq”的最后一条消息算出，在该气泡后显示已读头像。这比每条消息都显示“已读”更接近会话游标模型。

Enter 发送、Shift+Enter 换行；图片、文件、视频分别使用带 accept 的隐藏 File Input。组件只 Emit File，上传和消息发送留给 Composable。

当前“更多”和“语音”按钮只有 UI，没有业务事件。

### `WatchPanel.vue`

职责：创建/加入房间、展示邀请码、选择和分片上传视频、输入 fileId 加载、播放控制、上传进度和弹幕层。

本地播放器事件先由 `emitNativeVideoControl` 发给父层，父层结合 `applyingWatchState` 判断是否需要同步。它不自己持有 WebSocket，仍保持展示组件边界。

上传状态文字由 Computed 根据 `hashing/initializing/uploading/completing/completed/error` 映射，避免模板到处判断底层状态码。

## 13. 前端状态一致性原则

阅读代码时重点记住：

- 后端返回的 `messageId/seq` 是最终事实，本地 `clientMsgId` 负责发送期间关联。
- `activeConversation` 代表用户真正进入的会话，只有这个会话的新消息自动 ACK。
- `conversations` 是摘要列表，刷新摘要不能覆盖完整消息数组。
- 历史请求必须验证返回时用户仍在同一个会话。
- WebSocket 断线不代表数据永久丢失，重连后通过 Offline API 对账。
- 远端播放器状态应用期间必须屏蔽本地事件回传。
- 上传文件成功与发送媒体消息成功是两个阶段，当前第一阶段成功、第二阶段失败会留下未引用文件，需要后端定期清理孤儿对象。
