const THINK_OPEN = "<think>";
const THINK_CLOSE = "</think>";

export interface ParsedReasoningText {
  answer: string;
  thinking: string;
  hasThinking: boolean;
  thinkingOpen: boolean;
}

function trimReasoningSection(text: string): string {
  return text.replace(/<\/?think>/gi, "").trim();
}

export function parseReasoningText(text: string): ParsedReasoningText {
  const lower = text.toLowerCase();
  if (!lower.includes(THINK_OPEN) && !lower.includes(THINK_CLOSE)) {
    return {
      answer: text,
      thinking: "",
      hasThinking: false,
      thinkingOpen: false,
    };
  }

  const answerParts: string[] = [];
  const thinkingParts: string[] = [];
  let cursor = 0;
  let inThinking = false;
  let sawOpenTag = false;
  let thinkingOpen = false;

  for (;;) {
    if (!inThinking) {
      const openIdx = lower.indexOf(THINK_OPEN, cursor);
      const closeIdx = lower.indexOf(THINK_CLOSE, cursor);

      if (openIdx === -1 && closeIdx === -1) {
        answerParts.push(text.slice(cursor));
        break;
      }

      if (closeIdx !== -1 && (openIdx === -1 || closeIdx < openIdx)) {
        answerParts.push(text.slice(cursor, closeIdx));
        cursor = closeIdx + THINK_CLOSE.length;
        continue;
      }

      sawOpenTag = true;
      answerParts.push(text.slice(cursor, openIdx));
      cursor = openIdx + THINK_OPEN.length;
      inThinking = true;
      continue;
    }

    const closeIdx = lower.indexOf(THINK_CLOSE, cursor);
    if (closeIdx === -1) {
      thinkingOpen = true;
      thinkingParts.push(text.slice(cursor));
      break;
    }

    thinkingParts.push(text.slice(cursor, closeIdx));
    cursor = closeIdx + THINK_CLOSE.length;
    inThinking = false;
  }

  const thinking = trimReasoningSection(thinkingParts.join(""));

  return {
    answer: trimReasoningSection(answerParts.join("")),
    thinking,
    hasThinking: sawOpenTag || thinking !== "",
    thinkingOpen,
  };
}

export function stripReasoningText(text: string): string {
  return parseReasoningText(text).answer;
}
