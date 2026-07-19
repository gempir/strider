import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

const benchmarkThemeBridge = `
  (() => {
    const syncFrame = (frame) => {
      try {
        frame.contentDocument.documentElement.dataset.theme =
          document.documentElement.dataset.theme;
      } catch {}
    };
    const syncAll = () =>
      document.querySelectorAll('iframe.benchmark-report').forEach(syncFrame);

    addEventListener('load', (event) => {
      if (event.target?.matches?.('iframe.benchmark-report')) syncFrame(event.target);
    }, true);
    document.addEventListener('change', (event) => {
      if (event.target?.matches?.('starlight-theme-select select')) {
        requestAnimationFrame(syncAll);
      }
    });
    document.addEventListener('astro:page-load', syncAll);
  })();
`;

export default defineConfig({
  integrations: [
    starlight({
      title: 'Strider',
      description: 'Formatting and unified code checks for Go.',
      favicon: '/favicon.png',
      social: [
        { icon: 'github', label: 'gempir/strider on GitHub', href: 'https://github.com/gempir/strider' },
      ],
      pagination: false,
      customCss: ['./src/styles/custom.css'],
      head: [{ tag: 'script', content: benchmarkThemeBridge }],
      sidebar: [
        {
          label: 'Start here',
          items: [
            { slug: 'getting-started', label: 'Getting started' },
            { slug: 'configuration', label: 'Configuration' },
            { slug: 'baselines', label: 'Baselines' },
            { slug: 'suppress', label: 'Suppress' },
          ],
        },
        {
          label: 'Checks',
          items: [
            { slug: 'checks', label: 'Running checks' },
            { slug: 'formatter', label: 'Formatter' },
            {
              label: 'Style and maintainability',
              collapsed: true,
              items: [{ autogenerate: { directory: 'lints' } }],
            },
            {
              label: 'Correctness and safety',
              collapsed: true,
              items: [{ autogenerate: { directory: 'analyzers' } }],
            },
          ],
        },
        {
          label: 'Benchmarks',
          collapsed: true,
          items: [{ autogenerate: { directory: 'benchmarks' } }],
        },
        {
          label: 'Reference',
          items: [{ slug: 'reference/cli', label: 'CLI reference' }],
        },
      ],
    }),
  ],
});
