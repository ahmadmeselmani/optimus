// @ts-check
// `@type` JSDoc annotations allow editor autocompletion and type checking
// (when paired with `@ts-check`).
// See: https://docusaurus.io/docs/api/docusaurus-config

import {themes as prismThemes} from 'prism-react-renderer';

// This runs in Node.js - Don't use client-side code here (browser APIs, JSX...)

/** @type {import('@docusaurus/types').Config} */
const config = {
  title: 'Optimus Handbook',
  tagline: 'A lightweight terminal interface for local Ollama models',
  favicon: 'img/favicon.ico',

  future: {
    v4: true,
  },

  // This handbook is served locally / from an internal host, not GitHub Pages.
  url: 'https://optimus.local',
  baseUrl: '/',

  organizationName: 'optimus',
  projectName: 'optimus-handbook',

  onBrokenLinks: 'throw',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    mermaid: true,
    hooks: {
      onBrokenMarkdownLinks: 'warn',
    },
  },
  themes: ['@docusaurus/theme-mermaid'],

  presets: [
    [
      'classic',
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          routeBasePath: '/',
          sidebarPath: './sidebars.js',
          editUrl: undefined,
        },
        blog: false,
        theme: {
          customCss: './src/css/custom.css',
        },
      }),
    ],
  ],

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
      colorMode: {
        respectPrefersColorScheme: true,
      },
      navbar: {
        title: 'Optimus Handbook',
        logo: {
          alt: 'Optimus Logo',
          src: 'img/docusaurus.png',
        },
        items: [
          {
            type: 'docSidebar',
            sidebarId: 'handbookSidebar',
            position: 'left',
            label: 'Handbook',
          },
        ],
      },
      footer: {
        style: 'dark',
        links: [
          {
            title: 'Handbook',
            items: [
              {label: 'Overview', to: '/'},
              {label: 'Architecture', to: '/architecture'},
              {label: 'Current Scope', to: '/milestones'},
              {label: 'Manual Testing', to: '/testing'},
            ],
          },
        ],
        copyright: `Optimus Handbook. © ${new Date().getFullYear()}.`,
      },
      prism: {
        theme: prismThemes.github,
        darkTheme: prismThemes.dracula,
      },
    }),
};

export default config;
