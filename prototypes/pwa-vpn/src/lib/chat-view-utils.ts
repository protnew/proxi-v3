/**
 * Chat utility functions extracted from ChatView.svelte.
 * Pure functions that don't depend on Svelte reactive state.
 */

import type { Message } from '../stores/messenger';

  function isMine(msg: Message): boolean {
    return msg.from === currentProfile?.pubkey
  }


  function getReplyText(msgId: string | undefined): string {
    if (!msgId || !currentChat) return ''
    const orig = currentChat.messages.find(m => m.id === msgId)
    return orig ? orig.text.slice(0, 60) : 'Сообщение'
  }


  async function openByCID(cid: string) {
    const got = await downloadByCID(cid)
    if (got) window.open(got.url, '_blank')
    else console.warn('[chatview] CID not found locally/gateway', cid)
  }


  function downloadFile(msg: Message) {
    if (!msg.fileUrl) return
    const a = document.createElement('a')
    a.href = msg.fileUrl
    a.download = msg.fileName || 'file'
    a.click()
  }

// M-011: URL preview — extract URLs from message text

let urlPreviews = $state<Record<string, any>>({});


async function fetchUrlPreview(url: string) {
  if (urlPreviews[url]) return;
  try {
    const resp = await fetch(API_BASE + '/api/url-preview?url=' + encodeURIComponent(url));
    if (resp.ok) {
      const data = await resp.json();
      urlPreviews[url] = data;
    }
  } catch (e) { /* ignore */ }
}


function processMessageUrls(text: string) {
  const urls = extractUrls(text);
  urls.forEach(u => fetchUrlPreview(u));
}


export { isMine };

export { getReplyText };

export { downloadFile };

export { openByCID };

export { processMessageUrls };

export { fetchUrlPreview };
