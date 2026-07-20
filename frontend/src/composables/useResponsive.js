import { computed, onBeforeUnmount, onMounted, ref } from "vue";

const MOBILE_BREAKPOINT = 768;
const DESKTOP_BREAKPOINT = 1024;
const LARGE_DESKTOP_BREAKPOINT = 1440;

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
    width,
    isMobile: computed(() => width.value < MOBILE_BREAKPOINT),
    isTablet: computed(
      () => width.value >= MOBILE_BREAKPOINT && width.value < DESKTOP_BREAKPOINT,
    ),
    isDesktop: computed(() => width.value >= DESKTOP_BREAKPOINT),
    isLargeDesktop: computed(() => width.value >= LARGE_DESKTOP_BREAKPOINT),
  };
}
