async function getJSON(url, options = {}) {
  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  const text = await res.text()
  try {
    return JSON.parse(text)
  } catch {
    return { raw: text, status: res.status }
  }
}

function render(id, data) {
  document.getElementById(id).textContent = JSON.stringify(data, null, 2)
}

async function refreshAll() {
  render('status', await getJSON('/api/status'))
  render('sessions', await getJSON('/api/sessions'))
  render('prompts', await getJSON('/api/prompts'))
  render('tasks', await getJSON('/api/tasks'))
  render('schedules', await getJSON('/api/schedules'))
}

document.querySelector('[data-action="refresh-sessions"]').addEventListener('click', async () => render('sessions', await getJSON('/api/sessions')))
document.querySelector('[data-action="refresh-prompts"]').addEventListener('click', async () => render('prompts', await getJSON('/api/prompts')))
document.querySelector('[data-action="refresh-tasks"]').addEventListener('click', async () => render('tasks', await getJSON('/api/tasks')))
document.querySelector('[data-action="refresh-schedules"]').addEventListener('click', async () => render('schedules', await getJSON('/api/schedules')))

document.getElementById('schedule-form').addEventListener('submit', async (event) => {
  event.preventDefault()
  const form = new FormData(event.target)
  const kind = form.get('kind')
  const target = String(form.get('target') || '').trim()
  let parameters = {}
  const raw = String(form.get('parameters') || '').trim()
  if (raw) {
    parameters = JSON.parse(raw)
  }
  const payload = kind === 'shell'
    ? { command: target }
    : { tool: target, parameters }
  const body = {
    name: String(form.get('name') || ''),
    cron: String(form.get('cron') || ''),
    kind,
    workspace: String(form.get('workspace') || ''),
    enabled: form.get('enabled') === 'on',
    payload,
  }
  const result = await getJSON('/api/schedules', {
    method: 'POST',
    body: JSON.stringify(body),
  })
  document.getElementById('form-result').textContent = JSON.stringify(result, null, 2)
  render('schedules', await getJSON('/api/schedules'))
})

refreshAll()
