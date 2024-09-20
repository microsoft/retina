// @ts-check

import { themes as prismThemes } from "prism-react-renderer";
import { githubA11yLight } from "./src/prismColorTheme";

const config = {
  title: 'Retina',
  tagline: 'kubernetes network observability platform',
  favicon: 'img/favicon.svg',
  url: 'https://retina.sh',
  baseUrl: '/',
  organizationName: 'Azure',
  projectName: 'Retina',
  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  i18n: {
    defaultLocale: 'en',
    locales: ['en'],
  },

  markdown: {
    format: "detect",
    mermaid: true,
  },

  plugins: [
    "docusaurus-lunr-search",
    [
      "@docusaurus/plugin-ideal-image",
      {
        quality: 70,
        max: 1030,
        min: 640,
        steps: 2,
        disableInDev: false,
      },
    ],
    function (context, options) {
      return {
        name: "webpack-configuration-plugin",
        configureWebpack(config, isServer, utils) {
          return {
            resolve: {
              symlinks: false,
            },
          };
        },
      };
    },
  ],

  presets: [
    [
      'classic',
      {
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          path: '../docs',
          editUrl: 'https://github.com/microsoft/retina/blob/main/docs',
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      },
    ],
  ],

  headTags: [
    {
      tagName: "link",
      attributes: {
        rel: "preconnect",
        href: "https://fonts.googleapis.com",
      },
    },
    {
      tagName: "link",
      attributes: {
        rel: "preconnect",
        href: "https://fonts.gstatic.com",
        crossorigin: "true",
      },
    },
    {
      tagName: "link",
      attributes: {
        rel: "stylesheet",
        href: "https://fonts.googleapis.com/css2?family=Overpass+Mono:wght@300..700&family=Overpass:ital,wght@0,100..900;1,100..900&family=Urbanist:ital,wght@0,100..900;1,100..900&display=swap",
      },
    },
  ],

  themeConfig: {
    image: "img/retina-social-card.png",
    metadata: [
      { name: "og:url", content: "/" },
      { name: "og:site_name", content: "Retina" },
      { name: "og:image:width", content: "1200" },
      { name: "og:image:height", content: "600" },
    ],
    navbar: {
      logo: {
        alt: 'Retina Logo',
        src: 'img/retina-logo.svg',
        srcDark: "img/retina-logo-dark.svg",
        width: "103",
        height: "32",
      },
      items: [
        {
          position: "left",
          to: "/",
          label: "Home",
          activeBaseRegex: `^\/$`,
        },
        {
          type: "docSidebar",
          sidebarId: "mainSidebar",
          position: "left",
          label: "Docs",
        },
        {
          href: 'https://github.com/microsoft/retina',
          label: 'GitHub',
          position: 'right',
        },
      ],
    },
    footer: {
      style: "light",
      logo: {
        alt: "Retina logo",
        src: "img/retina-logo.svg",
        srcDark: "img/retina-logo-dark.svg",
        width: "155",
        height: "32",
      },
      links: [
        {
          title: "Community",
          items: [
            {
              label: "Contribute",
              href: "https://github.com/microsoft/retina/tree/main/docs/07-Contributing",
            },
            {
              label: "Github",
              href: "https://github.com/microsoft/retina",
            },
          ],
        },
      ],
      copyright: `Copyright ${new Date().getFullYear()} Retina Contributors`,
    },
    prism: {
      additionalLanguages: ["bash", "yaml", "docker", "go"],
      theme: githubA11yLight,
      darkTheme: prismThemes.oceanicNext,
    },
  },
};

export default config;
