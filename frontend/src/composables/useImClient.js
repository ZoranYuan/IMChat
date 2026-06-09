import { useAuthAndFriends } from "./useAuthAndFriends";
import { useConversation } from "./useConversation";
import { useRoomWatch } from "./useRoomWatch";

export function useImClient() {
  const auth = useAuthAndFriends({
    onWsReconnect: () => {
      conversation.loadOffline();
    },
  });
  const room = useRoomWatch({
    token: auth.token,
    currentUser: auth.currentUser,
    showMessage: auth.showMessage,
    sendFrame: auth.sendFrame,
    wsConnected: auth.wsConnected,
    openConversation: (conversationId, convType, content) => conversation.openConversation(conversationId, convType, content),
  });
  const conversation = useConversation({
    token: auth.token,
    currentUser: auth.currentUser,
    showMessage: auth.showMessage,
    sendFrame: auth.sendFrame,
    onRoomConversationSelected: room.selectRoomConversation,
    getWatchVideoTime: () => Math.floor((room.videoRef.value?.currentTime || 0) * 1000),
  });

  auth.setWsFrameHandler((frame) => {
    conversation.handleWsFrame(frame);
    room.handleWsFrame(frame);
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
    room.resetRoomState();
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
    roomForm: room.roomForm,
    activeRoomId: room.activeRoomId,
    fileIdInput: room.fileIdInput,
    uploadName: room.uploadName,
    chunkUpload: room.chunkUpload,
    video: room.video,
    watchSession: room.watchSession,
    applyingWatchState: room.applyingWatchState,
    videoRef: room.videoRef,
    visibleDanmaku: room.visibleDanmaku,
    canControlWatchVideo: room.canControlWatchVideo,
    canStartWatchSession: room.canStartWatchSession,
    canStopWatchSession: room.canStopWatchSession,
    watchActionLabel: room.watchActionLabel,
    watchStatusLabel: room.watchStatusLabel,
    watchOwnerLabel: room.watchOwnerLabel,
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
    handleCreateRoom: room.handleCreateRoom,
    handleJoinRoom: room.handleJoinRoom,
    handleInvite: room.handleInvite,
    handleUpload: room.handleUpload,
    loadFile: room.loadFile,
    loadVideoToRoom: room.loadVideoToRoom,
    stopWatchSession: room.stopWatchSession,
    sendWatchControl: room.sendWatchControl,
    seekBy: room.seekBy,
    onVideoTimeUpdate: room.onVideoTimeUpdate,
    setVideoElement: room.setVideoElement,
    formatTime: (ts) => {
      if (!ts) return "刚刚";
      return new Date(ts).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" });
    },
  };
}
