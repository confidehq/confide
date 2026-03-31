import type { BuilderField, CheckboxesConfig, DateTimeConfig } from './types/builder';

export type AnswerValue = string | string[] | number | null | undefined;

export function validateAnswer(field: BuilderField, value: AnswerValue): string | null {
	if (field.type === 'section_break' || field.type === 'heading' || field.type === 'accordion' || field.type === 'accent') return null;
	if (field.required && isEmpty(value)) return 'This field is required.';
	if (field.type === 'checkboxes') {
		const cfg = field.config as CheckboxesConfig;
		const arr = (value as string[]) ?? [];
		if (cfg.minSelect && arr.length < cfg.minSelect)
			return `Select at least ${cfg.minSelect} option${cfg.minSelect === 1 ? '' : 's'}.`;
		if (cfg.maxSelect && arr.length > cfg.maxSelect)
			return `Select at most ${cfg.maxSelect} option${cfg.maxSelect === 1 ? '' : 's'}.`;
	}
	if (field.type === 'date_time' && value) {
		const cfg = field.config as DateTimeConfig;
		if (cfg.min && String(value) < cfg.min) return `Date must be on or after ${cfg.min}.`;
		if (cfg.max && String(value) > cfg.max) return `Date must be on or before ${cfg.max}.`;
	}
	return null;
}

export function validateAll(
	fields: BuilderField[],
	answers: Record<string, AnswerValue>
): Record<string, string> {
	const errors: Record<string, string> = {};
	for (const field of fields) {
		if (field.type === 'section_break' || field.type === 'heading' || field.type === 'accordion' || field.type === 'accent') continue;
		const err = validateAnswer(field, answers[field.id]);
		if (err) errors[field.id] = err;
	}
	return errors;
}

function isEmpty(value: AnswerValue): boolean {
	if (value === null || value === undefined) return true;
	if (typeof value === 'string') return value.trim() === '';
	if (Array.isArray(value)) return value.length === 0;
	return false;
}
