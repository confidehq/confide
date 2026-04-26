/**
 * Confide form builder store (Svelte 5 Runes).
 *
 * Create one instance per builder page via createBuilderStore(masterKey, formId).
 * Pass the returned store object into child components via setContext/getContext.
 */

import { updateFormSchema, updateFormExpiration, getForm } from '$lib/forms';
import type { BuilderSchema, BuilderField, FieldType, FieldConfig, TranslationMap } from '$lib/types/builder';
import { getOrderedFields } from '$lib/types/builder';
import { formsStore } from '$lib/stores/forms.svelte';

export type BuilderMode = 'edit' | 'preview';

export interface BuilderStore {
	// State (readable)
	readonly schema: BuilderSchema;
	readonly saving: boolean;
	readonly lastSaved: Date | null;
	readonly dirty: boolean;
	readonly activeLocale: string;
	readonly selectedFieldId: string | null;
	readonly mode: BuilderMode;
	readonly renderKeySalt: Uint8Array | null;
	readonly formKey: CryptoKey | null;
	readonly expiresAt: string | null;
	readonly responseLimit: number | null;
	readonly responseTtlDays: number | null;
	readonly burnAfterReading: boolean;
	readonly showFormSettings: boolean;
	readonly formStatus: string;
	readonly hasUnpublishedChanges: boolean;

	// Derived (readable)
	readonly selectedField: BuilderField | null;
	readonly activeTranslation: TranslationMap;

	// Actions
	setRenderKeySalt(salt: Uint8Array): void;
	markPublished(): void;
	setShowFormSettings(show: boolean): void;
	addField(type: FieldType): void;
	addFieldAt(type: FieldType, afterIndex: number): void;
	removeField(id: string): void;
	reorderFields(newOrder: BuilderField[]): void;
	updateField(id: string, patch: Partial<BuilderField>): void;
	updateFieldConfig(id: string, patch: Partial<FieldConfig>): void;
	updateTranslation(fieldId: string | null, key: string, value: string): void;
	addLocale(locale: string): void;
	removeLocale(locale: string): void;
	setLayout(layout: BuilderSchema['layout']): void;
	setActiveLocale(locale: string): void;
	setSelectedField(id: string | null): void;
	setMode(mode: BuilderMode): void;
	setConvoAllowEdit(allow: boolean): void;
	setExpiration(expiresAt: string | null, responseLimit: number | null, responseTtlDays: number | null, burnAfterReading: boolean): Promise<void>;
	load(): Promise<void>;
	save(): Promise<void>;
	flushSave(): Promise<void>;
}

export function emptySchema(): BuilderSchema {
	return {
		version: 1,
		defaultLocale: 'en',
		locales: ['en'],
		layout: 'scroll',
		fields: [],
		translations: {
			en: {
				formTitle: '',
				formDescription: '',
				fields: {}
			}
		}
	};
}

function defaultConfigForType(type: FieldType): FieldConfig {
	switch (type) {
		case 'short_text':
			return {};
		case 'long_text':
			return { minRows: 3 };
		case 'multiple_choice':
			return { options: [{ id: crypto.randomUUID(), order: 0 }] };
		case 'checkboxes':
			return { options: [{ id: crypto.randomUUID(), order: 0 }] };
		case 'dropdown':
			return { options: [{ id: crypto.randomUUID(), order: 0 }] };
		case 'date_time':
			return { mode: 'date' };
		case 'rating':
			return { scale: 5, shape: 'star' };
		case 'section_break':
			return {};
		case 'heading':
			return { level: 2 };
		case 'accordion':
			return {};
		case 'accent':
			return { variant: 'note' };
	}
}

