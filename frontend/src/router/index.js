import { createRouter, createWebHistory } from "vue-router";
import { hasChatAuth } from "../stores/chat.js";
import ChatView from "../views/ChatView.vue";
import LoginView from "../views/LoginView.vue";

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: "/login", name: "login", component: LoginView, meta: { guestOnly: true } },
    { path: "/chat", name: "chat", component: ChatView, meta: { requiresAuth: true } },
    { path: "/", redirect: "/chat" },
    { path: "/:pathMatch(.*)*", redirect: "/chat" },
  ],
});

router.beforeEach((to) => {
  if (to.meta.requiresAuth && !hasChatAuth()) return { name: "login", query: { redirect: to.fullPath } };
  if (to.meta.guestOnly && hasChatAuth()) return "/chat";
  return true;
});

export default router;
