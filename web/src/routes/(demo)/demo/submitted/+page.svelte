<script lang="ts">
import { ShieldCheck, ArrowRight } from "@lucide/svelte";
</script>

<svelte:head>
	<title>Report Submitted — Demo</title>
</svelte:head>

<div class="shell">
	<div class="card">
		<div class="icon-wrap">
			<ShieldCheck size={32} strokeWidth={1.75} />
		</div>
		<h1 class="title">Your report has been received.</h1>
		<p class="sub">
			A case worker will review it within 48 hours. Your data was encrypted
			locally before it left your device — it is never readable in transit
			or at rest.
		</p>
		<div class="note">
			<ShieldCheck size={13} strokeWidth={2} />
			Your data was encrypted locally and safely submitted.
		</div>
		<a href="/demo/workspace" class="next-btn">
			View Workspace <ArrowRight size={15} strokeWidth={1.75} />
		</a>
	</div>
</div>

<style>
	:global(html), :global(body) {
		background: #fff;
		color: #111;
	}

	.shell {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 60vh;
		padding: 3rem 1.5rem;
	}

	.card {
		max-width: 420px;
		width: 100%;
		text-align: center;
		display: flex;
		flex-direction: column;
		align-items: center;
		gap: 1rem;
	}

	.icon-wrap {
		display: flex;
		align-items: center;
		justify-content: center;
		width: 4rem;
		height: 4rem;
		border-radius: 50%;
		background: #dcfce7;
		color: #16a34a;
		margin-bottom: 0.25rem;
	}

	.title {
		font-size: 1.375rem;
		font-weight: 700;
		margin: 0;
		color: #111;
	}

	.sub {
		font-size: 0.9375rem;
		color: #6b7280;
		margin: 0;
		line-height: 1.6;
	}

	.note {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.8125rem;
		color: #9ca3af;
	}

	.next-btn {
		display: inline-flex;
		align-items: center;
		gap: 0.5rem;
		margin-top: 0.5rem;
		padding: 0.625rem 1.25rem;
		background: #2563ea;
		color: #fff;
		text-decoration: none;
		border-radius: 8px;
		font-size: 0.9375rem;
		font-weight: 600;
		transition: background 0.1s;
	}

	.next-btn:hover {
		background: #1d4ed8;
	}
</style>
