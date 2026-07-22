const vm = require('vm');

let messages = Array.from({length: 100}, (_, i) => ({ id: i, text: 'Some long text string to make the payload heavy enough' }));
let chatsData = [{ id: '1', messages }];

const badStorage = {
  setItem(k, v) { 
    if (v.length > 4000) {
        const e = new Error('QuotaExceeded');
        e.name = 'QuotaExceededError';
        throw e;
    }
  }
};

const sandbox = {
    console,
    localStorage: badStorage,
    JSON,
    get: () => chatsData,
    chats: {
        update: (fn) => {
            chatsData = fn(chatsData);
        }
    }
};

vm.createContext(sandbox);
vm.runInContext(`
function saveChats() {
  try { 
    localStorage.setItem('messenger-chats', JSON.stringify(get(chats))) 
  } catch (e) {
    if (e.name === 'QuotaExceededError' || (e.message && e.message.includes('Quota'))) {
      console.warn('[messenger] Quota exceeded, trimming old messages...');
      chats.update(cs => cs.map(c => ({
        ...c,
        messages: c.messages.slice(-50)
      })));
      try {
        localStorage.setItem('messenger-chats', JSON.stringify(get(chats)));
      } catch (err) {
        console.error('[messenger] Save failed even after trim', err);
      }
    }
  }
}
saveChats();
`, sandbox);

console.log('Result messages count:', chatsData[0].messages.length);
if (chatsData[0].messages.length === 50) {
   console.log('Test PASSED: QuotaExceededError triggered and messages trimmed.');
} else {
   console.error('Test FAILED: Expected 50 messages, got ' + chatsData[0].messages.length);
   process.exit(1);
}
