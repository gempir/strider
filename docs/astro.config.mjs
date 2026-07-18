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
      description: 'Formatting, linting, and static analysis for Go.',
      customCss: ['./src/styles/custom.css'],
      head: [{ tag: 'script', content: benchmarkThemeBridge }],
      sidebar: [
        {
          label: 'Start here',
          items: ['getting-started', 'configuration', 'baselines'],
        },
        {
          label: 'Guides',
          items: ['formatter', 'linter', 'analyzers'],
        },
        {
          label: 'Benchmarks',
          collapsed: true,
          items: [{ autogenerate: { directory: 'benchmarks' } }],
        },
        {
          label: 'Reference',
          items: [
            'reference/cli',
            {
              label: 'Lints',
              items: [{ autogenerate: { directory: 'lints' } }],
            },
            {
              label: 'Analyzers',
              items: [{ autogenerate: { directory: 'analyzers' } }],
            },
          ],
        },
      ],
    }),
  ],
});
