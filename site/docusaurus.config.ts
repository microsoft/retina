// @ts-check
// Note: type annotations allow type checking and IDEs autocompletion

import { themes as prismThemes } from "prism-react-renderer";
import { githubA11yLight } from "./src/prismColorTheme";

const config = {
  title: 'Retina',
  tagline: 'kubernetes network observability platform',
  favicon: 'img/favicon.svg',

  // Set the production url of your site here
  url: 'https://retina.sh',
  // Set the /<baseUrl>/ pathname under which your site is served
  // For GitHub pages deployment, it is often '/<projectName>/'
  baseUrl: '/',

  // GitHub pages deployment config.
  // If you aren't using GitHub pages, you don't need these.
  organizationName: 'Azure', // Usually your GitHub org/user name.
  projectName: 'Retina', // Usually your repo name.

  onBrokenLinks: 'throw',
  onBrokenMarkdownLinks: 'warn',

  // Even if you don't use internalization, you can use this field to set useful
  // metadata like html lang. For example, if your site is Chinese, you may want
  // to replace "en" with "zh-Hans".
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
        max: 1030, // max resized image's size.
        min: 640, // min resized image's size. if original is lower, use that size.
        steps: 2, // the max number of images generated between min and max (inclusive)
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
      /** @type {import('@docusaurus/preset-classic').Options} */
      ({
        docs: {
          sidebarPath: require.resolve('./sidebars.js'),
          path: '../docs',
          editUrl:
            'https://github.com/microsoft/retina/blob/main/docs',
        },
        theme: {
          customCss: require.resolve('./src/css/custom.css'),
        },
      }),
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

  themeConfig:
    /** @type {import('@docusaurus/preset-classic').ThemeConfig} */
    ({
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
          srcDark : "img/retina-logo-dark.svg",
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
          srcDark : "img/retina-logo-dark.svg",
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
    }),
};

export default config;
