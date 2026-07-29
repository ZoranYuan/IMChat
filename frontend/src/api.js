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
  const token = config.token || localStorage.getItem("im_token") || sessionStorage.getItem("im_token");
  if (token) config.headers.Authorization = `Bearer ${token}`;
  delete config.token;
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

export const logoutUser = (token) => http.post("/users/logout", {}, { token });

export const getUser = (token, userId) => http.get(`/users/${userId}`, { token });

export const getFriends = (token) => http.get("/friends", { token });

export const getFriendRequests = (token) => http.get("/friend-requests", { token });

export const createFriendRequest = (token, { toUserId, message }) =>
  http.post("/friend-requests", { toUserId, message }, { token });

export const operateFriendRequest = (token, { requestId, action }) =>
  http.post("/friend-requests/actions", { requestId, action }, { token });

export const createRoom = (token, { roomName, description = "", avatar = "" }) =>
  http.post("/rooms", { roomName, description, avatar }, { token });

export const joinRoom = (token, inviteCode) =>
  http.post("/rooms/join", { inviteCode }, { token });

export const getRoomInviteCode = (token, roomId) =>
  http.get(`/rooms/${roomId}/invite-code`, { token });

export const getConversations = (token) => http.get("/conversations", { token });

export const getMessageHistory = (token, conversationId, cursor = 0, limit = 30) =>
  http.get("/messages/history", {
    token,
    params: { conversationId, cursor, limit },
  });

export const uploadFile = (token, file) => {
  const form = new FormData();
  form.append("file", file);
  return http.post("/files", form, { token });
};

export const initMultipartUpload = (token, payload) =>
  http.post("/files/multipart/init", payload, { token });

export const uploadMultipartPart = (token, uploadId, partNumber, chunk, chunkHash) => {
  const form = new FormData();
  form.append("chunk", chunk);
  form.append("chunkHash", chunkHash);
  return http.put(`/files/multipart/${uploadId}/parts/${partNumber}`, form, { token });
};

export const completeMultipartUpload = (token, uploadId) =>
  http.post(`/files/multipart/${uploadId}/complete`, {}, { token });

export const getFile = (token, fileId) => http.get(`/files/${fileId}`, { token });

export default http;
