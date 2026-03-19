<script lang="ts">
	import { onMount } from 'svelte';
	import { detectPRFSupport, surfacePrfError } from '$lib/prf-detection.ts';
	import {
		deriveFormKey,
		deriveFormKeypair,
		deriveRecoveryKey,
		wrapKey,
		unwrapKey,
		encryptSchema,
		decryptSchema,
		encryptResponse,
		decryptResponse,
		hashForVerification
	} from '$lib/crypto.ts';
	import type { FormSchema, ResponsePayload } from '$lib/types/crypto.ts';

	// ---------------------------------------------------------------------------
	// Types & state
	// ---------------------------------------------------------------------------

	type StepStatus = 'pending' | 'running' | 'pass' | 'fail';

	interface Step {
		id: number;
		title: string;
		status: StepStatus;
		detail: string;
		expanded: boolean;
	}

	let steps = $state<Step[]>([
		{ id: 1, title: 'PRF Support Detection', status: 'pending', detail: '', expanded: false },
		{ id: 2, title: 'Registration: wrapKey with PRF output', status: 'pending', detail: '', expanded: false },
		{ id: 3, title: 'Assertion: unwrapKey determinism', status: 'pending', detail: '', expanded: false },
		{ id: 4, title: 'Full Key Hierarchy: encryptResponse / decryptResponse', status: 'pending', detail: '', expanded: false },
		{ id: 5, title: 'Recovery Flow: deriveRecoveryKey + wrap/unwrap', status: 'pending', detail: '', expanded: false },
		{ id: 6, title: 'Hash Verification: hashForVerification', status: 'pending', detail: '', expanded: false }
	]);

	let running = $state(false);
	let overallResult = $state<'pending' | 'pass' | 'fail'>('pending');

	// State shared between step 2 and step 3
	let simulatedPrfOutput: Uint8Array | null = null;
	let wrappedMasterKeyBytes: ArrayBuffer | null = null;

	// ---------------------------------------------------------------------------
	// Helpers
	// ---------------------------------------------------------------------------

	function toHex(buf: ArrayBuffer | Uint8Array): string {
		const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
		return Array.from(bytes)
			.map((b) => b.toString(16).padStart(2, '0'))
			.join('');
	}

	function updateStep(id: number, patch: Partial<Step>) {
		steps = steps.map((s) => (s.id === id ? { ...s, ...patch } : s));
	}

	async function runStep(id: number, fn: () => Promise<string>): Promise<boolean> {
		updateStep(id, { status: 'running' });
		try {
			const detail = await fn();
			updateStep(id, { status: 'pass', detail });
			return true;
		} catch (err) {
			updateStep(id, {
				status: 'fail',
				detail: err instanceof Error ? err.message : String(err)
			});
			return false;
		}
	}

	// ---------------------------------------------------------------------------
	// Step implementations
	// ---------------------------------------------------------------------------

	async function step1(): Promise<string> {
		const result = await detectPRFSupport();
		const lines = [
			`webAuthnSupported: ${result.webAuthnSupported}`,
			`platformAuthenticatorAvailable: ${result.platformAuthenticatorAvailable}`,
			`supported: ${result.supported}`,
			`reason: ${result.reason ?? 'null'}`
		];
		if (!result.webAuthnSupported || !result.platformAuthenticatorAvailable) {
			// Surface expected limitations; not a hard failure for the harness
			lines.push('⚠ PRF ceremony will not be available in this environment');
		}
		return lines.join('\n');
	}

	async function step2(): Promise<string> {
		// Simulate PRF output (32 random bytes — stands in for real PRF output)
		simulatedPrfOutput = crypto.getRandomValues(new Uint8Array(32));

		// Derive a master key
		const masterKey = await crypto.subtle.generateKey(
			{ name: 'AES-GCM', length: 256 },
			true,
			['encrypt', 'decrypt', 'wrapKey', 'unwrapKey']
		);

		// Derive PRF-based KEK from the simulated PRF output
		const kek = await crypto.subtle.importKey(
			'raw',
			simulatedPrfOutput,
			{ name: 'AES-KW' },
			false,
			['wrapKey', 'unwrapKey']
		);

		// Wrap masterKey with PRF-derived KEK
		wrappedMasterKeyBytes = await wrapKey(masterKey, kek);

		return [
			`PRF output (simulated): ${toHex(simulatedPrfOutput).slice(0, 32)}...`,
			`Wrapped master key length: ${wrappedMasterKeyBytes.byteLength} bytes (expected 40)`,
			`Wrapped master key: ${toHex(wrappedMasterKeyBytes)}`
		].join('\n');
	}

	async function step3(): Promise<string> {
		if (!simulatedPrfOutput || !wrappedMasterKeyBytes) {
			throw new Error('Step 2 must run first');
		}

		// Re-derive KEK from same PRF output (simulates assertion flow)
		const kek = await crypto.subtle.importKey(
			'raw',
			simulatedPrfOutput,
			{ name: 'AES-KW' },
			false,
			['wrapKey', 'unwrapKey']
		);

		// Unwrap master key
		const restoredMasterKey = await unwrapKey(wrappedMasterKeyBytes, kek);

		// Verify the restored key works by encrypt/decrypt round-trip
		const testData = new TextEncoder().encode('prf-determinism-test');
		const iv = crypto.getRandomValues(new Uint8Array(12));
		const ciphertext = await crypto.subtle.encrypt(
			{ name: 'AES-GCM', iv },
			restoredMasterKey,
			testData
		);
		const plaintext = await crypto.subtle.decrypt(
			{ name: 'AES-GCM', iv },
			restoredMasterKey,
			ciphertext
		);
		const recovered = new TextDecoder().decode(plaintext);

		if (recovered !== 'prf-determinism-test') {
			throw new Error(`Round-trip failed: got "${recovered}"`);
		}

		return [
			'PRF assertion simulation: PASS',
			'Master key unwrapped successfully',
			'Encrypt/decrypt round-trip: PASS'
		].join('\n');
	}

	async function step4(): Promise<string> {
		const masterKey = await crypto.subtle.generateKey(
			{ name: 'AES-GCM', length: 256 },
			true,
			['encrypt', 'decrypt', 'wrapKey', 'unwrapKey']
		);
		const formKey = await deriveFormKey(masterKey, 'harness-form-001');
		const keypair = await deriveFormKeypair(formKey);

		const pubKeyBytes = await crypto.subtle.exportKey('raw', keypair.publicKey);

		const sampleResponse: ResponsePayload = {
			submittedAt: new Date().toISOString(),
			locale: 'en',
			answers: { q1: 'Alice', q2: 'blue', q3: null, q4: 42 }
		};

		const encrypted = await encryptResponse(sampleResponse, keypair.publicKey);
		const recovered = await decryptResponse(
			encrypted.encryptedData,
			encrypted.ephemeralPublicKey,
			keypair.privateKey
		);

		const matches = JSON.stringify(recovered) === JSON.stringify(sampleResponse);
		if (!matches) {
			throw new Error('Payload mismatch after decryption');
		}

		const sampleSchema: FormSchema = {
			version: 1,
			defaultLocale: 'en',
			locales: ['en'],
			layout: 'scroll',
			fields: [{ id: 'q1', type: 'text', config: { label: 'Name' } }],
			translations: { en: { q1: 'What is your name?' } }
		};

		const schemaBlob = await encryptSchema(sampleSchema, formKey);
		const recoveredSchema = await decryptSchema(schemaBlob, formKey);
		const schemaMatches = JSON.stringify(recoveredSchema) === JSON.stringify(sampleSchema);
		if (!schemaMatches) throw new Error('Schema mismatch after decryption');

		return [
			`Public key (32 bytes): ${toHex(pubKeyBytes)}`,
			`Ephemeral public key: ${toHex(encrypted.ephemeralPublicKey)}`,
			`Encrypted response length: ${encrypted.encryptedData.byteLength} bytes`,
			`Response round-trip: PASS`,
			`Schema encrypted length: ${schemaBlob.byteLength} bytes`,
			`Schema round-trip: PASS`
		].join('\n');
	}

	async function step5(): Promise<string> {
		const recoveryCodes = [
			'ABCD-1234', 'EFGH-5678', 'IJKL-9012',
			'MNOP-3456', 'QRST-7890', 'UVWX-1234',
			'YZ01-5678', 'ABCD-9012', 'EFGH-3456',
			'IJKL-7890', 'MNOP-1234', 'QRST-5678'
		];

		const masterKey = await crypto.subtle.generateKey(
			{ name: 'AES-GCM', length: 256 },
			true,
			['encrypt', 'decrypt', 'wrapKey', 'unwrapKey']
		);

		const recoveryKey = await deriveRecoveryKey(recoveryCodes);
		const wrapped = await wrapKey(masterKey, recoveryKey);
		const recoveryKey2 = await deriveRecoveryKey(recoveryCodes);
		const unwrapped = await unwrapKey(wrapped, recoveryKey2);

		// Verify unwrapped key works
		const iv = crypto.getRandomValues(new Uint8Array(12));
		const testData = new TextEncoder().encode('recovery-test');
		const ciphertext = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, masterKey, testData);
		const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv }, unwrapped, ciphertext);
		const recovered = new TextDecoder().decode(plaintext);
		if (recovered !== 'recovery-test') throw new Error('Recovery round-trip failed');

		return [
			`12 recovery codes used`,
			`Wrapped master key length: ${wrapped.byteLength} bytes`,
			`Recovery key derivation: deterministic ✓`,
			`Recovery round-trip: PASS`
		].join('\n');
	}

	async function step6(): Promise<string> {
		const inputs = [
			{ label: 'empty', data: new Uint8Array(0) },
			{ label: '"abc"', data: new Uint8Array([0x61, 0x62, 0x63]) },
			{ label: '32 zero bytes', data: new Uint8Array(32) }
		];

		const lines: string[] = [];
		for (const { label, data } of inputs) {
			const hash = await hashForVerification(data);
			lines.push(`SHA-256(${label}): ${toHex(hash)}`);
		}

		// Verify determinism
		const hash1 = await hashForVerification(new Uint8Array([1, 2, 3]));
		const hash2 = await hashForVerification(new Uint8Array([1, 2, 3]));
		if (toHex(hash1) !== toHex(hash2)) throw new Error('Hash is not deterministic');
		lines.push('Determinism: PASS');

		return lines.join('\n');
	}

	// ---------------------------------------------------------------------------
	// Run all steps
	// ---------------------------------------------------------------------------

	async function runAll() {
		running = true;
		overallResult = 'pending';

		const stepFns = [step1, step2, step3, step4, step5, step6];
		let allPassed = true;

		for (let i = 0; i < stepFns.length; i++) {
			const ok = await runStep(i + 1, stepFns[i]);
			if (!ok) allPassed = false;
		}

		overallResult = allPassed ? 'pass' : 'fail';
		running = false;
	}

	onMount(() => {
		// Auto-expand failed steps when done
	});
