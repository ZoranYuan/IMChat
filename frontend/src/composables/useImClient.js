import { useAuthAndFriends } from "./useAuthAndFriends";
import { useConversation } from "./useConversation";

export function useImClient() {
  const auth = useAuthAndFriends({
    onWsReconnect: () => {
      conversation.loadOffline();
    },
  });
  const conversation = useConversation({
    token: auth.token,
    currentUser: auth.currentUser,
    showMessage: auth.showMessage,
    sendFrame: auth.sendFrame,
  });

  auth.setWsFrameHandler((frame) => {
    conversation.handleWsFrame(frame);
  });

  async function submitAuth() {
    try {
      await auth.submitAuth();
      await Promise.all([conversation.loadOffline(), auth.loadFriends(), auth.loadFriendRequests()]);
      auth.connectWs();
    } catch (err) {
      auth.showMessage(err.message);
    }
  }

  function logout() {
    auth.logout();
    conversation.resetConversationState();
  }

  return {
    token: auth.token,
    currentUser: auth.currentUser,
    authMode: auth.authMode,
    authForm: auth.authForm,
    conversations: conversation.conversations,
    activeConversation: conversation.activeConversation,
    messages: conversation.messages,
    activeReadState: conversation.activeReadState,
    conversationLoading: conversation.conversationLoading,
    friends: auth.friends,
    friendRequests: auth.friendRequests,
    friendForm: auth.friendForm,
    messageText: conversation.messageText,
    messageList: conversation.messageList,
    wsConnected: auth.wsConnected,
    wsReconnecting: auth.wsReconnecting,
    wsReconnectFailed: auth.wsReconnectFailed,
    wsStatusText: auth.wsStatusText,
    message: auth.message,
    messageType: auth.messageType,
    showMessage: auth.showMessage,
    submitAuth,
    loadOffline: conversation.loadOffline,
    loadFriends: auth.loadFriends,
    loadFriendRequests: auth.loadFriendRequests,
    selectConversation: conversation.selectConversation,
    openPrivateConversation: conversation.openPrivateConversation,
    submitFriendRequest: auth.submitFriendRequest,
    handleFriendRequest: auth.handleFriendRequest,
    connectWs: auth.connectWs,
    retryWsConnection: auth.retryWsConnection,
    logout,
    sendMessage: conversation.sendMessage,
    sendImageMessage: conversation.sendImageMessage,
    sendVideoMessage: conversation.sendVideoMessage,
    sendFileMessage: conversation.sendFileMessage,
    sendStickerMessage: conversation.sendStickerMessage,
    formatTime: (ts) => {
      if (!ts) return "刚刚";
      return new Date(ts).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    },
  };
}
