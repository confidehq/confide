/**
 * GhostForm form builder store (Svelte 5 Runes).
 *
 * Create one instance per builder page via createBuilderStore(masterKey, formId).
 * Pass the returned store object into child components via setContext/getContext.
 */

import { updateFormSchema, getForm } from '$lib/forms';
import type { BuilderSchema, BuilderField, FieldType, FieldConfig, TranslationMap } from '$lib/types/builder';

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

	// Derived (readable)
	readonly selectedField: BuilderField | null;
	readonly activeTranslation: TranslationMap;

	// Actions
	addField(type: FieldType): void;
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

	// Debounce timer handle
	let debounceTimer: ReturnType<typeof setTimeout> | null = null;
	// Stored renderKey from when the form was last published
	let currentRenderKey: CryptoKey | null = null;

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

		schema = {
			...schema,
			fields: [...schema.fields, newField],
			translations: updatedTranslations
		};
		selectedFieldId = id;
		markDirty();
	}

	function removeField(id: string): void {
		schema = {
			...schema,
			fields: schema.fields
				.filter((f) => f.id !== id)
				.map((f, i) => ({ ...f, order: i }))
		};
		if (selectedFieldId === id) selectedFieldId = null;
		markDirty();
	}

	function reorderFields(newOrder: BuilderField[]): void {
		schema = {
			...schema,
			fields: newOrder.map((f, i) => ({ ...f, order: i }))
		};
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
			// Form-level translation (formTitle, formDescription, convoCompletionMessage)
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
			}
		};
		markDirty();
	}

	function removeLocale(locale: string): void {
		if (locale === schema.defaultLocale) return;
		const { [locale]: _removed, ...remaining } = schema.translations;
		schema = {
			...schema,
			locales: schema.locales.filter((l) => l !== locale),
			translations: remaining
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
	}

	function setMode(m: BuilderMode): void {
		mode = m;
	}

	function setConvoAllowEdit(allow: boolean): void {
		schema = { ...schema, convoAllowEdit: allow };
		markDirty();
	}

	async function load(): Promise<void> {
		const { schema: loaded } = await getForm(masterKey, formId);
		schema = loaded as BuilderSchema;
		activeLocale = schema.defaultLocale;
		dirty = false;
	}

	async function save(): Promise<void> {
		if (saving) return;
		saving = true;
		try {
			// For auto-save we need a renderKey. If we don't have one yet (never published),
			// generate a temporary one. The actual publish flow sets a canonical renderKey.
			let rk = currentRenderKey;
			if (!rk) {
				rk = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, true, [
					'encrypt',
					'decrypt'
				]);
				currentRenderKey = rk;
			}
			await updateFormSchema(masterKey, formId, schema, rk);
			lastSaved = new Date();
			dirty = false;
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
		get selectedField() {
			return schema.fields.find((f) => f.id === selectedFieldId) ?? null;
		},
		get activeTranslation() {
			return schema.translations[activeLocale] ?? schema.translations[schema.defaultLocale];
		},
		addField,
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
		setMode,
		setConvoAllowEdit,
		load,
		save,
		flushSave
	};
}
