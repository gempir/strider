import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  integrations: [
    starlight({
      title: 'Strider',
      description: 'A strict formatter and syntax linter for Go.',
      customCss: ['./src/styles/custom.css'],
      sidebar: [
        {
          label: 'Start here',
          items: ['getting-started', 'configuration'],
        },
        {
          label: 'Guides',
          items: ['formatter', 'linter'],
        },
        {
          label: 'Reference',
          items: [
            'reference/cli',
            {
              label: 'Lint rules',
              items: [{ autogenerate: { directory: 'rules' } }],
            },
          ],
        },
      ],
    }),
  ],
});
