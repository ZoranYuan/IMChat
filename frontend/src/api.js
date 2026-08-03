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
});

http.interceptors.request.use((config) => {
  const token = localStorage.getItem("im_token") || sessionStorage.getItem("im_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  return config;
});

http.interceptors.response.use(
  (response) => {
    const payload = response.data;
    if (payload && typeof payload === "object" && "code" in payload) {
      if (payload.code !== 0 && payload.code !== 200) {
        throw new ApiError(payload.message || "请求失败", {
          code: payload.code,
          status: response.status,
          data: payload.data,
        });
      }
      return payload.data ?? null;
    }
    return payload;
  },
  (error) => {
    const status = error.response?.status || 0;
    const requestURL = error.config?.url || "";
    // 排查登录和注册时返回的 401
    const isAuthRequest = /\/users\/(login|register)$/.test(requestURL);
    if (status === 401 && !isAuthRequest && typeof window !== "undefined") {
      // 通过 window 派发事件，后由 store 去进行执行，一次实现解耦
      window.dispatchEvent(new CustomEvent("auth:expired"));
      if (window.location.pathname !== "/login") {
        const redirect = `${window.location.pathname}${window.location.search}`;
        window.location.replace(`/login?redirect=${encodeURIComponent(redirect)}`);
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

export const loginUser = ({ account, phone, userName, password }) => {
  const credential = (account || phone || userName || "").trim();
  const payload = { loginType: 1, password };
  if (/^1\d{10}$/.test(credential)) payload.phone = credential;
  else payload.userName = credential;
  return http.post("/users/login", payload);
};

export const registerUser = ({ phone, password, reconfirmPassword }) =>
  http.post("/users/register", {
    loginType: 1,
    phone: phone.trim(),
    password,
    reconfirmPassword,
  });

export const logoutUser = () => http.post("/users/logout");

export const getUser = (userId) => http.get(`/users/${userId}`);

export const getFriends = () => http.get("/friends");

export const getFriendRequests = () => http.get("/friend-requests");

export const createFriendRequest = ({ toUserId, message }) =>
  http.post("/friend-requests", { toUserId, message });

export const operateFriendRequest = ({ requestId, action }) =>
  http.post("/friend-requests/actions", { requestId, action });

export const createRoom = ({ roomName, description = "", avatar = "" }) =>
  http.post("/rooms", { roomName, description, avatar });

export const joinRoom = (inviteCode) => http.post("/rooms/join", { inviteCode });

export const getRoomInviteCode = (roomId) => http.get(`/rooms/${roomId}/invite-code`);

export const getConversations = () => http.get("/conversations");

export const getMessageHistory = (conversationId, cursor = 0, limit = 30) =>
  http.get("/messages/history", {
    params: { conversationId, cursor, limit },
  });

export const uploadFile = (file, fileHash = "") => {
  const form = new FormData();
  form.append("file", file);
  if (fileHash) form.append("fileHash", fileHash);
  return http.post("/files", form);
};

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

export const getAttachmentAccessURL = (attachmentId) =>
  http.get(`/files/attachments/${attachmentId}/access-url`);

export default http;
