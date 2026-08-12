export const mockCredentials = {
  account: "13800138000",
  password: "123456",
};

export const mockCurrentUser = {
  id: "user-me",
  name: "周芷清",
  account: mockCredentials.account,
  title: "产品设计师",
  status: "在线",
  avatarColor: "#5b35f5",
};

export const mockConversations = [
  {
    id: "conv-product",
    type: "group",
    name: "IM 产品共创组",
    subtitle: "林岚：交互稿我已经更新到最新版本",
    time: "10:42",
    unread: 3,
    online: true,
    memberCount: 8,
    avatarColor: "#6d4aff",
    tag: "项目",
  },
  {
    id: "conv-linlan",
    type: "direct",
    name: "林岚",
    subtitle: "好的，下午一起过一下细节",
    time: "09:18",
    unread: 0,
    online: true,
    memberCount: 2,
    avatarColor: "#8b72d6",
    tag: "在线",
  },
  {
    id: "conv-release",
    type: "group",
    name: "7 月版本发布",
    subtitle: "程诺：[文件] 发布检查清单.pdf",
    time: "昨天",
    unread: 12,
    online: false,
    memberCount: 15,
    avatarColor: "#9176c6",
    tag: "群聊",
  },
  {
    id: "conv-xie",
    type: "direct",
    name: "谢知远",
    subtitle: "服务端联调环境已经恢复",
    time: "周六",
    unread: 0,
    online: false,
    memberCount: 2,
    avatarColor: "#7764b5",
    tag: "离线",
  },
  {
    id: "conv-design",
    type: "group",
    name: "体验设计中心",
    subtitle: "你：移动端聊天页已完成第一轮适配",
    time: "周五",
    unread: 0,
    online: false,
    memberCount: 24,
    avatarColor: "#6f61a8",
    tag: "部门",
  },
];

export const mockMessages = {
  "conv-product": [
    { id: "m1", senderId: "system", type: "system", content: "今天 09:30", time: "09:30" },
    { id: "m2", senderId: "user-lin", senderName: "林岚", content: "早上好，今天主要确认移动端会话列表和消息输入区。", time: "09:32", avatarColor: "#8b72d6" },
    { id: "m3", senderId: "user-me", senderName: "周芷清", content: "收到。我会重点检查 375px 下的触摸区域和安全区。", time: "09:35", status: "read", avatarColor: "#5b35f5" },
    { id: "m4", senderId: "user-chen", senderName: "程诺", content: "桌面端会话信息面板也需要保留，方便查看群成员和共享文件。", time: "09:46", avatarColor: "#9176c6" },
    { id: "m5", senderId: "user-me", senderName: "周芷清", content: "可以，宽屏显示右侧详情栏，平板和手机端改成抽屉。", time: "09:49", status: "read", avatarColor: "#5b35f5" },
    { id: "m6", senderId: "user-lin", senderName: "林岚", content: "交互稿我已经更新到最新版本，搜索和消息状态也补齐了。", time: "10:42", avatarColor: "#8b72d6" },
  ],
  "conv-linlan": [
    { id: "m7", senderId: "user-lin", senderName: "林岚", content: "登录页的文案我精简了一版，你有空看一下。", time: "09:02", avatarColor: "#8b72d6" },
    { id: "m8", senderId: "user-me", senderName: "周芷清", content: "看过了，信息层级清楚很多。", time: "09:11", status: "read", avatarColor: "#5b35f5" },
    { id: "m9", senderId: "user-lin", senderName: "林岚", content: "好的，下午一起过一下细节。", time: "09:18", avatarColor: "#8b72d6" },
  ],
  "conv-release": [
    { id: "m10", senderId: "user-chen", senderName: "程诺", content: "发布检查清单.pdf", type: "file", fileSize: "2.4 MB", time: "昨天 17:26", avatarColor: "#9176c6" },
  ],
  "conv-xie": [
    { id: "m11", senderId: "user-xie", senderName: "谢知远", content: "服务端联调环境已经恢复，可以继续测试消息 ACK。", time: "周六 15:10", avatarColor: "#6c5d9a" },
  ],
  "conv-design": [
    { id: "m12", senderId: "user-me", senderName: "周芷清", content: "移动端聊天页已完成第一轮适配。", time: "周五 18:20", status: "read", avatarColor: "#5b35f5" },
  ],
};

export const mockContacts = [
  { id: "user-lin", name: "林岚", role: "产品经理", online: true, avatarColor: "#8b72d6", conversationId: "conv-linlan" },
  { id: "user-chen", name: "程诺", role: "前端工程师", online: true, avatarColor: "#9176c6", conversationId: "conv-release" },
  { id: "user-xie", name: "谢知远", role: "后端工程师", online: false, avatarColor: "#7764b5", conversationId: "conv-xie" },
  { id: "user-song", name: "宋闻", role: "测试工程师", online: false, avatarColor: "#6f61a8", conversationId: "" },
];

export const mockFriendRequests = [
  { id: "request-1", name: "许墨", note: "你好，我是客户端团队的许墨", time: "20 分钟前", avatarColor: "#8069c7" },
  { id: "request-2", name: "顾言", note: "来自 7 月版本发布群", time: "昨天", avatarColor: "#8f78c2" },
];
