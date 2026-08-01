<script lang="ts">
export default {
  onLaunch() {
    console.info("Supp Q client launched");
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
