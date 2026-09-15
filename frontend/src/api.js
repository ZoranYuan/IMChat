import axios from "axios";

export class ApiError extends Error {
  constructor(message, { code = 0, status = 0, data = null } = {}) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.data = data;
  }
}

const http = axios.create({
  baseURL: "/api/v1",
  timeout: 15000,
  withCredentials: true,
});

const refreshHttp = axios.create({
  baseURL: "/api/v1",
  timeout: 10000,
  withCredentials: true,
});

let refreshPromise = null;

const refreshAccessToken = () => {
  if (!refreshPromise) {
    refreshPromise = refreshHttp.post("/users/refresh")
      .then((response) => {
        const payload = response.data;
        const data = payload && typeof payload === "object" && "code" in payload
          ? payload.data
          : payload;
        if (!data) throw new ApiError("刷新令牌无效", { status: 401 });
        return data;
      })
      .finally(() => {
        refreshPromise = null;
      });
  }
  return refreshPromise;
};

const clearAuthAndRedirect = () => {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new CustomEvent("auth:expired"));
  if (window.location.pathname !== "/login") {
    const redirect = `${window.location.pathname}${window.location.search}`;
    // 用户登录后跳转到用户希望浏览的地址
    window.location.replace(`/login?redirect=${encodeURIComponent(redirect)}`);
  }
};

http.interceptors.response.use(
  (response) => {
    const payload = response.data;
    if (payload && typeof payload === "object" && "code" in payload) {
      if (payload.code !== 0 && payload.code !== 200) {
        const apiError = new ApiError(payload.message || "请求失败", {
          code: payload.code,
          status: response.status,
          data: payload.data,
        });
        apiError.requestConfig = response.config;
        throw apiError;
      }
      return payload.data ?? null;
    }
    return payload;
  },
  async (error) => {
    const status = error.status || error.response?.status || 0;
    const requestConfig = error.requestConfig || error.config;
    const requestURL = requestConfig?.url || "";
    const isAuthRequest = /\/users\/(login|register|refresh)$/.test(requestURL);
    if (status === 401 && !isAuthRequest && requestConfig && !requestConfig._retry) {
      requestConfig._retry = true;
      try {
        await refreshAccessToken();
        return http(requestConfig);
      } catch {
        clearAuthAndRedirect();
      }
    }
    if (error instanceof ApiError) throw error;
    const payload = error.response?.data;
    throw new ApiError(payload?.message || error.message || "网络连接失败", {
      code: payload?.code || error.response?.status || 0,
      status: error.response?.status || 0,
      data: payload?.data,
    });
  },
);

export const loginUser = ({ account, password }) =>
  http.post("/users/login", {
    account: (account || "").trim(),
    password,
  });

export const registerUser = ({ phone, password, reconfirmPassword }) =>
  http.post("/users/register", {
    loginType: 1,
    phone: phone.trim(),
    password,
    reconfirmPassword,
  });

export const logoutUser = () => http.post("/users/logout");

export const refreshSession = () => refreshAccessToken();

export const findUserByPhoneAndUserName = (keyword) => (
  http.get(`/users/${encodeURIComponent((keyword || "").trim())}`)
);

export const updateUserProfile = ({ username, nickName, avatar }) => {
  const payload = {};
  if (username !== undefined) payload.username = username;
  if (nickName !== undefined) payload.nickName = nickName;
  if (avatar !== undefined) payload.avatar = avatar;
  return http.patch("/users/me", payload);
};

export const getFriends = () => http.get("/friends");

export const getFriendRequests = () => http.get("/friend-requests");

export const createFriendRequest = ({ targetUserId, message }) =>
  http.post("/friend-requests", { targetUserId, message });

export const operateFriendRequest = ({ requestId, action }) =>
  http.post("/friend-requests/actions", { requestId, action });

export const createRoom = ({ roomName, description = "", avatar = "" }) =>
  http.post("/rooms", { roomName, description, avatar });

export const joinRoom = (inviteCode) => http.post("/rooms/join", { inviteCode });

export const getRoomInviteCode = (roomId) => http.get(`/rooms/${roomId}/invite-code`);

export const leaveRoom = (roomId) => http.post(`/rooms/${roomId}/leave`);

export const getConversations = () => http.get("/conversations");

export const getMessageHistory = (conversationId, cursor = 0, limit = 30) =>
  http.get("/messages/history", {
    params: { conversationId, cursor, limit },
  });

export const syncMessages = (conversationId, afterSeq = 0) =>
	http.get("/messages/sync", {
		params: {
			conversationId,
			afterSeq,
		},
	});

export const getMessagesBySeqs = (conversationId, seqs = []) =>
  http.get("/messages/seqs", {
    params: { conversationId, seqs: seqs.join(",") },
  });

export const getRoomVideoHistory = (roomId, limit = 20) =>
  http.get("/messages/videos", { params: { roomId, limit } });

export const getVideoDanmaku = (roomId, videoId, { startTime = 0, endTime = 0, limit = 200 } = {}) =>
  http.get("/messages/danmaku", {
    params: { roomId, videoId, startTime, endTime, limit },
  });

export const initDirectUpload = (payload) =>
  http.post("/files/direct/init", payload);

export const initUpload = (payload) =>
  http.post("/files/uploads/init", payload);

export const uploadDirectObjectToStorage = (url, file, onUploadProgress) =>
  axios.put(url, file, {
    headers: { "Content-Type": file.type || "application/octet-stream" },
    onUploadProgress,
  });

export const completeDirectUpload = (uploadId) =>
  http.post(`/files/direct/${uploadId}/complete`);

export const completeUpload = (uploadId) =>
  http.post(`/files/uploads/${uploadId}/complete`);

export const initMultipartUpload = (payload) =>
  http.post("/files/multipart/init", payload);

export const presignMultipartParts = (uploadId, partNumbers) =>
  http.post(`/files/multipart/${uploadId}/parts/presign`, { partNumbers });

export const uploadMultipartPartToStorage = (url, chunk) =>
  axios.put(url, chunk, {
    headers: { "Content-Type": "application/octet-stream" },
  });

export const completeMultipartUpload = (uploadId) =>
  http.post(`/files/multipart/${uploadId}/complete`);

export const getAttachmentAccessURLs = (attachmentIds = []) =>
	http.post("/files/attachments/access-urls", {
		attachmentIds,
	});

export default http;
