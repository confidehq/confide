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
		ChoiceOption
	} from '$lib/types/builder';
	import TranslationEditor from './TranslationEditor.svelte';

	interface Props {
		store: ReturnType<typeof createBuilderStore>;
	}

	const { store }: Props = $props();

	const field = $derived(store.selectedField);
	const isConvo = $derived(store.schema.layout === 'convo');

	let expirationSaving = $state(false);
	let expirationError = $state<string | null>(null);

	async function applyExpiration(newExpiresAt: string | null, newResponseLimit: number | null, newTtlDays: number | null, newBurnAfterReading: boolean) {
		expirationSaving = true;
		expirationError = null;
		try {
			await store.setExpiration(newExpiresAt, newResponseLimit, newTtlDays, newBurnAfterReading);
		} catch {
			expirationError = 'Failed to save — please try again.';
		} finally {
			expirationSaving = false;
		}
	}

	type ResponseLifetimePolicy = 'none' | 'burn' | 'ttl';

	let responseLifetimePolicy = $derived<ResponseLifetimePolicy>(
		store.burnAfterReading ? 'burn' : store.responseTtlDays ? 'ttl' : 'none'
	);

	function applyResponseLifetime(policy: ResponseLifetimePolicy, ttlDays: number | null) {
		const burn = policy === 'burn';
		const days = policy === 'ttl' ? ttlDays : null;
		applyExpiration(store.expiresAt, store.responseLimit, days, burn);
	}

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
	class="properties-panel {store.showFormSettings || store.selectedField ? 'is-open' : ''}
		fixed bottom-0 left-0 right-0 max-h-[65vh] rounded-t-xl
		sm:absolute sm:top-2 sm:bottom-2 sm:left-auto sm:right-2 sm:w-[280px] sm:max-h-none sm:rounded-xl
		bg-surface-2 shadow-[0_4px_24px_rgba(0,0,0,0.4)] overflow-y-auto z-20"
