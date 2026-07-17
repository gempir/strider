import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Strider',
      description: 'Formatting, linting, and static analysis for Go.',
      customCss: ['./src/styles/custom.css'],
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
