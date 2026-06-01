<script lang="ts">
import {
	Bell,
	CircleCheck,
	Info,
	Lock,
	Shield,
	Star,
	TriangleAlert,
	Zap,
} from "@lucide/svelte";
import type { AccentConfig, BuilderField } from "$lib/types/builder";

interface Props {
	field: BuilderField;
	translation: { label: string; helpText?: string };
}

const { field, translation }: Props = $props();

const cfg = $derived(field.config as AccentConfig);
const variant = $derived(cfg.variant ?? "note");

const styles = $derived(
	{
		note: {
			border: "var(--color-info-border)",
			bg: "var(--color-info-bg)",
			color: "var(--color-info-text)",
		},
		warning: {
			border: "var(--color-warning-border)",
			bg: "var(--color-warning-bg)",
			color: "var(--color-warning-text)",
		},
		danger: {
			border: "var(--color-danger-border)",
			bg: "var(--color-danger-bg)",
			color: "var(--color-danger-text)",
		},
		success: {
			border: "var(--color-success-border)",
			bg: "var(--color-success-bg)",
			color: "var(--color-success-text)",
		},
	}[variant],
);

const iconMap = {
	shield: Shield,
	lock: Lock,
	check: CircleCheck,
	info: Info,
	alert: TriangleAlert,
	star: Star,
	bell: Bell,
	zap: Zap,
};
const IconComponent = $derived(cfg.icon ? iconMap[cfg.icon] : null);
</script>

<div
	style="border-color: {styles.border}; background: {styles.bg}; color: {styles.color};"
	class="px-4 py-3 border rounded text-base leading-relaxed"
>
	<div class="flex items-center gap-2">
		{#if IconComponent}
			<IconComponent size={16} class="shrink-0 opacity-80" />
		{/if}
		<div class="m-0 font-semibold rich-html">{@html translation.label}</div>
	</div>
	{#if translation.helpText}
		<div class="mt-1 m-0 rich-html">{@html translation.helpText}</div>
	{/if}
</div>
