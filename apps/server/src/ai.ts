import { openai } from '@ai-sdk/openai';
import { generateText } from 'ai';
import type { Broadcast, IncidentSummaryResponse } from '@vts/common';

function fallbackSummary(senderId: string, evidence: Broadcast[]): IncidentSummaryResponse {
  const latest = evidence.at(-1);
  const emergencies = evidence.filter((item) => item.type === 'emergency').length;

  return {
    senderId,
    source: 'fallback',
    evidenceIds: evidence.map((item) => item.id),
    summary: latest
      ? `${senderId} needs review. ${emergencies} related emergency broadcast(s) are present, and the latest evidence is ${latest.id} from ${latest.timestamp}. Dispatch a scout before trusting any unverified all-clear.`
      : `No evidence was found for ${senderId}. Verify the sender identity and data import.`,
  };
}

export async function summarizeIncident(
  senderId: string,
  evidence: Broadcast[],
): Promise<IncidentSummaryResponse> {
  if (!process.env.OPENAI_API_KEY || evidence.length === 0) {
    return fallbackSummary(senderId, evidence);
  }

  const modelName = process.env.OPENAI_MODEL ?? 'gpt-4.1-mini';
  const result = await generateText({
    model: openai(modelName),
    system: [
      'You are an incident analyst for the Silent Outposts relay network.',
      'Write a concise operational brief using only the supplied records.',
      'Cite broadcast IDs inline, state uncertainty, and finish with one recommended action.',
      'Never treat a ground-truth label as information that was available at message time.',
    ].join(' '),
    prompt: `Analyse ${senderId}. Evidence:\n${JSON.stringify(evidence, null, 2)}`,
  });

  return {
    senderId,
    source: 'ai',
    evidenceIds: evidence.map((item) => item.id),
    summary: result.text,
  };
}

