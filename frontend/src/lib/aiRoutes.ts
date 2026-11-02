/**
 * AI workspace routes. Kept apart from lib/orchestrator so the router (entry chunk) does not pull
 * the orchestrator client into the landing page bundle.
 *
 * Deep link (MCP-created plans link here): /dashboard/ai/conversations/<conversationId>?plan=<planId>
 */

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
export const isUuid = (value: string): boolean => UUID_RE.test(value);

export const AI_WORKSPACE_PATH = '/dashboard/ai';
export const AI_CONVERSATIONS_PATH = '/dashboard/ai/conversations';

/** True for every pathname under /dashboard/ai (not /dashboard/ai-agents or /dashboard/ai-assistant). */
export const isAIWorkspacePath = (pathname: string): boolean =>
  pathname === AI_WORKSPACE_PATH || pathname.startsWith(`${AI_WORKSPACE_PATH}/`);

/**
 * Conversation id from `/dashboard/ai/conversations/<id>`. Non-UUID values are rejected because
 * the id becomes an API path segment.
 */
export function conversationIdFromPath(pathname: string): { id: string | null; invalid: boolean } {
  const prefix = `${AI_CONVERSATIONS_PATH}/`;
  if (!pathname.startsWith(prefix)) return { id: null, invalid: false };
  let raw = pathname.slice(prefix.length).replace(/\/+$/, '');
  try {
    raw = decodeURIComponent(raw);
  } catch {
    return { id: null, invalid: true };
  }
  if (!raw) return { id: null, invalid: false };
  return isUuid(raw) ? { id: raw.toLowerCase(), invalid: false } : { id: null, invalid: true };
}
