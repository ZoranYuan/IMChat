import { computed, onBeforeUnmount, onMounted, ref } from "vue";

const MOBILE_BREAKPOINT = 768;

export function useResponsive() {
  const width = ref(typeof window === "undefined" ? 1280 : window.innerWidth);

  const syncWidth = () => {
    width.value = window.innerWidth;
  };

  onMounted(() => {
    syncWidth();
    window.addEventListener("resize", syncWidth);
  });

  onBeforeUnmount(() => {
    window.removeEventListener("resize", syncWidth);
  });

  return {
    isMobile: computed(() => width.value < MOBILE_BREAKPOINT),
  };
}
