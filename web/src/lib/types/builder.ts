/**
 * Confide form builder — strongly-typed field and schema definitions.
 *
 * All 8 field types are represented as a discriminated union so that
 * builder components get full type inference.
 */

export type FieldType =
	| 'short_text'
	| 'long_text'
	| 'multiple_choice'
	| 'checkboxes'
	| 'dropdown'
	| 'date_time'
	| 'rating'
	| 'section_break'
	| 'heading'
	| 'accordion'
	| 'accent';

export interface ShortTextConfig {
	maxLength?: number;
}

export interface LongTextConfig {
	maxLength?: number;
	minRows?: number;
}

export interface ChoiceOption {
	id: string;
	order: number;
}

export interface MultipleChoiceConfig {
	options: ChoiceOption[];
	allowOther?: boolean;
}

export interface CheckboxesConfig {
	options: ChoiceOption[];
	minSelect?: number;
	maxSelect?: number;
}

export interface DropdownConfig {
	options: ChoiceOption[];
	searchable?: boolean;
}

export interface DateTimeConfig {
	mode: 'date' | 'time' | 'datetime';
	min?: string;
	max?: string;
}

export interface RatingConfig {
	scale: 5 | 10;
	shape: 'star' | 'number';
}

export interface SectionBreakConfig {
	// no config
}

export interface HeadingConfig {
	level: 0 | 1 | 2 | 3; // 0 = plain paragraph text
}

export interface AccordionConfig {
	// no config — title in translation.label, body in translation.helpText
}

export interface AccentConfig {
	variant: 'note' | 'warning' | 'danger' | 'success';
}

export type FieldConfig =
	| ShortTextConfig
	| LongTextConfig
	| MultipleChoiceConfig
	| CheckboxesConfig
	| DropdownConfig
	| DateTimeConfig
	| RatingConfig
	| SectionBreakConfig
	| HeadingConfig
	| AccordionConfig
	| AccentConfig;

export interface BuilderField {
	id: string;
	type: FieldType;
	required: boolean;
	order: number;
	config: FieldConfig;
}

export interface TranslationMap {
	formTitle: string;
	formDescription: string;
	convoCompletionMessage?: string;
	fields: Record<
		string,
		{
			label: string;
			helpText?: string;
			placeholder?: string;
			options?: string[]; // parallel to config.options array, indexed by order
		}
	>;
}

export interface BuilderSchema {
	version: number;
	name: string;
	defaultLocale: string;
	locales: string[];
	layout: 'scroll' | 'steps' | 'convo';
	convoAllowEdit?: boolean;
	fields: BuilderField[];
	translations: Record<string, TranslationMap>;
}
