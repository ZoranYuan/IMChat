import axios from "axios";

export class ApiError extends Error {
  constructor(message, { code = 0, status = 0, data = null, raw = null } = {}) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
    this.data = data;
    this.raw = raw;
  }
}

function isSuccessCode(code) {
  return code === 200 || code === 0;
}

function toApiError(error, fallback = "请求失败") {
  const response = error?.response;
  const payload = response?.data;

  if (payload && typeof payload === "object") {
    const message = payload.message || payload.msg || fallback;
    return new ApiError(message, {
      code: payload.code ?? response?.status ?? 0,
      status: response?.status ?? 0,
      data: payload.data ?? null,
      raw: payload,
    });
  }

  if (response) {
    return new ApiError(response.statusText || fallback, {
      code: response.status,
      status: response.status,
      raw: response,
    });
  }

  return new ApiError(error?.message || "网络异常", {
    code: error?.code || 0,
    raw: error,
  });
}

export const http = axios.create({
  baseURL: "/api/v1",
  timeout: 15000,
});

http.interceptors.request.use((config) => {
  const token = config.token || localStorage.getItem("im_token");
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  delete config.token;
  return config;
});

http.interceptors.response.use(
  (response) => {
    const payload = response.data;
    if (payload && typeof payload === "object" && "code" in payload) {
      if (!isSuccessCode(payload.code)) {
        throw new ApiError(payload.message || payload.msg || "请求失败", {
          code: payload.code,
          status: response.status,
          data: payload.data ?? null,
          raw: payload,
        });
      }
      return payload.data ?? payload;
    }
    return payload;
  },
  (error) => {
    throw toApiError(error);
  },
);

export function login(form) {
  return http.post("/users/login", {
    loginType: 1,
    phone: (form.phone || "").trim(),
    password: form.password,
  });
}

export function register(form) {
  return http.post("/users/register", {
    loginType: 1,
    phone: form.phone,
    password: form.password,
    reconfirmPassword: form.password,
  });
}

export function getOfflineMessages(token) {
  return http.get("/messages/offline", { token });
}

export function getHistoryMessages(token, conversationId, cursor = 0, limit = 30) {
  return http.get("/messages/history", {
    token,
    params: {
      conversationId,
      cursor,
      limit,
    },
  });
}

export function getDanmaku(token, roomId, videoId, startTime = 0, endTime = 0, limit = 200) {
  return http.get("/messages/danmaku", {
    token,
    params: {
      roomId,
      videoId,
      startTime,
      endTime,
      limit,
    },
  });
}

export function createRoom(token, form) {
  return http.post(
    "/rooms",
    {
      roomName: form.roomName,
      avatar: form.avatar || "",
      description: form.description || "",
    },
    { token },
  );
}

export function joinRoom(token, inviteCode) {
  return http.post("/rooms/join", { inviteCode }, { token });
}

export function getInviteCode(token, roomId) {
  return http.get(`/rooms/${roomId}/invite-code`, { token });
}

export function uploadFile(token, file) {
  const form = new FormData();
  form.append("file", file);
  return http.post("/files", form, { token });
}

export function initMultipartUpload(token, payload) {
  return http.post("/files/multipart/init", payload, { token });
}

export function uploadMultipartPart(token, uploadId, partNumber, chunk, chunkHash = "") {
  const form = new FormData();
  form.append("chunk", chunk);
  form.append("chunkHash", chunkHash);
  return http.put(`/files/multipart/${uploadId}/parts/${partNumber}`, form, { token });
}

export function completeMultipartUpload(token, uploadId) {
  return http.post(`/files/multipart/${uploadId}/complete`, {}, { token });
}

export function getFile(token, fileId) {
  return http.get(`/files/${fileId}`, { token });
}

export function getFriends(token) {
  return http.get("/friends", { token });
}

export function resolveUser(token, keyword) {
  return http.get("/users/resolve", {
    token,
    params: { keyword },
  });
}

export function createFriendRequest(token, form) {
  return http.post(
    "/friend-requests",
    {
      toUserId: form.toUserId,
      message: form.message || "你好，我想加你为好友",
    },
    { token },
  );
}

export function getFriendRequests(token) {
  return http.get("/friend-requests", { token });
}

export function operateFriendRequest(token, requestId, action) {
  return http.post(
    "/friend-requests/actions",
    {
      requestId,
      action,
    },
    { token },
  );
}
