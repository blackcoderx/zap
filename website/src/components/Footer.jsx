function Footer() {
  return (
    <footer className="bg-charcoal py-12">
      <div className="max-w-4xl mx-auto px-6">
        <div className="flex flex-col md:flex-row justify-between items-center gap-6">
          <div className="flex items-center gap-3">
            <svg className="w-6 h-6" viewBox="0 0 100 100">
              <polygon
                points="60,5 25,50 45,50 40,95 75,50 55,50"
                fill="#D4A017"
                stroke="#000"
                strokeWidth="3"
              />
            </svg>
            <span className="font-bold tracking-wider">ZAP</span>
          </div>

          <div className="flex gap-6 text-sm text-silver">
            <a
              href="https://github.com/blackcoderx/zap"
              className="hover:text-mustard transition-colors"
            >
              GitHub
            </a>
            <a
              href="https://github.com/blackcoderx/zap/issues"
              className="hover:text-mustard transition-colors"
            >
              Issues
            </a>
            <a
              href="https://github.com/blackcoderx/zap/blob/main/LICENSE"
              className="hover:text-mustard transition-colors"
            >
              MIT License
            </a>
          </div>
        </div>

        <div className="mt-8 pt-6 border-t-2 border-ash text-center text-silver text-sm">
          <p>Built with the <a href="https://charm.sh/" className="text-mustard hover:underline">Charm</a> ecosystem</p>
          <p className="mt-2 text-xs">2026 &bull; blackcoderx</p>
        </div>
      </div>
    </footer>
  )
}

export default Footer
