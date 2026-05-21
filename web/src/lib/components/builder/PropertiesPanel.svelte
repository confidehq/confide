<script lang="ts">
	import type { createBuilderStore } from '$lib/stores/builder.svelte';
	import type {
		ShortTextConfig,
		LongTextConfig,
		MultipleChoiceConfig,
		CheckboxesConfig,
		DropdownConfig,
		DateTimeConfig,
		RatingConfig,
		HeadingConfig,
		AccentConfig,
		AccentIcon,
		ChoiceOption
	} from '$lib/types/builder';
	import { Shield, Lock, CircleCheck, Info, TriangleAlert, Star, Bell, Zap, Ban } from '@lucide/svelte';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
	}

	const { store }: Props = $props();

	const field = $derived(store.selectedField);

	let panelEl = $state<HTMLElement | null>(null);
	let panelTop = $state(8);

	function updatePosition() {
		if (!panelEl) return;
		const anchorId = store.selectedFieldId ?? (store.submitButtonSelected ? '__submit__' : null);
		if (!anchorId) return;
		const fieldEl = document.querySelector<HTMLElement>(`[data-field-id="${anchorId}"]`);
		const container = panelEl.parentElement;
		if (!fieldEl || !container) return;
		const fieldRect = fieldEl.getBoundingClientRect();
		const containerRect = container.getBoundingClientRect();
		panelTop = Math.max(8, fieldRect.top - containerRect.top);
	}

	$effect(() => {
		store.selectedFieldId; // reactive dependency
		store.submitButtonSelected;
		updatePosition();
		const canvas = document.querySelector<HTMLElement>('main[role="presentation"]');
		canvas?.addEventListener('scroll', updatePosition);
		return () => canvas?.removeEventListener('scroll', updatePosition);
	});

	function addOption() {
		if (!field) return;
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const options = cfg.options ?? [];
		const newOpt: ChoiceOption = { id: crypto.randomUUID(), order: options.length };
		store.updateFieldConfig(field.id, { options: [...options, newOpt] } as Partial<typeof cfg>);
	}

	function removeOption(optId: string) {
		if (!field) return;
		const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig;
		const options = (cfg.options ?? [])
			.filter((o) => o.id !== optId)
			.map((o, i) => ({ ...o, order: i }));
		store.updateFieldConfig(field.id, { options } as Partial<typeof cfg>);
	}
</script>

<aside
	bind:this={panelEl}
	style="top: {panelTop}px;"
	class="properties-panel {store.selectedField || store.submitButtonSelected ? 'is-open' : ''}
		fixed bottom-0 left-0 right-0 max-h-[65vh] rounded-t-xl
		sm:absolute sm:bottom-auto sm:left-auto sm:right-2 sm:w-64 sm:max-h-none sm:rounded-xl
		bg-canvas border border-border-deep overflow-y-auto z-20"
