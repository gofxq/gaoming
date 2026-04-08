const MOBILE_USER_AGENT_PATTERN =
  /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i;

export function shouldUsePwaLayout() {
  if (typeof window === "undefined") {
    return false;
  }

  const userAgent = navigator.userAgent || navigator.vendor || "";
  const hasMobileUserAgent = MOBILE_USER_AGENT_PATTERN.test(userAgent);
  const hasCoarsePointer = window.matchMedia("(pointer: coarse)").matches;
  const isNarrowViewport = window.matchMedia("(max-width: 960px)").matches;

  return hasMobileUserAgent || (hasCoarsePointer && isNarrowViewport);
}
