import { mockCurrentUser } from "../../../mocks/chat.js";

export const clone = (value) => JSON.parse(JSON.stringify(value));

export const authUserProfile = (auth) => ({
  userId: auth?.userId || "",
  username: auth?.username || "",
  nickName: auth?.nickName || "",
  phone: auth?.phone || "",
  avatar: auth?.avatar || "",
});

export const readStoredUser = () => {
  if (typeof window === "undefined") return null;
  try {
    return JSON.parse(
      localStorage.getItem("im_user") || sessionStorage.getItem("im_user") || "null",
    );
  } catch {
    return null;
  }
};

export const initialUser = () => readStoredUser() || clone(mockCurrentUser);

export const persistSession = (auth, remember) => {
  const profile = authUserProfile(auth);
  for (const storage of [localStorage, sessionStorage]) {
    storage.removeItem("im_user");
  }
  const storage = remember ? localStorage : sessionStorage;
  storage.setItem("im_user", JSON.stringify(profile));
};
