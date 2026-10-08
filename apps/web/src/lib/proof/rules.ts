// Mirrors `domain.Evidence.Check` (apps/server/internal/domain/terms.go): the proof must reach the
// member's minimums, and attachments still being processed never count and block the submit.

export type EvidenceRules = { minAttachments: number; minWords: number };
export type EvidenceCounts = { words: number; ready: number; pending: number };
export type EvidenceStatus = { ok: boolean; needWords: number; needFiles: number; waiting: boolean };

export function checkEvidence(rules: EvidenceRules, have: EvidenceCounts): EvidenceStatus {
  const needWords = Math.max(0, rules.minWords - have.words);
  const needFiles = Math.max(0, rules.minAttachments - have.ready);
  const waiting = have.pending > 0;
  return { ok: needWords === 0 && needFiles === 0 && !waiting, needWords, needFiles, waiting };
}
