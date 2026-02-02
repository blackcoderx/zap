const features = [
  {
    icon: '{}',
    title: '28+ Built-in Tools',
    desc: 'HTTP requests, auth helpers, JSON validation, load testing, webhooks—all in natural language.'
  },
  {
    icon: '</>',
    title: 'Codebase Aware',
    desc: 'Searches your actual code with ripgrep. Parses stack traces from Python, Go, JS errors.'
  },
  {
    icon: '>>',
    title: '15+ Frameworks',
    desc: 'Gin, FastAPI, Express, Django, Rails, Spring, and more. Framework-specific debugging hints.'
  },
  {
    icon: '[]',
    title: 'Request Chaining',
    desc: 'Extract values, store variables, chain requests. Build full test suites with assertions.'
  },
  {
    icon: '()',
    title: 'Human-in-the-Loop',
    desc: 'File changes show colored diffs and require your approval. No surprise modifications.'
  },
  {
    icon: '~~',
    title: 'Local or Cloud',
    desc: 'Use Ollama locally for privacy, or connect to Gemini for cloud power. Your choice.'
  }
]

function Features() {
  return (
    <section className="bg-charcoal py-16 border-b-4 border-ash">
      <div className="max-w-4xl mx-auto px-6">
        <h2 className="text-2xl font-bold mb-12 text-center">
          <span className="text-mustard">&gt;</span> Features
        </h2>

        <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6 stagger-children">
          {features.map((f, i) => (
            <div
              key={i}
              className="border-4 border-ash p-6 hover:border-mustard transition-colors fade-in-up"
              style={{ opacity: 0 }}
            >
              <div className="text-mustard text-2xl font-bold mb-3 font-mono">
                {f.icon}
              </div>
              <h3 className="text-lg font-bold mb-2">{f.title}</h3>
              <p className="text-silver text-sm leading-relaxed">{f.desc}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}

export default Features
