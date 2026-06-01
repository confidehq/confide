<script lang="ts">
  type Status = 'open' | 'closed' | 'draft';

  let {
    status,
    label,
    class: extraClass = '',
  }: {
    status: Status;
    label?: string;
    class?: string;
  } = $props();

  const colors: Record<Status, string> = {
    open:   'bg-open-bg text-open-text border-open-border',
    closed: 'bg-closed-bg text-closed-text border-closed-border',
    draft:  'bg-warn-dark text-warn border-warn-light',
  };
</script>

<span class="shrink-0 px-1.5 py-px text-sm rounded-full border {colors[status]} {extraClass}">
  {label ?? status}
</span>