>
	<!-- Mobile drag handle — hidden on desktop -->
	<div class="sm:hidden flex justify-center pt-2.5 pb-1 shrink-0 sticky top-0 bg-surface-2">
		<div class="w-8 h-1 bg-border rounded-full"></div>
	</div>

	{#if store.showFormSettings}
		<!-- Form settings panel -->
		<div class="p-4">
			<p class="m-0 mb-4 text-[0.875rem] text-muted-dark uppercase tracking-[0.05em]">Form settings</p>

			<div class="flex flex-col gap-3.5">
				<div>
					<label class="block text-[0.875rem] text-muted mb-1">Form name</label>
					<input
						type="text"
						placeholder="Internal name…"
						value={store.schema.name}
						oninput={(e) => store.setName((e.target as HTMLInputElement).value)}
						class="input-base"
					/>
					<p class="mt-1 m-0 text-[0.8rem] text-muted-dark">Used in your dashboard only.</p>
				</div>

				{#if isConvo}
					<div>
						<label class="block text-[0.875rem] text-muted mb-1">Completion message</label>
						<textarea
							value={store.activeTranslation?.convoCompletionMessage ?? ''}
							oninput={(e) => store.updateTranslation(null, 'convoCompletionMessage', (e.target as HTMLTextAreaElement).value)}
							rows={2}
							class="input-base"
						></textarea>
					</div>

					<div class="flex items-center justify-between">
						<label class="text-[0.925rem] text-text-dim">Allow edit after submit</label>
						<input
							type="checkbox"
							checked={store.schema.convoAllowEdit ?? false}
							onchange={(e) => store.setConvoAllowEdit((e.target as HTMLInputElement).checked)}
						/>
					</div>
				{/if}

				<!-- Access section -->
				<div class="border-t border-border pt-4">
					<p class="m-0 mb-3 text-[0.875rem] text-muted-dark uppercase tracking-[0.05em]">Access</p>
					<div class="flex flex-col gap-3.5">
						<div>
							<label class="block text-[0.875rem] text-muted mb-1">Close form on schedule</label>
							<div class="flex gap-1.5 items-center">
								<input
									type="date"
									value={store.expiresAt ?? ''}
									onchange={(e) => {
										const v = (e.target as HTMLInputElement).value;
										applyExpiration(v || null, store.responseLimit, store.responseTtlDays, store.burnAfterReading);
									}}
									class="input-base"
								/>
								{#if store.expiresAt}
									<button
										onclick={() => applyExpiration(null, store.responseLimit, store.responseTtlDays, store.burnAfterReading)}
										class="bg-transparent border-none text-muted-dark cursor-pointer font-mono text-[1.15rem] px-1 shrink-0"
										title="Clear close date"
									>×</button>
								{/if}
							</div>
							<p class="mt-1 m-0 text-[0.8rem] text-muted-dark">Stop accepting new responses after this date.</p>
						</div>

						<div>
							<label class="block text-[0.875rem] text-muted mb-1">Limit total responses</label>
							<div class="flex gap-1.5 items-center">
								<input
									type="number"
									min="1"
									placeholder="No limit"
									value={store.responseLimit ?? ''}
									onchange={(e) => {
										const v = parseInt((e.target as HTMLInputElement).value);
										applyExpiration(store.expiresAt, v > 0 ? v : null, store.responseTtlDays, store.burnAfterReading);
									}}
									class="input-base"
								/>
								{#if store.responseLimit}
									<button
										onclick={() => applyExpiration(store.expiresAt, null, store.responseTtlDays, store.burnAfterReading)}
										class="bg-transparent border-none text-muted-dark cursor-pointer font-mono text-[1.15rem] px-1 shrink-0"
										title="Clear submission limit"
									>×</button>
								{/if}
							</div>
							<p class="mt-1 m-0 text-[0.8rem] text-muted-dark">Stop accepting responses once this many submissions have been received.</p>
						</div>
					</div>
				</div>

				<!-- Auto delete responses section -->
				<div class="border-t border-border pt-4">
					<p class="m-0 mb-1 text-[0.875rem] text-muted tracking-[0.05em]">Auto delete responses</p>
					<p class="m-0 mb-3 text-[0.8rem] text-muted-dark leading-relaxed">
						Automatically remove a submission from our servers after it has been stored for a set period.
					</p>
					<div class="flex flex-col gap-2.5">
						<select
							value={responseLifetimePolicy}
							onchange={(e) => {
								const policy = (e.target as HTMLSelectElement).value as ResponseLifetimePolicy;
								applyResponseLifetime(policy, policy === 'ttl' ? (store.responseTtlDays ?? 30) : null);
							}}
							class="input-base"
						>
							<option value="none">Keep indefinitely</option>
							<option value="burn">Burn after reading</option>
							<option value="ttl">Delete after a set period</option>
						</select>

						{#if responseLifetimePolicy === 'ttl'}
							<div class="flex gap-1.5 items-center">
								<input
									type="number"
									min="1"
									placeholder="Days"
									value={store.responseTtlDays ?? ''}
									onchange={(e) => {
										const v = parseInt((e.target as HTMLInputElement).value);
										applyResponseLifetime('ttl', v > 0 ? v : null);
									}}
									class="input-base"
								/>
								<span class="text-[0.875rem] text-muted shrink-0">days</span>
							</div>
						{:else if responseLifetimePolicy === 'burn'}
							<p class="m-0 text-[0.8rem] text-muted-dark leading-relaxed">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
						{/if}
					</div>
				</div>

				{#if expirationSaving}
					<p class="m-0 text-[0.8rem] text-muted-dark">Saving…</p>
				{:else if expirationError}
					<p class="m-0 text-[0.8rem] text-error">{expirationError}</p>
				{/if}
			</div>
		</div>

	{:else if field}
		<!-- Field selected: single scrollable panel -->
		<div class="p-4 flex flex-col gap-5">

			<!-- Translation section -->
			<div>
				<p class="m-0 mb-3 text-[0.875rem] text-muted-dark uppercase tracking-[0.05em]">Content</p>
				<TranslationEditor {store} fieldId={field.id} />
			</div>

			<!-- Divider -->
			<div class="h-px bg-border"></div>

			<!-- Settings section -->
			<div>
				<p class="m-0 mb-3 text-[0.875rem] text-muted-dark uppercase tracking-[0.05em]">Settings</p>
				<div class="flex flex-col gap-3.5">

					<!-- Required toggle -->
					<div class="flex items-center justify-between">
						<label class="text-[0.925rem] text-text-dim">Required</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Max length</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Max length</label>
							<input
								type="number"
								min="1"
								value={cfg.maxLength ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { maxLength: parseInt((e.target as HTMLInputElement).value) || undefined })}
								class="input-base"
							/>
						</div>
						<div>
							<label class="block text-[0.875rem] text-muted mb-1">Min rows</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Options</label>
							<div class="flex flex-col gap-1.5">
								{#each cfg.options ?? [] as opt (opt.id)}
									<div class="flex items-center gap-1.5">
										<span class="text-muted-dark text-[0.875rem] min-w-5">{opt.order + 1}.</span>
										<span class="flex-1 text-[0.925rem] text-muted">Option {opt.order + 1}</span>
										<button
											onclick={() => removeOption(opt.id)}
											class="bg-transparent border-none text-muted-dark cursor-pointer font-mono"
										>×</button>
									</div>
								{/each}
								<button
									onclick={addOption}
									class="px-2.5 py-1.5 bg-transparent text-muted-dark border border-dashed border-border rounded cursor-pointer font-mono text-[0.875rem] hover:text-muted transition-colors duration-100"
								>
									+ Add option
								</button>
							</div>
						</div>

						{#if field.type === 'multiple_choice'}
							{@const mcCfg = field.config as MultipleChoiceConfig}
							<div class="flex items-center justify-between">
								<label class="text-[0.925rem] text-text-dim">Allow "Other"</label>
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
								<label class="block text-[0.875rem] text-muted mb-1">Min selections</label>
								<input
									type="number"
									min="0"
									value={cbCfg.minSelect ?? ''}
									oninput={(e) => store.updateFieldConfig(field.id, { minSelect: parseInt((e.target as HTMLInputElement).value) || undefined })}
									class="input-base"
								/>
							</div>
							<div>
								<label class="block text-[0.875rem] text-muted mb-1">Max selections</label>
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
								<label class="text-[0.925rem] text-text-dim">Searchable</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Mode</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Min</label>
							<input
								type="text"
								placeholder="e.g. 2024-01-01"
								value={cfg.min ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { min: (e.target as HTMLInputElement).value || undefined })}
								class="input-base"
							/>
						</div>
						<div>
							<label class="block text-[0.875rem] text-muted mb-1">Max</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Scale</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Shape</label>
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
							<label class="block text-[0.875rem] text-muted mb-1">Level</label>
							<select
								value={cfg.level}
								onchange={(e) => store.updateFieldConfig(field.id, { level: parseInt((e.target as HTMLSelectElement).value) as 0 | 1 | 2 | 3 })}
								class="input-base"
							>
								<option value={0}>Text — Paragraph</option>
								<option value={1}>H1 — Title</option>
								<option value={2}>H2 — Section</option>
								<option value={3}>H3 — Subsection</option>
							</select>
						</div>
					{/if}

					<!-- accent config -->
					{#if field.type === 'accent'}
						{@const cfg = field.config as AccentConfig}
						<div>
							<label class="block text-[0.875rem] text-muted mb-1">Variant</label>
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
					{/if}

					{#if field.type === 'section_break'}
						<p class="text-[0.925rem] text-muted-dark m-0">
							Section breaks have no settings. Use the Content section above to add a label.
						</p>
					{/if}

					{#if field.type === 'accordion'}
						<p class="text-[0.925rem] text-muted-dark m-0">
							Set the title and body text in the Content section above.
						</p>
					{/if}

				</div>
			</div>
		</div>
	{/if}
</aside>
