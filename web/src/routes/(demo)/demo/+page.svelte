<script lang="ts">
import { ArrowRight, FileText, ShieldCheck, Building2, LayoutDashboard } from "@lucide/svelte";
</script>

<svelte:head>
	<title>Confide — Live Demo</title>
</svelte:head>

<div class="page">
	<div class="container">
		<!-- Hero -->
		<div class="hero">
			<h1 class="hero-title">See Confide in action</h1>
			<p class="hero-sub">
				Walk through a complete workflow for a community organization collecting
				anonymous incident reports — from the secure intake form to the encrypted
				response dashboard.
			</p>
			<a href="/demo/form" class="cta-btn">
				Start Demo <ArrowRight size={16} strokeWidth={1.75} />
			</a>
		</div>

		<!-- Steps overview -->
		<div class="steps-grid">
			<a href="/demo/form" class="step-card">
				<div class="step-icon">
					<FileText size={22} strokeWidth={1.5} />
				</div>
				<div>
					<div class="step-num">Step 1</div>
					<div class="step-title">Intake Form</div>
					<div class="step-desc">
						A respondent fills out the anonymous incident report. Fields are
						validated locally before submission.
					</div>
				</div>
			</a>

			<a href="/demo/submitted" class="step-card">
				<div class="step-icon">
					<ShieldCheck size={22} strokeWidth={1.5} />
				</div>
				<div>
					<div class="step-num">Step 2</div>
					<div class="step-title">Submission</div>
					<div class="step-desc">
						The response is encrypted client-side before leaving the browser —
						never readable in transit or at rest.
					</div>
				</div>
			</a>

			<a href="/demo/workspace" class="step-card">
				<div class="step-icon">
					<Building2 size={22} strokeWidth={1.5} />
				</div>
				<div>
					<div class="step-num">Step 3</div>
					<div class="step-title">Workspace</div>
					<div class="step-desc">
						Your team's workspace shows all forms, response counts, and status —
						organized by project or campaign.
					</div>
				</div>
			</a>

			<a href="/demo/dashboard" class="step-card">
				<div class="step-icon">
					<LayoutDashboard size={22} strokeWidth={1.5} />
				</div>
				<div>
					<div class="step-num">Step 4</div>
					<div class="step-title">Dashboard</div>
					<div class="step-desc">
						A quick-glance overview of activity across all your forms — open
						forms, total responses, recent submissions.
					</div>
				</div>
			</a>
		</div>
	</div>
</div>

<style>
	.page {
		display: flex;
		justify-content: center;
		padding: 3.5rem 1.5rem 4rem;
	}

	.container {
		width: 100%;
		max-width: 800px;
	}

	.hero {
		text-align: center;
		margin-bottom: 3.5rem;
	}

	.hero-title {
		font-size: 2rem;
		font-weight: 700;
		color: var(--color-text);
		margin: 0 0 1rem;
	}

	.hero-sub {
		font-size: 0.9375rem;
		color: var(--color-subtle);
		max-width: 520px;
		margin: 0 auto 2rem;
		line-height: 1.65;
	}

	.cta-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		padding: 0.75rem 1.5rem;
		background: var(--color-primary);
		color: #fff;
		text-decoration: none;
		border-radius: 8px;
		font-size: 0.9375rem;
		font-weight: 600;
		transition: background 0.1s;
	}

	.cta-btn:hover {
		background: var(--color-primary-hover);
	}

	.steps-grid {
		display: grid;
		grid-template-columns: repeat(2, 1fr);
		gap: 1rem;
	}

	@media (max-width: 560px) {
		.steps-grid {
			grid-template-columns: 1fr;
		}
	}

	.step-card {
		display: flex;
		flex-direction: column;
		gap: 1rem;
		padding: 1.375rem;
		border: 1px solid var(--color-border-canvas);
		border-radius: 10px;
		text-decoration: none;
		color: inherit;
		background: var(--color-canvas);
		transition: border-color 0.15s, background 0.15s;
	}

	.step-card:hover {
		border-color: var(--color-primary);
		background: color-mix(in srgb, var(--color-primary) 5%, var(--color-canvas));
	}

	.step-icon {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 2.5rem;
		height: 2.5rem;
		border-radius: 8px;
		background: color-mix(in srgb, var(--color-primary) 12%, transparent);
		color: var(--color-primary);
		flex-shrink: 0;
	}

	.step-num {
		font-size: 0.75rem;
		font-weight: 600;
		letter-spacing: 0.07em;
		text-transform: uppercase;
		color: var(--color-muted);
		margin-bottom: 0.25rem;
	}

	.step-title {
		font-size: 1rem;
		font-weight: 600;
		color: var(--color-text);
		margin-bottom: 0.375rem;
	}

	.step-desc {
		font-size: 0.8125rem;
		color: var(--color-subtle);
		line-height: 1.6;
	}
</style>
