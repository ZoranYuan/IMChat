# Code Review Log

记录这次对项目代码做的规范化修改，重点放在“为什么不规范”和“应该怎么写”。

## 审查结论

这次修改主要处理了 4 类问题：

1. 错误检查顺序不合理
2. 临时状态挂在函数对象上
3. 重复逻辑较多
4. 局部代码可读性偏弱

## 修改明细

| 文件 | 问题 | 规范写法 | 修改结果 |
|---|---|---|---|
| `backend/internal/application/room/room.go` | 先生成 `conversationId` 再检查 `roomId` 生成错误，错误路径不够清晰 | 先判断错误，再使用生成结果 | 调整为先校验 `snow.GenerateSnowID` 的错误，再生成 `conversationId` |
| `backend/internal/application/user/user.go` | 多处重复“签发 token + 写缓存”逻辑，后续维护成本高 | 抽成统一 helper 函数，减少重复代码 | 新增 `issueTokensAndCache`，统一处理 token 签发和缓存写入 |
| `frontend/src/composables/useImClient.js` | `showMessage.timer` 挂在函数对象上，不够规范，也不利于维护 | 使用局部变量保存定时器句柄，并在卸载时清理 | 改为 `messageTimer`，并在组件卸载时清理定时器 |
| `frontend/src/components/WatchPanel.vue` | 上传状态判断在多个 computed 里重复写，读起来容易分散 | 抽出一个小型 helper，统一取状态 | 新增 `getChunkUploadStatus()`，复用状态读取逻辑 |

## 这次保留的写法说明

有些地方虽然还可以继续优化，但这次没有大改：

- `useImClient.js` 仍然承担较多职责，后续可以再拆分为房间、聊天、播放三个 composable
- `TODO` 注释还保留了一部分，说明这些能力是计划中的，不是这次重构的目标
- WebSocket 和回放逻辑目前已经可用，但还可以继续抽象成更清晰的状态机

## 建议的规范思路

以后看到这种代码时，可以优先检查下面几类问题：

1. 错误检查是不是在使用结果之前完成
2. 重复代码能不能抽成 helper
3. 临时状态是不是挂在了奇怪的位置
4. 一个函数是不是承担了过多职责
5. 注释是不是在解释“为什么这样做”，而不是重复代码本身