>
	<!-- Mobile drag handle — hidden on desktop -->
	<div class="sm:hidden flex justify-center pt-2.5 pb-1 shrink-0 sticky top-0 bg-canvas">
		<div class="w-8 h-1 bg-border rounded-full"></div>
	</div>

	{#if field}
		<!-- Field selected: single scrollable panel -->
		<div class="p-3 flex flex-col gap-3.5">

			<!-- Settings section -->
			<div>
				<div class="flex flex-col gap-3.5">

					<!-- Required toggle -->
					<div class="flex items-center justify-between">
						<label class="text-sm text-text-dim">Required</label>
						<input
							type="checkbox"
							checked={field.required}
							onchange={(e) => store.updateField(field.id, { required: (e.target as HTMLInputElement).checked })}
						/>
					</div>

					<!-- short_text config -->
					{#if field.type === 'short_text'}
						{@const cfg = field.config as ShortTextConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Max length</label>
							<input
								type="number"
								min="1"
								value={cfg.maxLength ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { maxLength: parseInt((e.target as HTMLInputElement).value) || undefined })}
								class="input-base"
							/>
						</div>
					{/if}

					<!-- long_text config -->
					{#if field.type === 'long_text'}
						{@const cfg = field.config as LongTextConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Max length</label>
							<input
								type="number"
								min="1"
								value={cfg.maxLength ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { maxLength: parseInt((e.target as HTMLInputElement).value) || undefined })}
								class="input-base"
							/>
						</div>
						<div>
							<label class="block text-sm text-muted mb-1">Min rows</label>
							<input
								type="number"
								min="1"
								max="20"
								value={cfg.minRows ?? 3}
								oninput={(e) => store.updateFieldConfig(field.id, { minRows: parseInt((e.target as HTMLInputElement).value) || 3 })}
								class="input-base"
							/>
						</div>
					{/if}

					<!-- choice fields config -->
					{#if field.type === 'multiple_choice' || field.type === 'checkboxes' || field.type === 'dropdown'}
						{@const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Options</label>
							<div class="flex flex-col gap-1.5">
								{#each cfg.options ?? [] as opt (opt.id)}
									<div class="flex items-center gap-1.5">
										<span class="text-muted-dark text-sm min-w-5">{opt.order + 1}.</span>
										<span class="flex-1 text-sm text-muted">Option {opt.order + 1}</span>
										<button
											onclick={() => removeOption(opt.id)}
											class="bg-transparent border-none text-muted-dark cursor-pointer font-mono"
										>×</button>
									</div>
								{/each}
								<button
									onclick={addOption}
									class="px-2.5 py-1.5 bg-transparent text-muted-dark border border-dashed border-border rounded cursor-pointer font-mono text-sm hover:text-muted transition-colors duration-100"
								>
									+ Add option
								</button>
							</div>
						</div>

						{#if field.type === 'multiple_choice'}
							{@const mcCfg = field.config as MultipleChoiceConfig}
							<div class="flex items-center justify-between">
								<label class="text-sm text-text-dim">Allow "Other"</label>
								<input
									type="checkbox"
									checked={mcCfg.allowOther ?? false}
									onchange={(e) => store.updateFieldConfig(field.id, { allowOther: (e.target as HTMLInputElement).checked })}
								/>
							</div>
						{/if}

						{#if field.type === 'checkboxes'}
							{@const cbCfg = field.config as CheckboxesConfig}
							<div>
								<label class="block text-sm text-muted mb-1">Min selections</label>
								<input
									type="number"
									min="0"
									value={cbCfg.minSelect ?? ''}
									oninput={(e) => store.updateFieldConfig(field.id, { minSelect: parseInt((e.target as HTMLInputElement).value) || undefined })}
									class="input-base"
								/>
							</div>
							<div>
								<label class="block text-sm text-muted mb-1">Max selections</label>
								<input
									type="number"
									min="0"
									value={cbCfg.maxSelect ?? ''}
									oninput={(e) => store.updateFieldConfig(field.id, { maxSelect: parseInt((e.target as HTMLInputElement).value) || undefined })}
									class="input-base"
								/>
							</div>
						{/if}

						{#if field.type === 'dropdown'}
							{@const ddCfg = field.config as DropdownConfig}
							<div class="flex items-center justify-between">
								<label class="text-sm text-text-dim">Searchable</label>
								<input
									type="checkbox"
									checked={ddCfg.searchable ?? false}
									onchange={(e) => store.updateFieldConfig(field.id, { searchable: (e.target as HTMLInputElement).checked })}
								/>
							</div>
						{/if}
					{/if}

					<!-- date_time config -->
					{#if field.type === 'date_time'}
						{@const cfg = field.config as DateTimeConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Mode</label>
							<select
								value={cfg.mode}
								onchange={(e) => store.updateFieldConfig(field.id, { mode: (e.target as HTMLSelectElement).value as 'date' | 'time' | 'datetime' })}
								class="input-base"
							>
								<option value="date">Date</option>
								<option value="time">Time</option>
								<option value="datetime">Date + time</option>
							</select>
						</div>
						<div>
							<label class="block text-sm text-muted mb-1">Min</label>
							<input
								type="text"
								placeholder="e.g. 2024-01-01"
								value={cfg.min ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { min: (e.target as HTMLInputElement).value || undefined })}
								class="input-base"
							/>
						</div>
						<div>
							<label class="block text-sm text-muted mb-1">Max</label>
							<input
								type="text"
								placeholder="e.g. 2030-12-31"
								value={cfg.max ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { max: (e.target as HTMLInputElement).value || undefined })}
								class="input-base"
							/>
						</div>
					{/if}

					<!-- rating config -->
					{#if field.type === 'rating'}
						{@const cfg = field.config as RatingConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Scale</label>
							<select
								value={cfg.scale}
								onchange={(e) => store.updateFieldConfig(field.id, { scale: parseInt((e.target as HTMLSelectElement).value) as 5 | 10 })}
								class="input-base"
							>
								<option value="5">1 – 5</option>
								<option value="10">1 – 10</option>
							</select>
						</div>
						<div>
							<label class="block text-sm text-muted mb-1">Shape</label>
							<select
								value={cfg.shape}
								onchange={(e) => store.updateFieldConfig(field.id, { shape: (e.target as HTMLSelectElement).value as 'star' | 'number' })}
								class="input-base"
							>
								<option value="star">Stars (★)</option>
								<option value="number">Numbers</option>
							</select>
						</div>
					{/if}

					<!-- heading config -->
					{#if field.type === 'heading'}
						{@const cfg = field.config as HeadingConfig}
						<div>
							<label class="block text-sm text-muted mb-1">Level</label>
							<select
								value={cfg.level}
								onchange={(e) => store.updateFieldConfig(field.id, { level: parseInt((e.target as HTMLSelectElement).value) as 0 | 1 | 2 | 3 | 4 })}
								class="input-base"
							>
								<option value={0}>Text — Paragraph</option>
								<option value={1}>H1 — Title</option>
								<option value={2}>H2 — Section</option>
								<option value={3}>H3 — Subsection</option>
								<option value={4}>Small — Caption</option>
							</select>
						</div>
					{/if}

					<!-- accent config -->
					{#if field.type === 'accent'}
						{@const cfg = field.config as AccentConfig}
						{@const accentIcons: { key: AccentIcon | null; component: typeof Shield | null; label: string }[] = [
							{ key: null,      component: null,          label: 'None' },
							{ key: 'shield',  component: Shield,        label: 'Shield' },
							{ key: 'lock',    component: Lock,          label: 'Lock' },
							{ key: 'check',   component: CircleCheck,   label: 'Check' },
							{ key: 'info',    component: Info,          label: 'Info' },
							{ key: 'alert',   component: TriangleAlert, label: 'Alert' },
							{ key: 'star',    component: Star,          label: 'Star' },
							{ key: 'bell',    component: Bell,          label: 'Bell' },
							{ key: 'zap',     component: Zap,           label: 'Zap' },
						]}
						<div>
							<label class="block text-sm text-muted mb-1">Variant</label>
							<select
								value={cfg.variant}
								onchange={(e) => store.updateFieldConfig(field.id, { variant: (e.target as HTMLSelectElement).value as AccentConfig['variant'] })}
								class="input-base"
							>
								<option value="note">Note</option>
								<option value="warning">Warning</option>
								<option value="danger">Danger</option>
								<option value="success">Success</option>
							</select>
						</div>
						<div>
							<label class="block text-sm text-muted mb-1">Icon</label>
							<div class="flex flex-wrap gap-1">
								{#each accentIcons as icon}
									<button
										type="button"
										title={icon.label}
										onclick={() => store.updateFieldConfig(field.id, { icon: icon.key ?? undefined })}
										class="icon-pick-btn"
										class:active={cfg.icon === icon.key || (!cfg.icon && icon.key === null)}
									>
										{#if icon.component}
											<svelte:component this={icon.component} size={14} />
										{:else}
											<Ban size={14} class="opacity-40" />
										{/if}
									</button>
								{/each}
							</div>
						</div>
					{/if}

					{#if field.type === 'section_break'}
						<p class="text-sm text-muted-dark m-0">
							Section breaks have no settings. Edit the label directly on the field.
						</p>
					{/if}

					{#if field.type === 'accordion'}
						<p class="text-sm text-muted-dark m-0">
							Edit the title and body text directly on the field.
						</p>
					{/if}

				</div>
			</div>
		</div>
	{/if}

	{#if store.submitButtonSelected}
		{@const submitIcons: { key: AccentIcon | undefined; component: typeof Shield | null; label: string }[] = [
			{ key: undefined,  component: null,          label: 'None' },
			{ key: 'lock',     component: Lock,          label: 'Lock' },
			{ key: 'shield',   component: Shield,        label: 'Shield' },
			{ key: 'check',    component: CircleCheck,   label: 'Check' },
			{ key: 'info',     component: Info,          label: 'Info' },
			{ key: 'alert',    component: TriangleAlert, label: 'Alert' },
			{ key: 'star',     component: Star,          label: 'Star' },
			{ key: 'bell',     component: Bell,          label: 'Bell' },
			{ key: 'zap',      component: Zap,           label: 'Zap' },
		]}
		<div class="p-3 flex flex-col gap-3.5">
			<p class="m-0 text-xs font-semibold uppercase tracking-widest text-muted">Submit Button</p>
			<div>
				<label class="block text-sm text-muted mb-1">Icon</label>
				<div class="flex flex-wrap gap-1">
					{#each submitIcons as icon}
						<button
							type="button"
							title={icon.label}
							onclick={() => store.setSubmitButtonIcon(icon.key)}
							class="icon-pick-btn"
							class:active={store.schema.submitButtonIcon === icon.key || (!store.schema.submitButtonIcon && icon.key === undefined)}
						>
							{#if icon.component}
								<svelte:component this={icon.component} size={14} />
							{:else}
								<Ban size={14} class="opacity-40" />
							{/if}
						</button>
					{/each}
				</div>
			</div>
		</div>
	{/if}
</aside>

<style>
	.properties-panel {
		transform: translateY(100%);
		transition: transform 0.2s ease;
	}
	.properties-panel.is-open {
		transform: translateY(0);
	}
	@media (max-width: 639px) {
		/* Neutralise the inline top:{panelTop}px on mobile so bottom-0 anchors the panel */
		.properties-panel {
			top: auto !important;
		}
	}
	@media (min-width: 640px) {
		.properties-panel {
			transform: translateX(calc(100% + 8px));
		}
		.properties-panel.is-open {
			transform: translateX(0);
		}
	}

	.icon-pick-btn {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 30px;
		height: 30px;
		border-radius: 5px;
		border: 1px solid var(--color-border-field);
		background: transparent;
		color: var(--color-muted);
		cursor: pointer;
		transition: background 0.1s, border-color 0.1s, color 0.1s;
	}

	.icon-pick-btn:hover {
		background: var(--color-surface-hover);
		color: var(--color-text-dim);
	}

	.icon-pick-btn.active {
		background: var(--color-surface-active);
		border-color: var(--color-primary);
		color: var(--color-text-bright);
	}
</style>