</script>

<svelte:head>
	<title>GhostForm — PRF Harness</title>
</svelte:head>

<div style="font-family: monospace; max-width: 800px; margin: 40px auto; padding: 0 20px;">
	<h1 style="font-size: 1.4rem; margin-bottom: 4px;">GhostForm — Crypto / PRF Test Harness</h1>
	<p style="color: #888; font-size: 0.85rem; margin-bottom: 24px;">
		Dev-only manual verification of Phase 1 crypto primitives. NODE_ENV=development only.
	</p>

	<button
		onclick={runAll}
		disabled={running}
		style="
			padding: 10px 24px;
			background: {running ? '#555' : '#2563eb'};
			color: white;
			border: none;
			border-radius: 4px;
			cursor: {running ? 'not-allowed' : 'pointer'};
			font-family: monospace;
			font-size: 0.9rem;
			margin-bottom: 28px;
		"
	>
		{running ? 'Running...' : 'Run All Steps'}
	</button>

	{#if overallResult !== 'pending'}
		<div style="
			padding: 12px 16px;
			margin-bottom: 24px;
			border-radius: 4px;
			background: {overallResult === 'pass' ? '#14532d' : '#450a0a'};
			color: {overallResult === 'pass' ? '#bbf7d0' : '#fecaca'};
			font-weight: bold;
		">
			{overallResult === 'pass' ? '✓ ALL STEPS PASSED' : '✗ ONE OR MORE STEPS FAILED'}
		</div>
	{/if}

	{#each steps as step}
		<div style="
			border: 1px solid {
				step.status === 'pass' ? '#166534' :
				step.status === 'fail' ? '#991b1b' :
				step.status === 'running' ? '#1d4ed8' : '#374151'
			};
			border-radius: 6px;
			margin-bottom: 12px;
			overflow: hidden;
		">
			<div
				style="
					display: flex;
					align-items: center;
					gap: 12px;
					padding: 12px 16px;
					cursor: {step.detail ? 'pointer' : 'default'};
					background: #111;
				"
				onclick={() => { if (step.detail) step.expanded = !step.expanded; }}
				role="button"
				tabindex="0"
				onkeydown={(e) => { if (e.key === 'Enter' && step.detail) step.expanded = !step.expanded; }}
			>
				<span style="
					display: inline-block;
					min-width: 80px;
					padding: 2px 8px;
					border-radius: 3px;
					font-size: 0.75rem;
					font-weight: bold;
					text-align: center;
					background: {
						step.status === 'pass' ? '#166534' :
						step.status === 'fail' ? '#991b1b' :
						step.status === 'running' ? '#1d4ed8' : '#374151'
					};
					color: {
						step.status === 'pass' ? '#bbf7d0' :
						step.status === 'fail' ? '#fecaca' :
						step.status === 'running' ? '#bfdbfe' : '#9ca3af'
					};
				">
					{step.status === 'pending' ? 'PENDING' :
					 step.status === 'running' ? 'RUNNING' :
					 step.status === 'pass' ? 'PASS' : 'FAIL'}
				</span>
				<span style="font-size: 0.9rem; color: #e5e7eb;">
					Step {step.id}: {step.title}
				</span>
				{#if step.detail}
					<span style="margin-left: auto; color: #6b7280; font-size: 0.75rem;">
						{step.expanded ? '▲ hide' : '▼ detail'}
					</span>
				{/if}
			</div>

			{#if step.expanded && step.detail}
				<div style="
					padding: 12px 16px;
					background: #0a0a0a;
					border-top: 1px solid #1f2937;
					font-size: 0.8rem;
					color: #9ca3af;
					white-space: pre-wrap;
					word-break: break-all;
				">
					{step.detail}
				</div>
			{/if}
		</div>
	{/each}
</div>
