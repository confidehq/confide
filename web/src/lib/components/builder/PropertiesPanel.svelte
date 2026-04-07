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

	function inputStyle(): string {
		return `
			width: 100%; padding: 6px 10px;
			background: #111827;
			border: 1px solid #374151;
			border-radius: 4px;
			color: #d1d5db;
			font-family: monospace; font-size: 0.925rem;
			outline: none; box-sizing: border-box;
		`;
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

<aside style="
	position: absolute;
	top: 8px; bottom: 8px; right: 8px;
	width: 280px;
	background: #1f2937;
	border-radius: 8px;
	box-shadow: 0 4px 24px rgba(0,0,0,0.4);
	overflow-y: auto;
	z-index: 20;
	transform: translateX({store.showFormSettings || store.selectedField ? '0' : 'calc(100% + 16px)'});
	transition: transform 220ms cubic-bezier(0.4, 0, 0.2, 1);
">
	{#if store.showFormSettings}
		<!-- Form settings panel -->
		<div style="padding: 16px;">
			<p style="margin: 0 0 16px; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">
				Form settings
			</p>

			<div style="display: flex; flex-direction: column; gap: 14px;">
				<div>
					<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Form name</label>
					<input
						type="text"
						placeholder="Internal name…"
						value={store.schema.name}
						oninput={(e) => store.setName((e.target as HTMLInputElement).value)}
						style={inputStyle()}
					/>
					<p style="margin: 4px 0 0; font-size: 0.8rem; color: #4b5563;">Used in your dashboard only.</p>
				</div>

				{#if isConvo}
					<div>
						<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">
							Completion message
						</label>
						<textarea
							value={store.activeTranslation?.convoCompletionMessage ?? ''}
							oninput={(e) => store.updateTranslation(null, 'convoCompletionMessage', (e.target as HTMLTextAreaElement).value)}
							rows={2}
							style={inputStyle()}
						></textarea>
					</div>

					<div style="display: flex; align-items: center; justify-content: space-between;">
						<label style="font-size: 0.925rem; color: #d1d5db;">Allow edit after submit</label>
						<input
							type="checkbox"
							checked={store.schema.convoAllowEdit ?? false}
							onchange={(e) => store.setConvoAllowEdit((e.target as HTMLInputElement).checked)}
						/>
					</div>
				{/if}

				<!-- Access section -->
				<div style="border-top: 1px solid #374151; padding-top: 16px;">
					<p style="margin: 0 0 12px; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">
						Access
					</p>
					<div style="display: flex; flex-direction: column; gap: 14px;">
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Close form on schedule</label>
							<div style="display: flex; gap: 6px; align-items: center;">
								<input
									type="date"
									value={store.expiresAt ?? ''}
									onchange={(e) => {
										const v = (e.target as HTMLInputElement).value;
										applyExpiration(v || null, store.responseLimit, store.responseTtlDays, store.burnAfterReading);
									}}
									style={inputStyle()}
								/>
								{#if store.expiresAt}
									<button
										onclick={() => applyExpiration(null, store.responseLimit, store.responseTtlDays, store.burnAfterReading)}
										style="background: transparent; border: none; color: #6b7280; cursor: pointer; font-family: monospace; font-size: 1.15rem; padding: 0 4px; flex-shrink: 0;"
										title="Clear close date"
									>×</button>
								{/if}
							</div>
							<p style="margin: 4px 0 0; font-size: 0.8rem; color: #4b5563;">Stop accepting new responses after this date.</p>
						</div>

						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Limit total responses</label>
							<div style="display: flex; gap: 6px; align-items: center;">
								<input
									type="number"
									min="1"
									placeholder="No limit"
									value={store.responseLimit ?? ''}
									onchange={(e) => {
										const v = parseInt((e.target as HTMLInputElement).value);
										applyExpiration(store.expiresAt, v > 0 ? v : null, store.responseTtlDays, store.burnAfterReading);
									}}
									style={inputStyle()}
								/>
								{#if store.responseLimit}
									<button
										onclick={() => applyExpiration(store.expiresAt, null, store.responseTtlDays, store.burnAfterReading)}
										style="background: transparent; border: none; color: #6b7280; cursor: pointer; font-family: monospace; font-size: 1.15rem; padding: 0 4px; flex-shrink: 0;"
										title="Clear submission limit"
									>×</button>
								{/if}
							</div>
							<p style="margin: 4px 0 0; font-size: 0.8rem; color: #4b5563;">Stop accepting responses once this many submissions have been received.</p>
						</div>
					</div>
				</div>

				<!-- Auto delete responses section -->
				<div style="border-top: 1px solid #374151; padding-top: 16px;">
					<p style="margin: 0 0 4px; font-size: 0.875rem; color: #9ca3af; letter-spacing: 0.05em;">
						Auto delete responses
					</p>
					<p style="margin: 0 0 12px; font-size: 0.8rem; color: #4b5563; line-height: 1.5;">
						Automatically remove a submission from our servers after it has been stored for a set period.
					</p>
					<div style="display: flex; flex-direction: column; gap: 10px;">
						<select
							value={responseLifetimePolicy}
							onchange={(e) => {
								const policy = (e.target as HTMLSelectElement).value as ResponseLifetimePolicy;
								applyResponseLifetime(policy, policy === 'ttl' ? (store.responseTtlDays ?? 30) : null);
							}}
							style={inputStyle()}
						>
							<option value="none">Keep indefinitely</option>
							<option value="burn">Burn after reading</option>
							<option value="ttl">Delete after a set period</option>
						</select>

						{#if responseLifetimePolicy === 'ttl'}
							<div style="display: flex; gap: 6px; align-items: center;">
								<input
									type="number"
									min="1"
									placeholder="Days"
									value={store.responseTtlDays ?? ''}
									onchange={(e) => {
										const v = parseInt((e.target as HTMLInputElement).value);
										applyResponseLifetime('ttl', v > 0 ? v : null);
									}}
									style={inputStyle()}
								/>
								<span style="font-size: 0.875rem; color: #9ca3af; flex-shrink: 0;">days</span>
							</div>
						{:else if responseLifetimePolicy === 'burn'}
							<p style="margin: 0; font-size: 0.8rem; color: #4b5563; line-height: 1.5;">Responses are scheduled for deletion once you view them. They remain visible until the next cleanup pass.</p>
						{/if}
					</div>
				</div>

				{#if expirationSaving}
					<p style="margin: 0; font-size: 0.8rem; color: #6b7280;">Saving…</p>
				{:else if expirationError}
					<p style="margin: 0; font-size: 0.8rem; color: #ef4444;">{expirationError}</p>
				{/if}
			</div>
		</div>

	{:else if field}
		<!-- Field selected: single scrollable panel -->
		<div style="padding: 16px; display: flex; flex-direction: column; gap: 20px;">

			<!-- Translation section -->
			<div>
				<p style="margin: 0 0 12px; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Content</p>
				<TranslationEditor {store} fieldId={field.id} />
			</div>

			<!-- Divider -->
			<div style="height: 1px; background: #374151;"></div>

			<!-- Settings section -->
			<div>
				<p style="margin: 0 0 12px; font-size: 0.875rem; color: #6b7280; text-transform: uppercase; letter-spacing: 0.05em;">Settings</p>
				<div style="display: flex; flex-direction: column; gap: 14px;">

					<!-- Required toggle -->
					<div style="display: flex; align-items: center; justify-content: space-between;">
						<label style="font-size: 0.925rem; color: #d1d5db;">Required</label>
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
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Max length</label>
							<input
								type="number"
								min="1"
								value={cfg.maxLength ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { maxLength: parseInt((e.target as HTMLInputElement).value) || undefined })}
								style={inputStyle()}
							/>
						</div>
					{/if}

					<!-- long_text config -->
					{#if field.type === 'long_text'}
						{@const cfg = field.config as LongTextConfig}
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Max length</label>
							<input
								type="number"
								min="1"
								value={cfg.maxLength ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { maxLength: parseInt((e.target as HTMLInputElement).value) || undefined })}
								style={inputStyle()}
							/>
						</div>
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Min rows</label>
							<input
								type="number"
								min="1"
								max="20"
								value={cfg.minRows ?? 3}
								oninput={(e) => store.updateFieldConfig(field.id, { minRows: parseInt((e.target as HTMLInputElement).value) || 3 })}
								style={inputStyle()}
							/>
						</div>
					{/if}

					<!-- choice fields config -->
					{#if field.type === 'multiple_choice' || field.type === 'checkboxes' || field.type === 'dropdown'}
						{@const cfg = field.config as MultipleChoiceConfig | CheckboxesConfig | DropdownConfig}
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Options</label>
							<div style="display: flex; flex-direction: column; gap: 6px;">
								{#each cfg.options ?? [] as opt (opt.id)}
									<div style="display: flex; align-items: center; gap: 6px;">
										<span style="color: #6b7280; font-size: 0.875rem; min-width: 20px;">{opt.order + 1}.</span>
										<span style="flex: 1; font-size: 0.925rem; color: #9ca3af;">Option {opt.order + 1}</span>
										<button
											onclick={() => removeOption(opt.id)}
											style="background: transparent; border: none; color: #6b7280; cursor: pointer; font-family: monospace;"
										>
											×
										</button>
									</div>
								{/each}
								<button
									onclick={addOption}
									style="
										padding: 6px 10px;
										background: transparent;
										color: #6b7280;
										border: 1px dashed #374151;
										border-radius: 4px;
										cursor: pointer;
										font-family: monospace;
										font-size: 0.875rem;
									"
								>
									+ Add option
								</button>
							</div>
						</div>

						{#if field.type === 'multiple_choice'}
							{@const mcCfg = field.config as MultipleChoiceConfig}
							<div style="display: flex; align-items: center; justify-content: space-between;">
								<label style="font-size: 0.925rem; color: #d1d5db;">Allow "Other"</label>
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
								<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Min selections</label>
								<input
									type="number"
									min="0"
									value={cbCfg.minSelect ?? ''}
									oninput={(e) => store.updateFieldConfig(field.id, { minSelect: parseInt((e.target as HTMLInputElement).value) || undefined })}
									style={inputStyle()}
								/>
							</div>
							<div>
								<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Max selections</label>
								<input
									type="number"
									min="0"
									value={cbCfg.maxSelect ?? ''}
									oninput={(e) => store.updateFieldConfig(field.id, { maxSelect: parseInt((e.target as HTMLInputElement).value) || undefined })}
									style={inputStyle()}
								/>
							</div>
						{/if}

						{#if field.type === 'dropdown'}
							{@const ddCfg = field.config as DropdownConfig}
							<div style="display: flex; align-items: center; justify-content: space-between;">
								<label style="font-size: 0.925rem; color: #d1d5db;">Searchable</label>
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
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Mode</label>
							<select
								value={cfg.mode}
								onchange={(e) => store.updateFieldConfig(field.id, { mode: (e.target as HTMLSelectElement).value as 'date' | 'time' | 'datetime' })}
								style={inputStyle()}
							>
								<option value="date">Date</option>
								<option value="time">Time</option>
								<option value="datetime">Date + time</option>
							</select>
						</div>
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Min</label>
							<input
								type="text"
								placeholder="e.g. 2024-01-01"
								value={cfg.min ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { min: (e.target as HTMLInputElement).value || undefined })}
								style={inputStyle()}
							/>
						</div>
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Max</label>
							<input
								type="text"
								placeholder="e.g. 2030-12-31"
								value={cfg.max ?? ''}
								oninput={(e) => store.updateFieldConfig(field.id, { max: (e.target as HTMLInputElement).value || undefined })}
								style={inputStyle()}
							/>
						</div>
					{/if}

					<!-- rating config -->
					{#if field.type === 'rating'}
						{@const cfg = field.config as RatingConfig}
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Scale</label>
							<select
								value={cfg.scale}
								onchange={(e) => store.updateFieldConfig(field.id, { scale: parseInt((e.target as HTMLSelectElement).value) as 5 | 10 })}
								style={inputStyle()}
							>
								<option value="5">1 – 5</option>
								<option value="10">1 – 10</option>
							</select>
						</div>
						<div>
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Shape</label>
							<select
								value={cfg.shape}
								onchange={(e) => store.updateFieldConfig(field.id, { shape: (e.target as HTMLSelectElement).value as 'star' | 'number' })}
								style={inputStyle()}
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
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Level</label>
							<select
								value={cfg.level}
								onchange={(e) => store.updateFieldConfig(field.id, { level: parseInt((e.target as HTMLSelectElement).value) as 0 | 1 | 2 | 3 })}
								style={inputStyle()}
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
							<label style="display: block; font-size: 0.875rem; color: #9ca3af; margin-bottom: 4px;">Variant</label>
							<select
								value={cfg.variant}
								onchange={(e) => store.updateFieldConfig(field.id, { variant: (e.target as HTMLSelectElement).value as AccentConfig['variant'] })}
								style={inputStyle()}
							>
								<option value="note">Note</option>
								<option value="warning">Warning</option>
								<option value="danger">Danger</option>
								<option value="success">Success</option>
							</select>
						</div>
					{/if}

					{#if field.type === 'section_break'}
						<p style="font-size: 0.925rem; color: #6b7280; margin: 0;">
							Section breaks have no settings. Use the Content section above to add a label.
						</p>
					{/if}

					{#if field.type === 'accordion'}
						<p style="font-size: 0.925rem; color: #6b7280; margin: 0;">
							Set the title and body text in the Content section above.
						</p>
					{/if}

				</div>
			</div>
		</div>
	{/if}
</aside>
