const elements = {
  form: document.querySelector('#reply-form'),
  input: document.querySelector('#offer-input'),
  generateButton: document.querySelector('#generate-button'),
  useNext: document.querySelector('#use-next'),
  copyButton: document.querySelector('#copy-button'),
  output: document.querySelector('#reply-output'),
  resultMeta: document.querySelector('#result-meta'),
  error: document.querySelector('#error-message'),
  toast: document.querySelector('#toast'),
  maxID: document.querySelector('#max-id'),
  nextID: document.querySelector('#next-id'),
  generatedCount: document.querySelector('#generated-count'),
  totalCount: document.querySelector('#total-count'),
};

let currentNextID = 0;
let toastTimer;

function renderStatus(status) {
  currentNextID = status.next_id;
  elements.maxID.textContent = status.max_id;
  elements.nextID.textContent = status.next_id || '全部完成';
  elements.generatedCount.textContent = status.generated_count;
  elements.totalCount.textContent = status.total_count;
  elements.useNext.disabled = !status.next_id;
}

async function requestJSON(url, options) {
  const response = await fetch(url, options);
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || '请求失败，请稍后重试');
  return body;
}

async function loadStatus() {
  try {
    renderStatus(await requestJSON('/api/status'));
  } catch (error) {
    elements.error.textContent = error.message;
  }
}

elements.useNext.addEventListener('click', () => {
  if (!currentNextID) return;
  elements.input.value = currentNextID;
  elements.input.focus();
});

elements.form.addEventListener('submit', async (event) => {
  event.preventDefault();
  const input = elements.input.value.trim();
  if (!input) return;

  elements.error.textContent = '';
  elements.generateButton.disabled = true;
  elements.generateButton.querySelector('span').textContent = '正在生成…';
  try {
    const result = await requestJSON('/api/replies', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ input }),
    });
    elements.output.value = result.reply;
    elements.copyButton.disabled = false;
    elements.resultMeta.textContent = `当前 ID：${result.id} · 最大 ID：${result.max_id}${result.already_generated ? ' · 此 ID 曾生成过' : ''}`;
    await loadStatus();
    elements.output.scrollTop = 0;
  } catch (error) {
    elements.error.textContent = error.message;
  } finally {
    elements.generateButton.disabled = false;
    elements.generateButton.querySelector('span').textContent = '生成回复';
  }
});

elements.copyButton.addEventListener('click', async () => {
  if (!elements.output.value) return;
  try {
    await navigator.clipboard.writeText(elements.output.value);
  } catch (_) {
    elements.output.select();
    document.execCommand('copy');
    window.getSelection()?.removeAllRanges();
  }
  clearTimeout(toastTimer);
  elements.toast.classList.add('show');
  elements.copyButton.querySelector('span').textContent = '已复制';
  toastTimer = setTimeout(() => {
    elements.toast.classList.remove('show');
    elements.copyButton.querySelector('span').textContent = '复制全文';
  }, 1800);
});

loadStatus();
