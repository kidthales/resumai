import type { ArtifactTabKey, LlmConfig, VaultOverview } from '../types';

export const SAMPLE_PREVIEW_MARKDOWN = `# Dr. Henry "Hank" McCoy

Westchester County, NY • h.mccoy@krakoa.org • (555) 019-1963 • linkedin.com/in/hankmccoy

## Professional Summary

Principal Systems Architect and Technical Consultant with **20+ years** of experience designing fault-tolerant distributed telemetry networks, bio-organic gateway relays, and high-concurrency simulation engines. Proven track record leading cross-functional crisis engineering teams and delivering zero-downtime infrastructure at planetary scale.

## Skills

- **Systems & Languages:** Rust, C++, Python, TypeScript, Linux Kernel, Distributed Systems, Embedded Telemetry
- **Architecture & Security:** Fault-Tolerant Mesh Relays, Telepathic Cryptography, High-Concurrency Simulation (Danger Room Holo-Matrix)
- **Leadership & Consulting:** Principal Architecture Audits, Technical Due Diligence, Cross-Functional Team Mentorship

## Professional Experience

### Krakoan Habitat & Gateway Systems - Krakoa

**Principal Architect - Bio-Organic Infrastructures** | Oct 2019 – Sep 2026

- Accomplished **99.998% global transit availability**, as measured by real-time telemetry across 140+ habitat relays, by architecting fault-tolerant gateway synchronization protocols in Rust and Python.
- Reduced anomalous neural feedback incidents by [METRIC: e.g. 45% reduction in latency/faults], as measured by quarterly security audits, by implementing automated bio-dome equilibrium circuit breakers.

### Wakandan International Outreach - Birnin Zana, Wakanda

**Senior Defense Systems & Anomaly Specialist** | Jan 2012 – Sep 2019

- Accelerated anomaly containment response times by **62%**, as measured by perimeter grid diagnostics, by engineering automated vibranium-matrix kinetic dissipation arrays in C++.

## Education

- **Ph.D. in Biophysics & Genetic Systems** — Xavier Institute for Higher Learning
`;

export interface StudioState {
    overview: VaultOverview | null;
    llmConfig: LlmConfig;
    activeMode: 'pipeline' | 'vault';
    activeArtifactTab: ArtifactTabKey;
    artifacts: Record<ArtifactTabKey, string>;
    matchScore: number | null;
    selectedArchetype: string;
    archetypeRationale: string;
    activeStyleFile: string;
    activeCssContent: string;
    fontSizePt: number;
    lineHeight: number;
    marginIn: number;
    targetPages: 1 | 2;
    vaultEditor: {
        category: string;
        filename: string;
        customTargetFilename: string | null;
    };
    isRunning: boolean;
}

export function createDefaultState(): StudioState {
    return {
        overview: null,
        llmConfig: {
            platform: 'ollama',
            ollama_base_url: 'http://localhost:11434',
            ollama_model: 'qwen2.5:14b-instruct-q5_K_M',
            gemini_api_key: '',
            gemini_flash_model: 'gemini-3.5-flash',
            gemini_lite_model: 'gemini-3.5-flash-lite'
        },
        activeMode: 'pipeline',
        activeArtifactTab: 'final',
        artifacts: {
            final: SAMPLE_PREVIEW_MARKDOWN,
            draft: '',
            fact_check: '',
            alignment: '',
            cover_letter: ''
        },
        matchScore: null,
        selectedArchetype: '',
        archetypeRationale: '',
        activeStyleFile: 'resume.sample.css',
        activeCssContent: '',
        fontSizePt: 10,
        lineHeight: 1.35,
        marginIn: 0.45,
        targetPages: 2,
        vaultEditor: {
            category: 'pii',
            filename: 'work_history.sample.md',
            customTargetFilename: 'work_history.md'
        },
        isRunning: false
    };
}
