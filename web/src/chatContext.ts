import type { ChatMessage, ContentPart } from "./api/client";

const recentMessageLimit = 6;
const relevantMessageLimit = 6;

const stopWords = new Set([
  "a",
  "an",
  "and",
  "are",
  "as",
  "at",
  "be",
  "but",
  "by",
  "can",
  "do",
  "for",
  "from",
  "how",
  "i",
  "in",
  "is",
  "it",
  "of",
  "on",
  "or",
  "that",
  "the",
  "this",
  "to",
  "what",
  "with",
  "you",
  "我",
  "你",
  "他",
  "她",
  "它",
  "的",
  "了",
  "是",
  "在",
  "和",
]);

interface MessageGroup {
  indices: number[];
  score: number;
}

export function buildChatContextMessages(
  history: ChatMessage[],
  currentUserMessage: ChatMessage,
): ChatMessage[] {
  if (history.length === 0) {
    return [currentUserMessage];
  }

  const selected = new Set<number>();
  const recentStart = Math.max(0, history.length - recentMessageLimit);
  for (let i = recentStart; i < history.length; i++) {
    selected.add(i);
  }

  for (const group of relevantOlderGroups(history, currentUserMessage, recentStart)) {
    for (const index of group.indices) {
      selected.add(index);
    }
  }

  return [...selected]
    .sort((a, b) => a - b)
    .map((index) => history[index])
    .concat(currentUserMessage);
}

function relevantOlderGroups(
  history: ChatMessage[],
  currentUserMessage: ChatMessage,
  recentStart: number,
): MessageGroup[] {
  const queryKeywords = keywordsForMessage(currentUserMessage);
  if (queryKeywords.size === 0 || recentStart <= 0) {
    return [];
  }

  const groups: MessageGroup[] = [];
  for (const indices of olderConversationGroups(history, recentStart)) {
    const groupKeywords = keywordsForText(indices.map((index) => messageText(history[index])).join(" "));
    let score = 0;
    for (const keyword of queryKeywords) {
      if (groupKeywords.has(keyword)) {
        score++;
      }
    }
    if (score > 0) {
      groups.push({ indices, score });
    }
  }

  groups.sort((a, b) => {
    if (a.score !== b.score) return b.score - a.score;
    return b.indices[0] - a.indices[0];
  });

  const selected: MessageGroup[] = [];
  let selectedMessages = 0;
  for (const group of groups) {
    if (selectedMessages >= relevantMessageLimit) {
      break;
    }
    if (selectedMessages + group.indices.length > relevantMessageLimit) {
      continue;
    }
    selected.push(group);
    selectedMessages += group.indices.length;
  }
  return selected;
}

function olderConversationGroups(history: ChatMessage[], end: number): number[][] {
  const groups: number[][] = [];
  let current: number[] = [];

  for (let i = 0; i < end; i++) {
    const role = history[i].role;
    if (role === "user" && current.length > 0) {
      groups.push(current);
      current = [];
    }
    current.push(i);
  }
  if (current.length > 0) {
    groups.push(current);
  }
  return groups;
}

function keywordsForMessage(message: ChatMessage): Set<string> {
  return keywordsForText(messageText(message));
}

function keywordsForText(text: string): Set<string> {
  const keywords = new Set<string>();
  const normalized = text.toLowerCase();

  for (const match of normalized.matchAll(/[\p{L}\p{N}_-]+/gu)) {
    const token = match[0].trim();
    if (isUsefulKeyword(token)) {
      keywords.add(token);
    }
  }

  for (const match of normalized.matchAll(/\p{Script=Han}+/gu)) {
    const segment = match[0];
    if (segment.length >= 2 && !stopWords.has(segment)) {
      keywords.add(segment);
    }
    for (let i = 0; i < segment.length - 1; i++) {
      const pair = segment.slice(i, i + 2);
      if (!stopWords.has(pair)) {
        keywords.add(pair);
      }
    }
  }

  return keywords;
}

function isUsefulKeyword(token: string): boolean {
  if (stopWords.has(token)) {
    return false;
  }
  if (/^\d+$/.test(token)) {
    return token.length >= 2;
  }
  if (/^\p{Script=Han}+$/u.test(token)) {
    return token.length >= 2;
  }
  return token.length >= 3;
}

function messageText(message: ChatMessage): string {
  return contentText(message.content);
}

function contentText(content: ChatMessage["content"]): string {
  if (typeof content === "string") {
    return content;
  }
  return content
    .map((part: ContentPart) => (part.type === "text" ? part.text || "" : ""))
    .join(" ");
}
