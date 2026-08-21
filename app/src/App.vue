<script lang="ts">
function installH5ButtonSemantics() {
  if (typeof window === "undefined" || typeof MutationObserver === "undefined") return;
  const enhanced = new WeakSet<HTMLElement>();
  const enhance = () => {
    document.querySelectorAll<HTMLElement>("uni-button").forEach((element) => {
      element.setAttribute("role", "button");
      element.setAttribute("tabindex", element.hasAttribute("disabled") ? "-1" : "0");
      if (enhanced.has(element)) return;
      enhanced.add(element);
      element.addEventListener("keydown", (event) => {
        if ((event.key === "Enter" || event.key === " ") && !element.hasAttribute("disabled")) {
          event.preventDefault();
          element.click();
        }
      });
    });
  };
  enhance();
  new MutationObserver(enhance).observe(document.documentElement, { attributes: true, attributeFilter: ["disabled"], childList: true, subtree: true });
}

export default {
  onLaunch() {
    console.info("Supp Q client launched");
    installH5ButtonSemantics();
  },
};
</script>

<style lang="scss">
page {
  min-height: 100%;
  background: #f7f5f2;
  color: #292824;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", "PingFang SC",
    "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
}

view,
text,
button,
main,
section,
nav {
  box-sizing: border-box;
}

button::after {
  border: 0;
}
</style>
