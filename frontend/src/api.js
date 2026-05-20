import axios from "axios";

export class ApiError extends Error {
  constructor(message, code, raw) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.raw = raw;
  }
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
      if (payload.code !== 200) {
        throw new ApiError(payload.message || "请求失败", payload.code, payload);
      }
      return payload.data ?? payload;
    }
    return payload;
  },
  (error) => {
    const payload = error.response?.data;
    if (payload && typeof payload === "object") {
      throw new ApiError(payload.message || error.message, payload.code || error.response?.status, payload);
    }
    throw new ApiError(error.message || "网络异常", error.response?.status, error);
  },
);

export function login(form) {
  return http.post("/users/login", {
    loginType: 1,
    phone: form.phone,
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

export function getDanmaku(token, roomId, startTime = 0, endTime = Date.now(), limit = 200) {
  return http.get("/messages/danmaku", {
    token,
    params: {
      roomId,
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

export function getFile(token, fileId) {
  return http.get(`/files/${fileId}`, { token });
}
