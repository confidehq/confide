import type { FormSummary } from "$lib/forms";
import type { BuilderSchema } from "$lib/types/builder";
import type { Workspace } from "$lib/workspaces";

export interface DemoResponse {
	id: string;
	receivedAt: string;
	answers: Record<string, string | string[]>;
}

export const DEMO_WORKSPACE: Workspace = {
	id: "demo-ws-1",
	name: "Community Justice Initiative",
	slug: "community-justice",
	plan: "pro",
	planStatus: "active",
	role: "owner",
	status: "active",
};

export const DEMO_FORM_NAMES: Record<string, string> = {
	"demo-form-1": "Anonymous Incident Report",
	"demo-form-2": "Legal Aid Request",
	"demo-form-3": "Financial Assistance Application",
};

export const DEMO_FORMS: FormSummary[] = [
	{
		formId: "demo-form-1",
		status: "open",
		schemaVersion: 1,
		responseCount: 23,
		createdAt: "2025-03-01T10:00:00Z",
		updatedAt: "2026-05-28T14:32:00Z",
		burnAfterReading: false,
		hasUnpublishedChanges: false,
	},
	{
		formId: "demo-form-2",
		status: "open",
		schemaVersion: 1,
		responseCount: 8,
		createdAt: "2025-04-15T09:00:00Z",
		updatedAt: "2026-06-01T11:00:00Z",
		burnAfterReading: false,
		hasUnpublishedChanges: false,
	},
	{
		formId: "demo-form-3",
		status: "closed",
		schemaVersion: 1,
		responseCount: 47,
		createdAt: "2024-11-01T08:00:00Z",
		updatedAt: "2025-12-31T23:59:00Z",
		burnAfterReading: false,
		hasUnpublishedChanges: false,
	},
];

export const DEMO_RESPONSES: DemoResponse[] = [
	{
		id: "a1b2c3d4e5f6",
		receivedAt: "2026-06-03T14:22:00Z",
		answers: {
			incident_type: "opt-1",
			incident_date: "2026-05-29",
			description:
				"During a routine traffic stop near 5th and MLK, the officer yanked me out of the car without cause, slammed me against the hood, and used profanity. I was not resisting. A bystander recorded part of it. I was given no reason for the stop and released without a ticket.",
			location: "5th Ave & MLK Blvd, Riverside District",
			assistance_needed: ["aid-1", "aid-4"],
			contact_method: "contact-1",
		},
	},
	{
		id: "b2c3d4e5f6a1",
		receivedAt: "2026-06-02T09:45:00Z",
		answers: {
			incident_type: "opt-2",
			incident_date: "2026-05-15",
			description:
				"Landlord posted a 3-day eviction notice on my door with no prior warning. I have paid rent on time every month for 3 years. The notice cites a lease violation I was never informed of. I believe this is retaliation for a recent maintenance complaint I filed with the city.",
			location: "Eastside neighborhood",
			assistance_needed: ["aid-1"],
			contact_method: "contact-2",
		},
	},
	{
		id: "c3d4e5f6a1b2",
		receivedAt: "2026-06-01T16:08:00Z",
		answers: {
			incident_type: "opt-3",
			incident_date: "2026-05-20",
			description:
				"My employer withheld two weeks of pay claiming I was terminated for cause. I have documentation showing I resigned with proper notice. HR has stopped responding to my emails. I need help recovering unpaid wages and getting a proper separation letter.",
			location: "Downtown business district",
			assistance_needed: ["aid-1", "aid-2"],
			contact_method: "contact-1",
		},
	},
	{
		id: "d4e5f6a1b2c3",
		receivedAt: "2026-05-30T11:33:00Z",
		answers: {
			incident_type: "opt-4",
			incident_date: "2026-05-10",
			description:
				"Was denied housing in a building that had available units. The property manager told a colleague there were vacancies but told me units were full. I suspect this is racial discrimination. I have texts showing the timeline discrepancy.",
			location: "",
			assistance_needed: ["aid-1", "aid-4"],
			contact_method: "contact-3",
		},
	},
	{
		id: "e5f6a1b2c3d4",
		receivedAt: "2026-05-28T08:17:00Z",
		answers: {
			incident_type: "opt-1",
			incident_date: "2026-05-27",
			description:
				"Officers conducted a search of my home without a warrant. They showed paperwork but refused to let me read it. Items were taken and no receipt was provided. I was told I would be contacted for follow-up but have heard nothing in 24 hours.",
			location: "North Park neighborhood",
			assistance_needed: ["aid-1"],
			contact_method: "contact-1",
		},
	},
];

export const DEMO_FORM_SCHEMA: BuilderSchema = {
	version: 1,
	defaultLocale: "en",
	locales: ["en"],
	layout: "scroll",
	showWatermark: false,
	fields: [
		{
			id: "privacy_note",
			type: "accent",
			required: false,
			order: 0,
			config: { variant: "note", icon: "shield" },
		},
		{
			id: "incident_type",
			type: "multiple_choice",
			required: true,
			order: 1,
			config: {
				options: [
					{ id: "opt-1", order: 0 },
					{ id: "opt-2", order: 1 },
					{ id: "opt-3", order: 2 },
					{ id: "opt-4", order: 3 },
					{ id: "opt-5", order: 4 },
				],
			},
		},
		{
			id: "incident_date",
			type: "date_time",
			required: false,
			order: 2,
			config: { mode: "date" },
		},
		{
			id: "description",
			type: "long_text",
			required: true,
			order: 3,
			config: { minRows: 4 },
		},
		{
			id: "location",
			type: "short_text",
			required: false,
			order: 4,
			config: {},
		},
		{
			id: "assistance_needed",
			type: "checkboxes",
			required: false,
			order: 5,
			config: {
				options: [
					{ id: "aid-1", order: 0 },
					{ id: "aid-2", order: 1 },
					{ id: "aid-3", order: 2 },
					{ id: "aid-4", order: 3 },
				],
			},
		},
		{
			id: "contact_method",
			type: "dropdown",
			required: false,
			order: 6,
			config: {
				options: [
					{ id: "contact-1", order: 0 },
					{ id: "contact-2", order: 1 },
					{ id: "contact-3", order: 2 },
				],
			},
		},
	],
	translations: {
		en: {
			formTitle: "Anonymous Incident Report",
			formDescription:
				"Submit your report safely and anonymously. Your response is encrypted before it leaves your device and is never linked to your identity.",
			submitButtonText: "Submit Report",
			convoCompletionMessage:
				"Your report has been received. A case worker will review it within 48 hours.",
			fields: {
				privacy_note: {
					label:
						"Your response is end-to-end encrypted and never linked to your identity.",
				},
				incident_type: {
					label: "What type of incident occurred?",
					options: [
						"Police misconduct",
						"Housing / eviction",
						"Workplace violation",
						"Discrimination",
						"Other",
					],
				},
				incident_date: {
					label: "Approximate date of incident",
				},
				description: {
					label: "Describe what happened",
					placeholder: "Provide as much detail as you're comfortable sharing…",
				},
				location: {
					label: "Where did this occur?",
					placeholder: "Neighborhood, city, or address (optional)",
				},
				assistance_needed: {
					label: "What type of assistance do you need?",
					options: [
						"Legal representation",
						"Financial aid",
						"Emergency housing",
						"Document support",
					],
				},
				contact_method: {
					label: "Preferred follow-up method",
					options: [
						"Secure message (this app)",
						"Email",
						"No follow-up needed",
					],
				},
			},
		},
	},
};
