import React, { useEffect, useRef, useState } from 'react';

interface MermaidProps {
  children: string;
}

declare global {
  interface Window {
    mermaid: any;
  }
}

export default function Mermaid({ children }: MermaidProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const loadMermaid = async () => {
      if (!window.mermaid) {
        const script = document.createElement('script');
        script.src = 'https://cdn.jsdelivr.net/npm/mermaid@10.6.1/dist/mermaid.min.js';
        await new Promise((resolve) => {
          script.onload = resolve;
          document.head.appendChild(script);
        });
        window.mermaid.initialize({
          startOnLoad: false,
          theme: 'default',
          securityLevel: 'loose',
        });
      }

      if (ref.current && window.mermaid) {
        try {
          const id = `mermaid-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
          const { svg } = await window.mermaid.render(id, children.trim());
          ref.current.innerHTML = svg;
          setLoaded(true);
        } catch (e) {
          console.error('Mermaid render error:', e);
          ref.current.innerHTML = `<pre style="padding:16px;background:#f5f5f5;border-radius:4px;overflow:auto;">${children}</pre>`;
        }
      }
    };

    loadMermaid();
  }, [children]);

  return <div ref={ref} style={{ margin: '16px 0' }} />;
}
