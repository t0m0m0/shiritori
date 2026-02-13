export function getRoomLink(roomId: string): string {
  const url = new URL(window.location.href);
  url.searchParams.set('room', roomId);
  return url.toString();
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const ta = document.createElement('textarea');
    ta.value = text;
    ta.style.position = 'fixed';
    ta.style.opacity = '0';
    document.body.appendChild(ta);
    ta.select();
    try {
      const ok = document.execCommand('copy');
      document.body.removeChild(ta);
      return ok;
    } catch {
      document.body.removeChild(ta);
      return false;
    }
  }
}

export function katakanaToHiragana(str: string): string {
  return str.replace(/[\u30A1-\u30F6]/g, (ch) =>
    String.fromCharCode(ch.charCodeAt(0) - 0x60)
  );
}

export function isAllowedChar(ch: string): boolean {
  const code = ch.charCodeAt(0);
  if (code >= 0x3040 && code <= 0x309f) return true;
  if (code >= 0x30a0 && code <= 0x30ff) return true;
  if (ch === 'ー') return true;
  return false;
}

export function filterHiragana(value: string): string {
  let filtered = '';
  for (let i = 0; i < value.length; i++) {
    if (isAllowedChar(value[i])) filtered += value[i];
  }
  return katakanaToHiragana(filtered);
}

let toastId = 0;
export function nextToastId(): number {
  return ++toastId;
}
