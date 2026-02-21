<script lang="ts">
  import { t } from "svelte-i18n";
  import { Spinner } from "$shared/ui";

  // The redirecting is already handled in +layout.svelte, so we don't need to handle it here.
</script>

<div class="">
  <Spinner />
  <p>{$t("setup.redirecting")}</p>
</div>
