import { useState, useEffect } from 'react'

const lines = [
  { type: 'input', text: '> GET http://localhost:8000/api/users' },
  { type: 'status', text: '  thinking  analyzing request...' },
  { type: 'tool', text: '  tool  http_request' },
  { type: 'result', text: '  result  500 Internal Server Error' },
  { type: 'status', text: '  thinking  searching codebase for error...' },
  { type: 'tool', text: '  tool  search_code "def get_users"' },
  { type: 'tool', text: '  tool  read_file api/handlers.py:42' },
  { type: 'answer', text: '' },
  { type: 'code', text: 'Found the bug at api/handlers.py:47' },
  { type: 'code', text: '' },
  { type: 'code', text: 'The query uses `user_id` but the' },
  { type: 'code', text: 'column is named `id`. Fix:' },
  { type: 'code', text: '' },
  { type: 'fix', text: '-  .filter(User.user_id == id)' },
  { type: 'fix-add', text: '+  .filter(User.id == id)' },
]

function Terminal() {
  const [visibleLines, setVisibleLines] = useState(0)
  const [started, setStarted] = useState(false)

  useEffect(() => {
    const observer = new IntersectionObserver(
      ([entry]) => {
        if (entry.isIntersecting && !started) {
          setStarted(true)
        }
      },
      { threshold: 0.3 }
    )

    const el = document.getElementById('terminal-demo')
    if (el) observer.observe(el)

    return () => observer.disconnect()
  }, [started])

  useEffect(() => {
    if (!started) return

    if (visibleLines < lines.length) {
      const delay = lines[visibleLines]?.type === 'input' ? 800 : 200
      const timer = setTimeout(() => {
        setVisibleLines(v => v + 1)
      }, delay)
      return () => clearTimeout(timer)
    }
  }, [visibleLines, started])

  const getLineClass = (type) => {
    switch (type) {
      case 'input': return 'text-bone'
      case 'status': return 'text-silver italic'
      case 'tool': return 'text-mustard'
      case 'result': return 'text-red-400'
      case 'answer': return 'text-bone'
      case 'code': return 'text-silver'
      case 'fix': return 'text-red-400'
      case 'fix-add': return 'text-green-400'
      default: return 'text-bone'
    }
  }

  return (
    <section className="bg-smoke py-16 border-b-4 border-ash">
      <div className="max-w-4xl mx-auto px-6">
        <h2 className="text-2xl font-bold mb-8 text-center">
          <span className="text-mustard">&gt;</span> See it in action
        </h2>

        <div
          id="terminal-demo"
          className="bg-charcoal border-4 border-ash p-6 font-mono text-sm relative overflow-hidden scanline"
        >
          <div className="flex gap-2 mb-4">
            <div className="w-3 h-3 bg-red-500 border-2 border-red-700"></div>
            <div className="w-3 h-3 bg-yellow-500 border-2 border-yellow-700"></div>
            <div className="w-3 h-3 bg-green-500 border-2 border-green-700"></div>
          </div>

          <div className="space-y-1 min-h-80">
            {lines.slice(0, visibleLines).map((line, i) => (
              <div
                key={i}
                className={`${getLineClass(line.type)} fade-in-up`}
                style={{ animationDuration: '0.2s' }}
              >
                {line.text || '\u00A0'}
              </div>
            ))}
            {visibleLines < lines.length && started && (
              <span className="cursor-blink text-mustard">_</span>
            )}
          </div>
        </div>

        <p className="text-silver text-center mt-4 text-sm">
          ZAP found the bug, read the code, and suggested the fix.
        </p>
      </div>
    </section>
  )
}

export default Terminal
