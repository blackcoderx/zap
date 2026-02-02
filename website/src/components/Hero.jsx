import { useState, useEffect } from 'react'

function Hero() {
  const [showCursor, setShowCursor] = useState(true)
  const [typed, setTyped] = useState('')
  const fullText = 'API debugging that understands your code'

  useEffect(() => {
    let i = 0
    const timer = setInterval(() => {
      if (i < fullText.length) {
        setTyped(fullText.slice(0, i + 1))
        i++
      } else {
        clearInterval(timer)
      }
    }, 50)
    return () => clearInterval(timer)
  }, [])

  return (
    <header className="border-b-4 border-mustard bg-charcoal">
      <nav className="max-w-4xl mx-auto px-6 py-4 flex justify-between items-center">
        <div className="flex items-center gap-3">
          <svg className="w-8 h-8 flash" viewBox="0 0 100 100">
            <polygon
              points="60,5 25,50 45,50 40,95 75,50 55,50"
              fill="#D4A017"
              stroke="#000"
              strokeWidth="3"
            />
          </svg>
          <span className="text-xl font-bold tracking-wider">ZAP</span>
        </div>
        <div className="flex gap-6 text-sm">
          <a href="https://github.com/blackcoderx/zap" className="text-silver hover:text-mustard transition-colors">
            GitHub
          </a>
          <a href="#install" className="text-silver hover:text-mustard transition-colors">
            Install
          </a>
        </div>
      </nav>

      <div className="max-w-4xl mx-auto px-6 py-20">
        <div className="slide-in-left">
          <p className="text-mustard text-sm mb-2 tracking-widest">&gt; LAUNCHING TODAY</p>
          <h1 className="text-4xl md:text-5xl font-bold mb-6 leading-tight">
            {typed}
            <span className="cursor-blink text-mustard">_</span>
          </h1>
        </div>

        <p className="text-silver text-lg mb-8 max-w-2xl fade-in-up" style={{ animationDelay: '0.3s', opacity: 0 }}>
          ZAP is a terminal AI that doesn't just test APIs—it debugs them.
          When something breaks, it searches your actual code to find why.
        </p>

        <div className="flex gap-4 fade-in-up" style={{ animationDelay: '0.5s', opacity: 0 }}>
          <a
            href="#install"
            className="bg-mustard text-charcoal px-6 py-3 font-bold border-4 border-black hover:bg-mustard-dark transition-colors pulse-glow"
          >
            GET STARTED
          </a>
          <a
            href="https://github.com/blackcoderx/zap"
            className="border-4 border-silver text-silver px-6 py-3 font-bold hover:border-bone hover:text-bone transition-colors"
          >
            VIEW SOURCE
          </a>
        </div>
      </div>
    </header>
  )
}

export default Hero