export function createBuilderStore(masterKey: CryptoKey, formId: string): BuilderStore {
	let schema = $state<BuilderSchema>(emptySchema());
	let saving = $state(false);
	let lastSaved = $state<Date | null>(null);
	let dirty = $state(false);
	let activeLocale = $state('en');
	let selectedFieldId = $state<string | null>(null);
	let mode = $state<BuilderMode>('edit');
	let expiresAt = $state<string | null>(null);
	let responseLimit = $state<number | null>(null);
	let responseTtlDays = $state<number | null>(null);
	let burnAfterReading = $state(false);
	let showFormSettings = $state(false);
	let formStatus = $state('draft');
	let hasUnpublishedChanges = $state(true);

	// Debounce timer handle
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	// Stable salt for the render key (loaded from server or generated on first save)
	let currentRenderKeySalt: Uint8Array | null = null;
	// The resolved form key — may come from workspace key path for non-owners
	let resolvedFormKey: CryptoKey | null = null;

	// Computed via getters in the returned object — no $derived needed.
	// Svelte tracks these reactively because they read $state variables.

	// Watch schema changes and debounce auto-save
	$effect(() => {
		// Access schema to subscribe to changes
		const _s = schema;
		void _s;
		if (!dirty) return;
		if (debounceTimer) clearTimeout(debounceTimer);
		debounceTimer = setTimeout(() => {
			void save();
		}, 2000);
	});

	function markDirty() {
		dirty = true;
	}

	function ensureLocaleTranslation(locale: string) {
		if (!schema.translations[locale]) {
			schema = {
				...schema,
				translations: {
					...schema.translations,
					[locale]: {
						formTitle: '',
						formDescription: '',
						fields: {}
					}
				}
			};
		}
	}

	function ensureFieldTranslation(locale: string, fieldId: string) {
		ensureLocaleTranslation(locale);
		const t = schema.translations[locale];
		if (!t.fields[fieldId]) {
			schema = {
				...schema,
				translations: {
					...schema.translations,
					[locale]: {
						...t,
						fields: {
							...t.fields,
							[fieldId]: { label: '' }
						}
					}
				}
			};
		}
	}

	function addField(type: FieldType): void {
		const id = crypto.randomUUID();
		const order = schema.fields.length;
		const newField: BuilderField = {
			id,
			type,
			required: false,
			order,
			config: defaultConfigForType(type)
		};

		// Ensure translation slots exist for all locales
		const updatedTranslations = { ...schema.translations };
		for (const locale of schema.locales) {
			if (!updatedTranslations[locale]) {
				updatedTranslations[locale] = { formTitle: '', formDescription: '', fields: {} };
			}
			updatedTranslations[locale] = {
				...updatedTranslations[locale],
				fields: {
					...updatedTranslations[locale].fields,
					[id]: { label: '' }
				}
			};
		}

		// Append new field ID to every locale-specific order list
		const updatedFieldOrders: Record<string, string[]> = {};
		if (schema.fieldOrders) {
			for (const [loc, ids] of Object.entries(schema.fieldOrders)) {
				updatedFieldOrders[loc] = [...ids, id];
			}
		}

		schema = {
			...schema,
			fields: [...schema.fields, newField],
			translations: updatedTranslations,
			...(schema.fieldOrders ? { fieldOrders: updatedFieldOrders } : {})
		};
		selectedFieldId = id;
		markDirty();
	}

	function addFieldAt(type: FieldType, afterIndex: number): void {
		const orderedIds = getOrderedFields(schema, activeLocale).map((f) => f.id);
		const id = crypto.randomUUID();
		const newField: BuilderField = {
			id,
			type,
			required: false,
			order: schema.fields.length,
			config: defaultConfigForType(type)
		};

		const updatedTranslations = { ...schema.translations };
		for (const locale of schema.locales) {
			if (!updatedTranslations[locale]) {
				updatedTranslations[locale] = { formTitle: '', formDescription: '', fields: {} };
			}
			updatedTranslations[locale] = {
				...updatedTranslations[locale],
				fields: { ...updatedTranslations[locale].fields, [id]: { label: '' } }
			};
		}

		const baseIds = [...orderedIds];
		baseIds.splice(afterIndex + 1, 0, id);

		const updatedFieldOrders: Record<string, string[]> = {};
		for (const locale of schema.locales) {
			if (locale === activeLocale) {
				updatedFieldOrders[locale] = baseIds;
			} else {
				const existing = schema.fieldOrders?.[locale] ?? getOrderedFields(schema, locale).map((f) => f.id);
				updatedFieldOrders[locale] = [...existing, id];
			}
		}

		schema = {
			...schema,
			fields: [...schema.fields, newField],
			translations: updatedTranslations,
			fieldOrders: updatedFieldOrders
		};
		selectedFieldId = id;
		markDirty();
	}

	function removeField(id: string): void {
		const updatedFieldOrders: Record<string, string[]> | undefined = schema.fieldOrders
			? Object.fromEntries(
					Object.entries(schema.fieldOrders).map(([loc, ids]) => [loc, ids.filter((fid) => fid !== id)])
				)
			: undefined;

		schema = {
			...schema,
			fields: schema.fields
				.filter((f) => f.id !== id)
				.map((f, i) => ({ ...f, order: i })),
			...(updatedFieldOrders ? { fieldOrders: updatedFieldOrders } : {})
		};
		if (selectedFieldId === id) selectedFieldId = null;
		markDirty();
	}

	function reorderFields(newOrder: BuilderField[]): void {
		const newIds = newOrder.map((f) => f.id);
		if (activeLocale === schema.defaultLocale) {
			// Update the canonical order on each field AND sync the default-locale entry in fieldOrders
			const updatedFieldOrders: Record<string, string[]> = {
				...(schema.fieldOrders ?? {}),
				[schema.defaultLocale]: newIds
			};
			schema = {
				...schema,
				fields: newOrder.map((f, i) => ({ ...f, order: i })),
				fieldOrders: updatedFieldOrders
			};
		} else {
			// Only write a locale-specific order; leave field.order (default order) untouched
			schema = {
				...schema,
				fieldOrders: {
					...(schema.fieldOrders ?? {}),
					[activeLocale]: newIds
				}
			};
		}
		markDirty();
	}

	function updateField(id: string, patch: Partial<BuilderField>): void {
		schema = {
			...schema,
			fields: schema.fields.map((f) => (f.id === id ? { ...f, ...patch } : f))
		};
		markDirty();
	}

	function updateFieldConfig(id: string, patch: Partial<FieldConfig>): void {
		schema = {
			...schema,
			fields: schema.fields.map((f) =>
				f.id === id ? { ...f, config: { ...f.config, ...patch } } : f
			)
		};
		markDirty();
	}

	function updateTranslation(fieldId: string | null, key: string, value: string): void {
		const locale = activeLocale;
		ensureLocaleTranslation(locale);
		const t = schema.translations[locale];

		if (fieldId === null) {
			schema = {
				...schema,
				translations: {
					...schema.translations,
					[locale]: {
						...t,
						[key]: value
					}
				}
			};
		} else {
			// Field-level translation
			ensureFieldTranslation(locale, fieldId);
			const updatedT = schema.translations[locale];
			schema = {
				...schema,
				translations: {
					...schema.translations,
					[locale]: {
						...updatedT,
						fields: {
							...updatedT.fields,
							[fieldId]: {
								...updatedT.fields[fieldId],
								[key]: value
							}
						}
					}
				}
			};
		}
		markDirty();
	}

	function addLocale(locale: string): void {
		if (schema.locales.includes(locale)) return;
		// Seed the new locale's field order from the current default order
		const defaultOrder = getOrderedFields(schema, schema.defaultLocale).map((f) => f.id);
		schema = {
			...schema,
			locales: [...schema.locales, locale],
			translations: {
				...schema.translations,
				[locale]: {
					formTitle: '',
					formDescription: '',
					fields: Object.fromEntries(schema.fields.map((f) => [f.id, { label: '' }]))
				}
			},
			fieldOrders: {
				...(schema.fieldOrders ?? { [schema.defaultLocale]: defaultOrder }),
				[locale]: [...defaultOrder]
			}
		};
		markDirty();
	}

	function removeLocale(locale: string): void {
		if (locale === schema.defaultLocale) return;
		const { [locale]: _removed, ...remaining } = schema.translations;
		const { [locale]: _removedOrder, ...remainingOrders } = schema.fieldOrders ?? {};
		schema = {
			...schema,
			locales: schema.locales.filter((l) => l !== locale),
			translations: remaining,
			fieldOrders: Object.keys(remainingOrders).length > 0 ? remainingOrders : undefined
		};
		if (activeLocale === locale) activeLocale = schema.defaultLocale;
		markDirty();
	}

	function setLayout(layout: BuilderSchema['layout']): void {
		schema = { ...schema, layout };
		markDirty();
	}

	function setActiveLocale(locale: string): void {
		activeLocale = locale;
	}

	function setSelectedField(id: string | null): void {
		selectedFieldId = id;
		if (id !== null) showFormSettings = false;
	}

	function setShowFormSettings(show: boolean): void {
		showFormSettings = show;
		if (show) selectedFieldId = null;
	}

	function setMode(m: BuilderMode): void {
		mode = m;
	}

	function setConvoAllowEdit(allow: boolean): void {
		schema = { ...schema, convoAllowEdit: allow };
		markDirty();
	}

	async function setExpiration(newExpiresAt: string | null, newResponseLimit: number | null, newResponseTtlDays: number | null, newBurnAfterReading: boolean): Promise<void> {
		await updateFormExpiration(formId, newExpiresAt, newResponseLimit, newResponseTtlDays, newBurnAfterReading);
		expiresAt = newExpiresAt;
		responseLimit = newResponseLimit;
		responseTtlDays = newResponseTtlDays;
		burnAfterReading = newBurnAfterReading;
	}

	async function load(): Promise<void> {
		const { schema: loaded, record, formKey } = await getForm(masterKey, formId);
		resolvedFormKey = formKey;
		const s = loaded as BuilderSchema;
		schema = s;
		activeLocale = schema.defaultLocale;
		expiresAt = record.expiresAt ?? null;
		responseLimit = record.responseLimit ?? null;
		responseTtlDays = record.responseTtlDays ?? null;
		burnAfterReading = record.burnAfterReading ?? false;
		formStatus = record.status;
		hasUnpublishedChanges = record.hasUnpublishedChanges ?? true;
		if (record.renderKeySalt) {
			const raw = atob(record.renderKeySalt);
			const bytes = new Uint8Array(raw.length);
			for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
			currentRenderKeySalt = bytes;
		}
		dirty = false;
	}

	function setRenderKeySalt(salt: Uint8Array): void {
		currentRenderKeySalt = salt;
	}

	function markPublished(): void {
		hasUnpublishedChanges = false;
		formStatus = 'open';
		formsStore.updateStatus(formId, 'open');
	}

	async function save(): Promise<void> {
		if (saving) return;
		saving = true;
		try {
			await updateFormSchema(masterKey, formId, schema, resolvedFormKey ?? undefined);
			lastSaved = new Date();
			dirty = false;
			hasUnpublishedChanges = true;
			const title = schema.translations[schema.defaultLocale]?.formTitle;
			if (title) formsStore.updateName(formId, title);
		} finally {
			saving = false;
		}
	}

	async function flushSave(): Promise<void> {
		if (debounceTimer) {
			clearTimeout(debounceTimer);
			debounceTimer = null;
		}
		if (dirty) {
			await save();
		}
	}

	return {
		get schema() {
			return schema;
		},
		get saving() {
			return saving;
		},
		get lastSaved() {
			return lastSaved;
		},
		get dirty() {
			return dirty;
		},
		get activeLocale() {
			return activeLocale;
		},
		get selectedFieldId() {
			return selectedFieldId;
		},
		get mode() {
			return mode;
		},
		get renderKeySalt() {
			return currentRenderKeySalt;
		},
		get formKey() {
			return resolvedFormKey;
		},
		get expiresAt() {
			return expiresAt;
		},
		get responseLimit() {
			return responseLimit;
		},
		get responseTtlDays() {
			return responseTtlDays;
		},
		get burnAfterReading() {
			return burnAfterReading;
		},
		get showFormSettings() {
			return showFormSettings;
		},
		get formStatus() {
			return formStatus;
		},
		get hasUnpublishedChanges() {
			return hasUnpublishedChanges;
		},
		get selectedField() {
			return schema.fields.find((f) => f.id === selectedFieldId) ?? null;
		},
		get activeTranslation() {
			return schema.translations[activeLocale] ?? schema.translations[schema.defaultLocale];
		},
		addField,
		addFieldAt,
		removeField,
		reorderFields,
		updateField,
		updateFieldConfig,
		updateTranslation,
		addLocale,
		removeLocale,
		setLayout,
		setActiveLocale,
		setSelectedField,
		setShowFormSettings,
		setMode,
		setConvoAllowEdit,
		setExpiration,
		setRenderKeySalt,
		markPublished,
		load,
		save,
		flushSave
	};
}
