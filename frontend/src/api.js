const API_BASE = "/api/v1";

export class ApiError extends Error {
  constructor(message, code) {
    super(message);
    this.name = "ApiError";
    this.code = code;
  }
}

export function authHeaders(token) {
  return token ? { Authorization: `Bearer ${token}` } : {};
}

export async function request(path, options = {}) {
  const headers = {
    ...(options.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
    ...authHeaders(options.token),
    ...options.headers,
  };

  const res = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });
  const payload = await res.json().catch(() => ({}));

  if (!res.ok || (payload.code && payload.code !== 200)) {
    throw new ApiError(payload.message || `请求失败：${res.status}`, payload.code || res.status);
  }
  return payload.data ?? payload;
}

export function login(form) {
  return request("/users/login", {
    method: "POST",
    body: JSON.stringify({
      loginType: 1,
      phone: form.phone,
      password: form.password,
    }),
  });
}

export function register(form) {
  return request("/users/register", {
    method: "POST",
    body: JSON.stringify({
      loginType: 1,
      phone: form.phone,
      password: form.password,
      reconfirmPassword: form.password,
    }),
  });
}

export function getOfflineMessages(token) {
  return request("/messages/offline", { token });
}

export function getHistoryMessages(token, conversationId, cursor = 0, limit = 30) {
  const params = new URLSearchParams({
    conversationId,
    cursor: String(cursor),
    limit: String(limit),
  });
  return request(`/messages/history?${params.toString()}`, { token });
}

export function getDanmaku(token, roomId, startTime = 0, endTime = Date.now(), limit = 200) {
  const params = new URLSearchParams({
    roomId,
    startTime: String(startTime),
    endTime: String(endTime),
    limit: String(limit),
  });
  return request(`/messages/danmaku?${params.toString()}`, { token });
}

export function createRoom(token, form) {
  return request("/rooms", {
    method: "POST",
    token,
    body: JSON.stringify({
      roomName: form.roomName,
      avatar: form.avatar || "",
      description: form.description || "",
    }),
  });
}

export function joinRoom(token, inviteCode) {
  return request("/rooms/join", {
    method: "POST",
    token,
    body: JSON.stringify({ inviteCode }),
  });
}

export function getInviteCode(token, roomId) {
  return request(`/rooms/${roomId}/invite-code`, { token });
}

export function uploadFile(token, file) {
  const form = new FormData();
  form.append("file", file);
  return request("/files", {
    method: "POST",
    token,
    body: form,
  });
}

export function getFile(token, fileId) {
  return request(`/files/${fileId}`, { token });
}
